//go:build integration

package preview_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	preview "git.shw.top/shw-project/file-preview-server/internal/module/preview"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	s3 "github.com/minio/minio-go/v7"
)

func TestTC_S04_AC03_ProfileGenerationAndConfig(t *testing.T) {
	for _, change := range []string{"bucket", "generation"} {
		t.Run(change, func(t *testing.T) {
			var downloads atomic.Int32
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downloads.Add(1)
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write(rawPDF)
			}))
			defer source.Close()
			f, first := setupSilo(t, source.Client())
			input := resource()
			input["storage_profile"], input["url"], input["content_sha256"] = "silo", source.URL, sha256Hex(rawPDF)
			code, oldLocation := profileLocation(t, f, issueResource(t, f, input))
			if code != 302 {
				t.Fatal("initial profile failed")
			}
			cfg := f.cfg
			if change == "bucket" {
				second := newSiloFixture(t)
				cfg.Silo = second.cfg
			} else {
				cfg.Silo.Generation = "next"
			}
			m, err := preview.New(cfg, func() time.Time { return time.Unix(f.now.Load(), 0) })
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewTLSServer(m.Handler())
			defer server.Close()
			defer m.Close(context.Background())
			next := &fixture{t: t, cfg: cfg, db: f.db, module: m, server: server}
			next.now.Store(f.now.Load())
			next.nonce.Store(f.nonce.Load() + 1000)
			code, newLocation := profileLocation(t, next, issueResource(t, next, input))
			if code != 302 {
				t.Fatalf("new config expected302 got%d", code)
			}
			if oldLocation == newLocation || downloads.Load() != 2 || profileID(f.cfg, "silo") == profileID(cfg, "silo") {
				t.Fatal("new config reused old cache or preparation")
			}
			oldURL, _ := url.Parse(oldLocation)
			key := strings.TrimPrefix(oldURL.Path, "/"+first.cfg.Bucket+"/")
			if _, err := first.api.StatObject(context.Background(), first.cfg.Bucket, key, s3.StatObjectOptions{}); err != nil {
				t.Fatal("config switch removed old bucket object")
			}
		})
	}
	t.Run("unknown-and-unconfigured", func(t *testing.T) {
		f := setup(t)
		for _, tc := range []struct {
			profile string
			status  int
		}{{"third-provider", 422}, {"silo", 503}} {
			input := resource()
			input["storage_profile"] = tc.profile
			r, _, _ := f.call("/internal/tokens", "internal", input)
			if r.StatusCode != tc.status {
				t.Fatalf("profile %s expected%d got%d", tc.profile, tc.status, r.StatusCode)
			}
		}
	})
}

