package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/domain/entity"
	s3 "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func siloProfileIdentity(c SiloConfig) string {
	digest := sha256.Sum256([]byte("silo\n" + c.Endpoint + "\n" + c.Bucket + "\n" + c.PrefixBase + "\n" + c.Generation))
	return hex.EncodeToString(digest[:])
}

type siloObjectStorage struct {
	client, signer *s3.Client
	bucket         string
}

func NewSiloPreviewStore(cfg Config, cache *ValkeyStore, clock func() time.Time) (*PreviewStore, error) {
	c := cfg.Silo
	if !c.configured() {
		return nil, nil
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	region := c.Region
	if region == "" {
		region = "us-east-1"
	}
	makeClient := func(endpoint string) (*s3.Client, error) {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, entity.ErrUnavailable
		}
		return s3.New(u.Host, &s3.Options{Creds: credentials.NewStaticV4(c.AccessKeyID, c.AccessKeySecret, c.SecurityToken), Secure: u.Scheme == "https", Region: region, BucketLookup: s3.BucketLookupPath, Transport: cfg.SiloTransport, MaxRetries: 1})
	}
	client, err := makeClient(c.Endpoint)
	if err != nil {
		return nil, entity.ErrUnavailable
	}
	signer, err := makeClient(c.previewEndpoint())
	if err != nil {
		return nil, entity.ErrUnavailable
	}
	storage := &siloObjectStorage{client: client, signer: signer, bucket: c.Bucket}
	return newPreviewStore(cfg, cache, clock, storage, c.PrefixBase, siloProfileIdentity(c), c.SignedURLMaxTTL), nil
}

func (s *siloObjectStorage) Put(ctx context.Context, key, contentType string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(body), int64(len(body)), s3.PutObjectOptions{ContentType: contentType, ContentDisposition: "inline", DisableMultipart: true})
	if err != nil {
		return entity.ErrUnavailable
	}
	return nil
}
func (s *siloObjectStorage) Stat(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, err := s.client.StatObject(ctx, s.bucket, key, s3.StatObjectOptions{})
	if err == nil {
		return nil
	}
	code := s3.ToErrorResponse(err).Code
	if code == "NoSuchKey" || code == "NoSuchObject" || code == "NotFound" {
		return entity.ErrNotFound
	}
	return entity.ErrUnavailable
}
func (s *siloObjectStorage) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := s.client.RemoveObject(ctx, s.bucket, key, s3.RemoveObjectOptions{}); err != nil {
		return entity.ErrUnavailable
	}
	return nil
}
func (s *siloObjectStorage) SignGet(ctx context.Context, key string, ttl int64) (string, int64, error) {
	u, err := s.signer.PresignedGetObject(ctx, s.bucket, key, time.Duration(ttl)*time.Second, nil)
	if err != nil {
		return "", 0, entity.ErrUnavailable
	}
	created, err := time.Parse("20060102T150405Z", u.Query().Get("X-Amz-Date"))
	if err != nil {
		return "", 0, entity.ErrUnavailable
	}
	duration, err := strconv.ParseInt(u.Query().Get("X-Amz-Expires"), 10, 64)
	if err != nil || duration < 1 || duration > 604800 {
		return "", 0, entity.ErrUnavailable
	}
	return u.String(), created.Unix() + duration, nil
}
func (s *siloObjectStorage) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil || !ok {
		return entity.ErrUnavailable
	}
	return nil
}
