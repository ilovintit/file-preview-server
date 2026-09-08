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
	"strconv"
	"strings"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	_ "github.com/gen2brain/avif"
	"github.com/gogf/gf/v2/errors/gerror"
	_ "golang.org/x/image/webp"
)

const rawOutputVersion = "raw-v1"

func ossProfileIdentity(c AliyunOSSConfig) string {
	digest := sha256.Sum256([]byte(c.Endpoint + "\n" + c.Bucket + "\n" + c.PrefixBase))
	return hex.EncodeToString(digest[:])
}

type AliyunPreviewStore struct {
	bucket       *oss.Bucket
	signer       *oss.Bucket
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

func NewAliyunPreviewStore(cfg Config, cache *ValkeyStore, clock func() time.Time) (*AliyunPreviewStore, error) {
	c := cfg.AliyunOSS
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKeyID == "" || c.AccessKeySecret == "" {
		return nil, nil
	}
	options := []oss.ClientOption{oss.Timeout(3, 8)}
	if cfg.OSSHTTPClient != nil {
		options = append(options, oss.HTTPClient(cfg.OSSHTTPClient))
	}
	if c.SecurityToken != "" {
		options = append(options, oss.SecurityToken(c.SecurityToken))
	}
	client, err := oss.New(c.Endpoint, c.AccessKeyID, c.AccessKeySecret, options...)
	if err != nil {
		return nil, gerror.Wrap(entity.ErrUnavailable, "create OSS client")
	}
	client.SetRegion(c.Region)
	bucket, err := client.Bucket(c.Bucket)
	if err != nil {
		return nil, gerror.Wrap(entity.ErrUnavailable, "open OSS bucket")
	}
	signer := bucket
	if c.PreviewEndpoint != "" {
		previewOptions := append(append([]oss.ClientOption(nil), options...), oss.UseCname(true))
		previewClient, err := oss.New(c.PreviewEndpoint, c.AccessKeyID, c.AccessKeySecret, previewOptions...)
		if err != nil {
			return nil, entity.ErrUnavailable
		}
		previewClient.SetRegion(c.Region)
		signer, err = previewClient.Bucket(c.Bucket)
		if err != nil {
			return nil, entity.ErrUnavailable
		}
	}
	if c.SignedURLMaxTTL < 1 || c.PrefixBase == "" {
		return nil, gerror.Wrap(entity.ErrUnavailable, "invalid OSS configuration")
	}
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
	return &AliyunPreviewStore{bucket: bucket, signer: signer, cache: cache, clock: clock, sourceClient: source, prefix: strings.Trim(c.PrefixBase, "/"), identity: ossProfileIdentity(c), signedURLMax: c.SignedURLMaxTTL, slots: make(chan struct{}, 4), admission: make(chan struct{}, 36), converter: newOfficeConverter(cfg.GotenbergURL)}, nil
}

func (s *AliyunPreviewStore) Prepare(ctx context.Context, grant entity.Grant) (string, error) {
	if s == nil {
		return "", entity.ErrUnavailable
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
			if _, headErr := s.bucket.GetObjectDetailedMeta(record.ObjectKey, oss.WithContext(ctx)); headErr == nil {
				return s.sign(ctx, record.ObjectKey, grant)
			} else {
				var serviceError oss.ServiceError
				if !errors.As(headErr, &serviceError) || serviceError.StatusCode != 404 {
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
		if _, err = s.bucket.GetObjectDetailedMeta(record.ObjectKey, oss.WithContext(ctx)); err == nil {
			return s.sign(ctx, record.ObjectKey, grant)
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
	if err = s.bucket.PutObject(objectKey, bytes.NewReader(body), oss.ContentType(contentType), oss.ContentDisposition("inline"), oss.WithContext(ctx)); err != nil {
		return "", gerror.Wrap(entity.ErrUnavailable, "upload OSS object")
	}
	if _, err = s.bucket.GetObjectDetailedMeta(objectKey, oss.WithContext(ctx)); err != nil {
		return "", entity.ErrUnavailable
	}
	if err = s.cache.PutCache(ctx, cacheKey, owner, cacheRecord{ObjectKey: objectKey, ExpiresAt: grant.CacheExpiresAt, MediaType: contentType}); err != nil {
		return "", err
	}
	return s.sign(ctx, objectKey, grant)
}

func (s *AliyunPreviewStore) Cleanup(ctx context.Context) error {
	if s == nil || s.cache.db == nil {
		return entity.ErrUnavailable
	}
	records, err := s.cache.cleanupCandidates(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if !strings.HasPrefix(record.ObjectKey, s.prefix+"/"+s.identity+"/") {
			continue
		}
		if err := s.bucket.DeleteObject(record.ObjectKey, oss.WithContext(ctx)); err != nil {
			continue
		}
		if err := s.cache.forgetObject(ctx, record.ObjectKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *AliyunPreviewStore) sign(ctx context.Context, objectKey string, grant entity.Grant) (string, error) {
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
	signedURL, err := s.signer.SignURL(objectKey, oss.HTTPGet, remaining)
	if err != nil {
		return "", gerror.Wrap(entity.ErrUnavailable, "sign OSS object")
	}
	// SDK 自取时钟；跨秒/调度延迟不能让目标短链超出授权绝对期限。
	parsed, err := url.Parse(signedURL)
	if err != nil {
		return "", entity.ErrUnavailable
	}
	expires, err := strconv.ParseInt(parsed.Query().Get("Expires"), 10, 64)
	if err != nil || expires > grant.ExpiresAt || expires > record.ExpiresAt {
		return "", entity.ErrUnavailable
	}
	return signedURL, nil
}

func (s *AliyunPreviewStore) download(ctx context.Context, grant entity.Grant) ([]byte, string, error) {
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
	if coreOffice(grant.Filename) {
		if err := validateOffice(ctx, body, strings.ToLower(path.Ext(grant.Filename))); err != nil {
			return nil, "", err
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
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif":
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
	case ".avif":
		return contentType == "image/avif"
	default:
		return false
	}
}