func TestTC_S04_AC04_SiloDeleteFailureAndInactiveQueue(t *testing.T) {
	var deny atomic.Bool
	deny.Store(true)
	transport := transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodDelete && deny.Load() {
			return &http.Response{StatusCode: 403, Status: "403 Forbidden", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("<Error><Code>AccessDenied</Code></Error>")), Request: r}, nil
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	silo := newSiloFixture(t)
	f := setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.Silo = silo.cfg; c.SiloTransport = transport })
	input := resource()
	input["storage_profile"], input["url"], input["content_sha256"] = "silo", source.URL, sha256Hex(rawPDF)
	code, location := profileLocation(t, f, issueResource(t, f, input))
	if code != 302 {
		t.Fatal("silo preparation failed")
	}
	u, _ := url.Parse(location)
	key := strings.TrimPrefix(u.Path, "/"+silo.cfg.Bucket+"/")
	ctx := context.Background()
	store, err := infrastructure.NewValkeyStore(f.cfg, func() time.Time { return time.Unix(f.now.Load(), 0) })
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close(ctx)
	preparer, err := infrastructure.NewProfilePreparer(f.cfg, store, func() time.Time { return time.Unix(f.now.Load(), 0) })
	if err != nil {
		t.Fatal(err)
	}
	// More than one due page from an inactive configuration must not starve
	// this silo profile or be claimed/deleted by it. These are synthetic
	// maintenance entries only, not objects in an external bucket.
	foreign := strings.Repeat("f", 64)
	for i := 0; i < 70; i++ {
		object := fmt.Sprintf("ci/preview/%s/raw-v1/%03d/owner", foreign, i)
		record, _ := json.Marshal(map[string]any{"identity": foreign + ":raw-v1:hash", "epoch_prefix": "inactive:", "object_key": object, "not_before": f.now.Load() - 1})
		if _, err := f.db.Do(ctx, "HSET", f.cfg.Namespace+":objects", object, string(record)); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Do(ctx, "ZADD", f.cfg.Namespace+":objects:due", f.now.Load()-1, object); err != nil {
			t.Fatal(err)
		}
	}
	f.now.Add(121)
	for i := 0; i < 10; i++ {
		if err := preparer.Cleanup(ctx); err != nil {
			t.Fatal("cleanup failed")
		}
	}
	if _, err := silo.api.StatObject(ctx, silo.cfg.Bucket, key, s3.StatObjectOptions{}); err != nil {
		t.Fatal("denied delete removed object")
	}
	count, err := f.db.Do(ctx, "HLEN", f.cfg.Namespace+":objects")
	if err != nil || count.Int() != 71 {
		t.Fatal("failed delete lost maintenance or touched inactive config")
	}
	deny.Store(false)
	f.now.Add(31)
	for i := 0; i < 10; i++ {
		if err := preparer.Cleanup(ctx); err != nil {
			t.Fatal("cleanup recovery failed")
		}
	}
	if _, err := silo.api.StatObject(ctx, silo.cfg.Bucket, key, s3.StatObjectOptions{}); err == nil {
		t.Fatal("silo object not deleted after recovery")
	}
	count, err = f.db.Do(ctx, "HLEN", f.cfg.Namespace+":objects")
	if err != nil || count.Int() != 70 {
		t.Fatal("cleanup touched inactive profile records")
	}
}

func TestTC_S04_AC03_ReadinessChecksActualProfiles(t *testing.T) {
	f, silo := setupSilo(t, nil)
	r, err := f.server.Client().Get(f.server.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		bucket := fixtureOSSBucket(t, f.cfg.AliyunOSS)
		_, err := bucket.GetObjectDetailedMeta(strings.TrimSuffix(f.cfg.AliyunOSS.PrefixBase, "/") + "/.preview-readiness")
		var serviceError oss.ServiceError
		if errors.As(err, &serviceError) {
			t.Logf("readiness OSS object probe status=%d code=%s", serviceError.StatusCode, serviceError.Code)
		} else {
			t.Logf("readiness OSS object probe ok=%t", err == nil)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ok, err := silo.api.BucketExists(ctx, f.cfg.Silo.Bucket)
		t.Logf("readiness silo bucket exists=%t code=%s", ok, s3.ToErrorResponse(err).Code)
		probe, err := (&http.Client{Timeout: 2 * time.Second}).Get(f.cfg.GotenbergURL + "/health")
		if err == nil {
			t.Logf("readiness converter status=%d", probe.StatusCode)
			probe.Body.Close()
		} else {
			t.Log("readiness converter connection failed")
		}
		t.Fatalf("configured actual dependencies expectedready200 got%d", r.StatusCode)
	}
	for _, profile := range []string{"silo", "aliyun-oss"} {
		t.Run(profile+"-missing-bucket", func(t *testing.T) {
			cfg := f.cfg
			if profile == "silo" {
				cfg.Silo.Bucket += "-missing"
			} else {
				cfg.AliyunOSS.Bucket += "-missing"
			}
			m, err := preview.New(cfg, func() time.Time { return time.Unix(f.now.Load(), 0) })
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close(context.Background())
			server := httptest.NewTLSServer(m.Handler())
			defer server.Close()
			r, err := server.Client().Get(server.URL + "/readyz")
			if err != nil {
				t.Fatal(err)
			}
			r.Body.Close()
			if r.StatusCode != 503 {
				t.Fatalf("missing bucket expectedready503 got%d", r.StatusCode)
			}
		})
	}
}
