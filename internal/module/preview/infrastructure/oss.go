package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gogf/gf/v2/errors/gerror"
)

const rawOutputVersion = "raw-v1"

type AliyunPreviewStore struct {
	bucket       *oss.Bucket
	cache        *ValkeyStore
	clock        func() time.Time
	sourceClient *http.Client
	prefix       string
	identity     string
	signedURLMax int64
}

func NewAliyunPreviewStore(cfg Config, cache *ValkeyStore, clock func() time.Time) (*AliyunPreviewStore, error) {
	c := cfg.AliyunOSS
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKeyID == "" || c.AccessKeySecret == "" {
		return nil, nil
	}
	options := make([]oss.ClientOption, 0, 1)
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
	if c.SignedURLMaxTTL < 1 || c.PrefixBase == "" {
		return nil, gerror.Wrap(entity.ErrUnavailable, "invalid OSS configuration")
	}
	source := cfg.SourceHTTPClient
	if source == nil {
		source = &http.Client{Timeout: 20 * time.Second}
	}
	configDigest := sha256.Sum256([]byte(c.Endpoint + "\n" + c.Bucket + "\n" + c.PrefixBase))
	return &AliyunPreviewStore{bucket: bucket, cache: cache, clock: clock, sourceClient: source, prefix: strings.Trim(c.PrefixBase, "/"), identity: hex.EncodeToString(configDigest[:8]), signedURLMax: c.SignedURLMaxTTL}, nil
}

func (s *AliyunPreviewStore) Prepare(ctx context.Context, grant entity.Grant) (string, error) {
	cacheKey := s.identity + ":" + rawOutputVersion + ":" + grant.ContentSHA256
	if record, err := s.cache.GetCache(ctx, cacheKey); err == nil {
		if _, headErr := s.bucket.GetObjectDetailedMeta(record.ObjectKey); headErr == nil {
			return s.sign(record.ObjectKey, grant)
		}
	} else if err != entity.ErrNotFound {
		return "", err
	}
	body, contentType, err := s.download(ctx, grant)
	if err != nil {
		return "", err
	}
	objectKey := fmt.Sprintf("%s/%s/%s/%s", s.prefix, s.identity, rawOutputVersion, grant.ContentSHA256)
	if err = s.bucket.PutObject(objectKey, bytes.NewReader(body), oss.ContentType(contentType), oss.ContentDisposition("inline")); err != nil {
		return "", gerror.Wrap(entity.ErrUnavailable, "upload OSS object")
	}
	if err = s.cache.PutCache(ctx, cacheKey, cacheRecord{ObjectKey: objectKey, ExpiresAt: grant.CacheExpiresAt}); err != nil {
		return "", err
	}
	return s.sign(objectKey, grant)
}

func (s *AliyunPreviewStore) sign(objectKey string, grant entity.Grant) (string, error) {
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
	url, err := s.bucket.SignURL(objectKey, oss.HTTPGet, remaining)
	if err != nil {
		return "", gerror.Wrap(entity.ErrUnavailable, "sign OSS object")
	}
	return url, nil
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
	if err != nil || len(body) > 32<<20 {
		return nil, "", gerror.Wrap(entity.ErrUnavailable, "source body")
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != grant.ContentSHA256 {
		return nil, "", entity.ErrInvalid
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if !rawMIME(path.Ext(grant.Filename), contentType) {
		return nil, "", entity.ErrInvalid
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
