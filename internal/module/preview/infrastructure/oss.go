package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/ilovintit/file-preview-server/internal/module/preview/domain/entity"
)

func ossProfileIdentity(c AliyunOSSConfig) string {
	digest := sha256.Sum256([]byte(c.Endpoint + "\n" + c.Bucket + "\n" + c.PrefixBase))
	return hex.EncodeToString(digest[:])
}

type aliyunObjectStorage struct {
	bucket, signer *oss.Bucket
	prefix         string
}

func NewAliyunPreviewStore(cfg Config, cache *ValkeyStore, clock func() time.Time) (*PreviewStore, error) {
	c := cfg.AliyunOSS
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKeyID == "" || c.AccessKeySecret == "" {
		return nil, nil
	}
	if c.SignedURLMaxTTL < 1 || c.PrefixBase == "" {
		return nil, entity.ErrUnavailable
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
		return nil, entity.ErrUnavailable
	}
	client.SetRegion(c.Region)
	bucket, err := client.Bucket(c.Bucket)
	if err != nil {
		return nil, entity.ErrUnavailable
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
	storage := &aliyunObjectStorage{bucket: bucket, signer: signer, prefix: strings.Trim(c.PrefixBase, "/")}
	return newPreviewStore(cfg, cache, clock, storage, c.PrefixBase, ossProfileIdentity(c), c.SignedURLMaxTTL), nil
}

func (s *aliyunObjectStorage) Put(ctx context.Context, key, contentType string, body []byte) error {
	if err := s.bucket.PutObject(key, bytes.NewReader(body), oss.ContentType(contentType), oss.ContentDisposition("inline"), oss.WithContext(ctx)); err != nil {
		return entity.ErrUnavailable
	}
	return nil
}
func (s *aliyunObjectStorage) Stat(ctx context.Context, key string) error {
	_, err := s.bucket.GetObjectDetailedMeta(key, oss.WithContext(ctx))
	if err == nil {
		return nil
	}
	var serviceError oss.ServiceError
	if errors.As(err, &serviceError) && serviceError.StatusCode == 404 && serviceError.Code != "NoSuchBucket" {
		return entity.ErrNotFound
	}
	return entity.ErrUnavailable
}
func (s *aliyunObjectStorage) Delete(ctx context.Context, key string) error {
	if err := s.bucket.DeleteObject(key, oss.WithContext(ctx)); err != nil {
		return entity.ErrUnavailable
	}
	return nil
}
func (s *aliyunObjectStorage) SignGet(ctx context.Context, key string, ttl int64) (string, int64, error) {
	if ctx.Err() != nil {
		return "", 0, entity.ErrUnavailable
	}
	location, err := s.signer.SignURL(key, oss.HTTPGet, ttl)
	if err != nil {
		return "", 0, entity.ErrUnavailable
	}
	u, err := url.Parse(location)
	if err != nil {
		return "", 0, entity.ErrUnavailable
	}
	expires, err := strconv.ParseInt(u.Query().Get("Expires"), 10, 64)
	if err != nil {
		return "", 0, entity.ErrUnavailable
	}
	return location, expires, nil
}
func (s *aliyunObjectStorage) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// Stay within the application's object-read permissions. Only an explicit
	// NoSuchKey proves the bucket exists; an unclassified 404 is not healthy.
	_, err := s.bucket.GetObjectDetailedMeta(s.prefix+"/.preview-readiness", oss.WithContext(ctx))
	if err == nil {
		return nil
	}
	var providerError oss.ServiceError
	if errors.As(err, &providerError) && providerError.StatusCode == 404 && providerError.Code == "NoSuchKey" {
		return nil
	}
	return entity.ErrUnavailable
}
