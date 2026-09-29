//go:build integration

package preview_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/infrastructure"
	s3 "github.com/minio/minio-go/v7"
)

func profileLocation(t *testing.T, f *fixture, token string) (int, string) {
	t.Helper()
	client := *f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Error("profile navigation failed")
		return 0, ""
	}
	r.Body.Close()
	return r.StatusCode, r.Header.Get("Location")
}

func profileID(cfg infrastructure.Config, profile string) string {
	if profile == "silo" {
		c := cfg.Silo
		return sha256Hex([]byte("silo\n" + c.Endpoint + "\n" + c.Bucket + "\n" + c.PrefixBase + "\n" + c.Generation))
	}
	c := cfg.AliyunOSS
	return sha256Hex([]byte(c.Endpoint + "\n" + c.Bucket + "\n" + c.PrefixBase))
}

func profileDeadline(t *testing.T, f *fixture, profile string) int64 {
	t.Helper()
	keys, err := f.db.Do(context.Background(), "KEYS", f.cfg.Namespace+":auth:*:deadline:"+profileID(f.cfg, profile)+":*")
	if err != nil || len(keys.Strings()) != 1 {
		t.Fatal("expected one isolated profile deadline")
	}
	deadline, err := f.db.Do(context.Background(), "GET", keys.Strings()[0])
	if err != nil {
		t.Fatal("deadline lookup failed")
	}
	return deadline.Int64()
}

func TestTC_S04_AC02_ProfileCacheLockIsolation(t *testing.T) {
	for _, format := range []string{"pdf", "docx"} {
		t.Run(format, func(t *testing.T) {
			body, mediaType := rawPDF, "application/pdf"
			if format == "docx" {
				body, mediaType = officeInput(t, "document.docx"), "application/octet-stream"
			}
			var downloads, conversions atomic.Int32
			entered, release := make(chan struct{}, 2), make(chan struct{})
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if downloads.Add(1) <= 2 {
					entered <- struct{}{}
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				w.Header().Set("Content-Type", mediaType)
				_, _ = w.Write(body)
			}))
			defer source.Close()
			proxy := realOfficeProxy(t)
			converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/forms/libreoffice/convert" {
					conversions.Add(1)
				}
				proxy.ServeHTTP(w, r)
			}))
			defer converter.Close()
			silo := newSiloFixture(t)
			f := setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.Silo = silo.cfg; c.GotenbergURL = converter.URL })
			silo.allowOrigin(t, f.server.URL)
			profiles := []string{"aliyun-oss", "silo"}
			tokens, locations := make([]string, 2), make([]string, 2)
			for i, profile := range profiles {
				input := resource()
				input["storage_profile"], input["url"], input["filename"], input["content_sha256"] = profile, source.URL, "file."+format, sha256Hex(body)
				tokens[i] = issueResource(t, f, input)
			}
			var wg sync.WaitGroup
			for i := range profiles {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					code, location := profileLocation(t, f, tokens[i])
					if code != 302 {
						t.Errorf("profile %s expected302 got%d", profiles[i], code)
					}
					locations[i] = location
				}(i)
			}
			for i := 0; i < 2; i++ {
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Error("profiles did not independently acquire preparation locks")
				}
			}
			close(release)
			wg.Wait()
			if downloads.Load() != 2 {
				t.Fatalf("cross-profile preparation downloaded %d times instead of twice", downloads.Load())
			}
			if format == "docx" && conversions.Load() != 2 {
				t.Fatal("Office conversion was shared across profiles")
			}
			for i := range profiles {
				code, _ := profileLocation(t, f, tokens[i])
				if code != 302 {
					t.Fatal("profile cache not reusable")
				}
			}
			if downloads.Load() != 2 {
				t.Fatal("same profile cache missed")
			}
			for i, profile := range profiles {
				u, err := url.Parse(locations[i])
				if err != nil || u.Scheme != "https" {
					t.Fatal("invalid profile location")
				}
				if !strings.Contains(u.Path, "/"+profileID(f.cfg, profile)+"/") {
					t.Fatal("object belongs to wrong profile identity")
				}
			}
			if profileDeadline(t, f, "silo") != profileDeadline(t, f, "aliyun-oss") {
				t.Fatal("initial deadline mismatch")
			}
			ossDeadline := profileDeadline(t, f, "aliyun-oss")
			input := resource()
			input["storage_profile"], input["url"], input["filename"], input["content_sha256"], input["cache_ttl"] = "silo", source.URL, "file."+format, sha256Hex(body), 180
			_ = issueResource(t, f, input)
			if profileDeadline(t, f, "aliyun-oss") != ossDeadline || profileDeadline(t, f, "silo") != f.now.Load()+180 {
				t.Fatal("cross-profile deadline extension")
			}
			r, _, _ := f.call("/admin/tokens/"+tokens[1]+"/revoke", "admin", map[string]any{})
			if r.StatusCode != 200 {
				t.Fatal("revoke failed")
			}
			if code, _ := profileLocation(t, f, tokens[0]); code != 302 {
				t.Fatal("silo revoke affected OSS token")
			}
			store, err := infrastructure.NewValkeyStore(f.cfg, func() time.Time { return time.Unix(f.now.Load(), 0) })
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close(context.Background())
			preparer, err := infrastructure.NewProfilePreparer(f.cfg, store, func() time.Time { return time.Unix(f.now.Load(), 0) })
			if err != nil {
				t.Fatal(err)
			}
			f.now.Add(121)
			if err := preparer.Cleanup(context.Background()); err != nil {
				t.Fatal("dual profile cleanup failed")
			}
			ossURL, _ := url.Parse(locations[0])
			bucket := fixtureOSSBucket(t, f.cfg.AliyunOSS)
			exists, err := bucket.IsObjectExist(strings.TrimPrefix(ossURL.Path, "/"))
			if err != nil || exists {
				t.Fatal("expired OSS object remains")
			}
			siloURL, _ := url.Parse(locations[1])
			siloKey := strings.TrimPrefix(siloURL.Path, "/"+silo.cfg.Bucket+"/")
			if _, err := silo.api.StatObject(context.Background(), silo.cfg.Bucket, siloKey, s3.StatObjectOptions{}); err != nil {
				t.Fatal("OSS cleanup deleted live silo object")
			}
		})
	}
}

