package infrastructure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/repository"
	"github.com/gogf/gf/v2/errors/gerror"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const rawOutputVersion = "raw-v1"

type PreviewStore struct {
	storage      repository.ObjectStorage
	cache        *ValkeyStore
	clock        func() time.Time
	sourceClient *http.Client
	prefix       string
	identity     string
	signedURLMax int64
	slots        chan struct{}
	admission    chan struct{}
	converter    *officeConverter
}

func newPreviewStore(cfg Config, cache *ValkeyStore, clock func() time.Time, storage repository.ObjectStorage, prefix, identity string, maxTTL int64) *PreviewStore {
	source := cfg.SourceHTTPClient
	if source == nil {
		source = &http.Client{Timeout: 20 * time.Second}
	}
	// 保留测试 CA 和连接池，但强制每一跳都使用 HTTPS。
	copyClient := *source
	copyClient.Timeout = 8 * time.Second
	copyClient.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || r.URL.Scheme != "https" || r.URL.User != nil {
			return entity.ErrUnavailable
		}
		return nil
	}
	source = &copyClient
	return &PreviewStore{storage: storage, cache: cache, clock: clock, sourceClient: source, prefix: strings.Trim(prefix, "/"), identity: identity, signedURLMax: maxTTL, slots: make(chan struct{}, 4), admission: make(chan struct{}, 36), converter: newOfficeConverter(cfg.GotenbergURL)}
}

func (s *PreviewStore) Prepare(ctx context.Context, grant entity.Grant) (string, error) {
	if s == nil {
		return "", entity.ErrUnavailable
	}
	if !entity.SupportedExtension(path.Ext(grant.Filename)) {
		return "", entity.ErrInvalid
	}
	select {
	case s.admission <- struct{}{}:
		defer func() { <-s.admission }()
	default:
		return "", entity.ErrUnavailable
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return "", entity.ErrUnavailable
	}
	version := outputVersion(grant.Filename)
	cacheKey := s.identity + ":" + version + ":" + grant.ContentSHA256
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", entity.ErrUnavailable
	}
	owner := hex.EncodeToString(entropy[:])
	for {
		if record, err := s.cache.GetCache(ctx, cacheKey); err == nil {
			if !outputMIME(grant.Filename, record.MediaType) {
				return "", entity.ErrInvalid
			}
			if headErr := s.storage.Stat(ctx, record.ObjectKey); headErr == nil {
				return s.sign(ctx, record.ObjectKey, grant)
			} else {
				if !errors.Is(headErr, entity.ErrNotFound) {
					return "", entity.ErrUnavailable
				}
			}
		} else if !errors.Is(err, entity.ErrNotFound) {
			return "", err
		}
		acquired, err := s.cache.acquirePreparation(ctx, cacheKey, owner)
		if err != nil {
			return "", err
		}
		if acquired {
			break
		}
		select {
		case <-ctx.Done():
			return "", entity.ErrUnavailable
		case <-time.After(50 * time.Millisecond):
		}
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.cache.updatePreparation(releaseCtx, cacheKey, owner, true)
	}()
	// 取锁后再次检查，避免前一持有者刚发布时重复准备。
	if record, err := s.cache.GetCache(ctx, cacheKey); err == nil {
		if !outputMIME(grant.Filename, record.MediaType) {
			return "", entity.ErrInvalid
		}
		if err = s.storage.Stat(ctx, record.ObjectKey); err == nil {
			return s.sign(ctx, record.ObjectKey, grant)
		} else if !errors.Is(err, entity.ErrNotFound) {
			return "", entity.ErrUnavailable
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(preparationLease / 3)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if s.cache.updatePreparation(ctx, cacheKey, owner, false) != nil {
					cancel()
					return
				}
			}
		}
	}()
	body, contentType, err := s.download(ctx, grant)
	if err != nil {
		return "", err
	}
	if _, err = s.cache.Get(ctx, grant.Token); err != nil {
		return "", err
	}
	objectKey := fmt.Sprintf("%s/%s/%s/%s/%s", s.prefix, s.identity, version, grant.ContentSHA256, owner)
	if err = s.cache.registerObject(ctx, cacheKey, objectKey, grant.CacheExpiresAt); err != nil {
		return "", err
	}
	if err = s.storage.Put(ctx, objectKey, contentType, body); err != nil {
		return "", err
	}
	if err = s.storage.Stat(ctx, objectKey); err != nil {
		return "", entity.ErrUnavailable
	}
	if err = s.cache.PutCache(ctx, cacheKey, owner, cacheRecord{ObjectKey: objectKey, ExpiresAt: grant.CacheExpiresAt, MediaType: contentType}); err != nil {
		return "", err
	}
	return s.sign(ctx, objectKey, grant)
}

