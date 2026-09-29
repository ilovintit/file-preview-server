//go:build integration

package preview_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/infrastructure"
	s3 "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type siloFixture struct {
	cfg   infrastructure.SiloConfig
	api   *s3.Client
	proxy *httptest.Server
}

func newSiloFixture(t *testing.T) *siloFixture {
	t.Helper()
	endpoint, key, secret := os.Getenv("SILO_TEST_ENDPOINT"), os.Getenv("SILO_TEST_ACCESS_KEY"), os.Getenv("SILO_TEST_SECRET_KEY")
	if endpoint == "" || key == "" || secret == "" {
		t.Fatal("S04 requires isolated real silo service configuration")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal("invalid silo fixture endpoint")
	}
	api, err := s3.New(u.Host, &s3.Options{Creds: credentials.NewStaticV4(key, secret, ""), Secure: u.Scheme == "https", Region: "us-east-1", BucketLookup: s3.BucketLookupPath, MaxRetries: 1})
	if err != nil {
		t.Fatal("invalid silo fixture client")
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorLog = log.New(io.Discard, "", 0)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) { w.WriteHeader(503) }
	// Preserve Host: SigV4 is verified by the actual silo server against the
	// public TLS endpoint's signed Host. No signature or CORS rewriting.
	tlsProxy := httptest.NewTLSServer(proxy)
	t.Cleanup(tlsProxy.Close)
	bucket := fmt.Sprintf("preview-ci-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		err = api.MakeBucket(ctx, bucket, s3.MakeBucketOptions{Region: "us-east-1"})
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("silo fixture bucket creation failed")
		}
		time.Sleep(200 * time.Millisecond)
	}
	f := &siloFixture{cfg: infrastructure.SiloConfig{Endpoint: endpoint, PreviewEndpoint: tlsProxy.URL, Region: "us-east-1", Bucket: bucket, PrefixBase: "ci/preview", AccessKeyID: key, AccessKeySecret: secret, SignedURLMaxTTL: 60}, api: api, proxy: tlsProxy}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if !strings.HasPrefix(bucket, "preview-ci-") {
			t.Error("refusing non-fixture bucket cleanup")
			return
		}
		for object := range api.ListObjects(ctx, bucket, s3.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				t.Error("silo fixture listing failed")
				return
			}
			if err := api.RemoveObject(ctx, bucket, object.Key, s3.RemoveObjectOptions{}); err != nil {
				t.Error("silo fixture deletion failed")
				return
			}
		}
		if err := api.RemoveBucket(ctx, bucket); err != nil {
			t.Error("silo fixture bucket cleanup failed")
		}
	})
	return f
}

func (s *siloFixture) allowOrigin(t *testing.T, origin string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := s.api.SetBucketCors(ctx, s.cfg.Bucket, cors.NewConfig([]cors.Rule{{AllowedOrigin: []string{origin}, AllowedMethod: []string{"GET", "HEAD"}, AllowedHeader: []string{"Range"}, ExposeHeader: []string{"Content-Range", "Accept-Ranges", "Content-Length", "Content-Type"}}}))
	if err != nil {
		t.Fatal("real silo CORS configuration failed")
	}
}

func setupSilo(t *testing.T, source *http.Client) (*fixture, *siloFixture) {
	t.Helper()
	silo := newSiloFixture(t)
	f := setupWithOSS(t, testAliyunOSS(t), source, func(cfg *infrastructure.Config) {
		cfg.Silo = silo.cfg
		cfg.GotenbergURL = os.Getenv("GOTENBERG_TEST_URL")
	})
	silo.allowOrigin(t, f.server.URL)
	return f, silo
}