func TestTC_S04_AC04_SiloCORSRangeAndRealExpiry(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	silo := newSiloFixture(t)
	silo.cfg.SignedURLMaxTTL = 2
	f := setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.Silo = silo.cfg })
	silo.allowOrigin(t, f.server.URL)
	input := resource()
	input["storage_profile"], input["url"], input["content_sha256"] = "silo", source.URL, sha256Hex(rawPDF)
	code, location := profileLocation(t, f, issueResource(t, f, input))
	if code != 302 {
		t.Fatal("silo signature missing")
	}
	req, _ := http.NewRequest(http.MethodGet, location, nil)
	req.Header.Set("Origin", f.server.URL)
	req.Header.Set("Range", "bytes=0-3")
	r, err := silo.proxy.Client().Do(req)
	if err != nil {
		t.Fatal("silo range transport failed")
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil || r.StatusCode != 206 || string(body) != "%PDF" || r.Header.Get("Access-Control-Allow-Origin") != f.server.URL || !strings.Contains(r.Header.Get("Content-Range"), "bytes 0-3/") {
		t.Fatal("silo CORS/Range mismatch")
	}
	req, _ = http.NewRequest(http.MethodGet, location, nil)
	req.Header.Set("Origin", "https://not-allowed.invalid")
	r, err = silo.proxy.Client().Do(req)
	if err != nil {
		t.Fatal("CORS rejection probe failed")
	}
	r.Body.Close()
	if r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("silo CORS expanded allowed origins")
	}
	time.Sleep(3 * time.Second)
	r, err = silo.proxy.Client().Get(location)
	if err != nil {
		t.Fatal("silo expiry transport failed")
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatalf("expired silo signature expected403 got%d", r.StatusCode)
	}
}