func (s *PreviewStore) Cleanup(ctx context.Context) error {
	if s == nil || s.cache.db == nil {
		return entity.ErrUnavailable
	}
	records, err := s.cache.cleanupCandidates(ctx, s.identity)
	if err != nil {
		return err
	}
	for _, record := range records {
		if !strings.HasPrefix(record.ObjectKey, s.prefix+"/"+s.identity+"/") {
			continue
		}
		if err := s.storage.Delete(ctx, record.ObjectKey); err != nil {
			continue
		}
		if err := s.cache.forgetObject(ctx, record.ObjectKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *PreviewStore) sign(ctx context.Context, objectKey string, grant entity.Grant) (string, error) {
	if _, err := s.cache.Get(ctx, grant.Token); err != nil {
		return "", err
	}
	record, err := s.cache.GetCache(ctx, s.identity+":"+outputVersion(grant.Filename)+":"+grant.ContentSHA256)
	if err != nil || record.ObjectKey != objectKey {
		return "", entity.ErrUnavailable
	}
	remaining := grant.ExpiresAt - s.clock().Unix()
	if cacheRemaining := grant.CacheExpiresAt - s.clock().Unix(); cacheRemaining < remaining {
		remaining = cacheRemaining
	}
	if s.signedURLMax < remaining {
		remaining = s.signedURLMax
	}
	if remaining < 1 {
		return "", entity.ErrNotFound
	}
	signedURL, expires, err := s.storage.SignGet(ctx, objectKey, remaining)
	if err != nil {
		return "", err
	}
	// SDK 自取时钟；跨秒/调度延迟不能让目标短链超出授权绝对期限。
	parsed, err := url.Parse(signedURL)
	if err != nil || parsed.Scheme != "https" {
		return "", entity.ErrUnavailable
	}
	if expires <= time.Now().Unix() || expires > grant.ExpiresAt || expires > record.ExpiresAt {
		return "", entity.ErrUnavailable
	}
	return signedURL, nil
}

func (s *PreviewStore) download(ctx context.Context, grant entity.Grant) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, grant.URL, nil)
	if err != nil {
		return nil, "", entity.ErrInvalid
	}
	response, err := s.sourceClient.Do(req)
	if err != nil {
		return nil, "", gerror.Wrap(entity.ErrUnavailable, "download source")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", gerror.Wrap(entity.ErrUnavailable, "source response")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
	if err != nil {
		return nil, "", gerror.Wrap(entity.ErrUnavailable, "source body")
	}
	if len(body) > 32<<20 {
		return nil, "", entity.ErrInvalid
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != grant.ContentSHA256 {
		return nil, "", entity.ErrInvalid
	}
	if ext := officeInputExtension(grant.Filename); ext != "" {
		if err := validateOffice(ctx, body, ext); err != nil {
			return nil, "", err
		}
		converted, err := s.converter.convert(ctx, body, "source"+ext)
		return converted, "application/pdf", err
	}
	if ext := strings.ToLower(path.Ext(grant.Filename)); ext == ".tif" || ext == ".tiff" {
		inputs, err := prepareTIFF(ctx, body)
		if err != nil {
			return nil, "", err
		}
		converted, err := s.converter.convertInputs(ctx, inputs)
		return converted, "application/pdf", err
	}
	if convertedImage(grant.Filename) {
		config, format, err := image.DecodeConfig(bytes.NewReader(body))
		expected := "tiff"
		if strings.EqualFold(path.Ext(grant.Filename), ".bmp") {
			expected = "bmp"
		}
		if err != nil || format != expected || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40000000 {
			return nil, "", entity.ErrInvalid
		}
		if _, _, err := image.Decode(bytes.NewReader(body)); err != nil {
			return nil, "", entity.ErrInvalid
		}
		converted, err := s.converter.convert(ctx, body, grant.Filename)
		return converted, "application/pdf", err
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if !rawMIME(path.Ext(grant.Filename), contentType) {
		return nil, "", entity.ErrInvalid
	}
	switch contentType {
	case "application/pdf":
		if !bytes.HasPrefix(body, []byte("%PDF-")) || !bytes.Contains(body, []byte("%%EOF")) {
			return nil, "", entity.ErrInvalid
		}
		if err := validatePDF(ctx, body); err != nil {
			return nil, "", err
		}
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		config, format, err := image.DecodeConfig(bytes.NewReader(body))
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40000000 || contentType != "image/"+format {
			return nil, "", entity.ErrInvalid
		}
		if _, _, err = image.Decode(bytes.NewReader(body)); err != nil {
			return nil, "", entity.ErrInvalid
		}
	}
	return body, contentType, nil
}

func rawMIME(ext, contentType string) bool {
	switch strings.ToLower(ext) {
	case ".pdf":
		return contentType == "application/pdf"
	case ".jpg", ".jpeg":
		return contentType == "image/jpeg"
	case ".png":
		return contentType == "image/png"
	case ".gif":
		return contentType == "image/gif"
	case ".webp":
		return contentType == "image/webp"
	default:
		return false
	}
}
