//go:build integration

package preview_test

import (
	"context"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTC_S02_AC06_UnconfiguredStorageFailsClosed(t *testing.T) {
	f := setupWithOSS(t, infrastructure.AliyunOSSConfig{}, nil)
	r, _, _ := f.call("/internal/tokens", "internal", resource())
	if r.StatusCode != 503 {
		t.Fatalf("unconfigured OSS must reject issuance: expected503 got%d", r.StatusCode)
	}
}

func TestTC_S02_AC05_MissingObjectRebuildKeepsDeadline(t *testing.T) {
	var missing atomic.Bool
	var downloads atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		if missing.Load() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(rawPDF)
	token := issueResource(t, f, input)
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	preview := func() (int, string) {
		t.Helper()
		r, err := client.Get(f.server.URL + "/v/" + token)
		if err != nil {
			t.Fatal("preview transport failed")
		}
		defer r.Body.Close()
		return r.StatusCode, r.Header.Get("Location")
	}
	code, location := preview()
	if code != 302 {
		t.Fatalf("first preview got%d", code)
	}
	objectURL, err := url.Parse(location)
	if err != nil {
		t.Fatal("invalid object URL")
	}
	ctx := context.Background()
	keys, err := f.db.Do(ctx, "KEYS", f.cfg.Namespace+":auth:*:deadline:*")
	if err != nil || len(keys.Strings()) != 1 {
		t.Fatal("missing deadline")
	}
	before, err := f.db.Do(ctx, "GET", keys.Strings()[0])
	if err != nil {
		t.Fatal("deadline lookup failed")
	}
	bucket := fixtureOSSBucket(t, f.cfg.AliyunOSS)
	if err = bucket.DeleteObject(strings.TrimPrefix(objectURL.Path, "/"), oss.WithContext(ctx)); err != nil {
		t.Fatal("external object deletion failed")
	}
	f.now.Add(5)
	code, reconstructed := preview()
	if code != 302 {
		t.Fatalf("rebuild got%d", code)
	}
	newObjectURL, err := url.Parse(reconstructed)
	if err != nil {
		t.Fatal("invalid rebuilt URL")
	}
	if newObjectURL.Path == objectURL.Path || downloads.Load() != 2 {
		t.Fatal("rebuild must download once into a fresh generation")
	}
	after, err := f.db.Do(ctx, "GET", keys.Strings()[0])
	if err != nil || after.Int64() != before.Int64() {
		t.Fatal("rebuild renewed cache deadline")
	}
	if err = bucket.DeleteObject(strings.TrimPrefix(newObjectURL.Path, "/"), oss.WithContext(ctx)); err != nil {
		t.Fatal("second external deletion failed")
	}
	missing.Store(true)
	code, location = preview()
	if code < 500 || location != "" {
		t.Fatalf("missing source during rebuild must fail closed, got%d", code)
	}
	after, err = f.db.Do(ctx, "GET", keys.Strings()[0])
	if err != nil || after.Int64() != before.Int64() {
		t.Fatal("failed rebuild renewed cache deadline")
	}
}

func TestTC_S02_AC06_SignedHeadAndExpiry(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	config := testAliyunOSS(t)
	config.SignedURLMaxTTL = 2
	f := setupWithOSS(t, config, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(rawPDF)
	token := issueResource(t, f, input)
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Fatal("preview request failed")
	}
	r.Body.Close()
	if r.StatusCode != 302 {
		t.Fatal("preview did not redirect")
	}
	location := r.Header.Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal("invalid signed URL")
	}
	object := strings.TrimPrefix(parsed.Path, "/")
	expires, err := strconv.ParseInt(parsed.Query().Get("Expires"), 10, 64)
	if err != nil || expires > f.now.Load()+60 || expires > time.Now().Unix()+2 {
		t.Fatal("signed URL exceeds token or provider lifetime")
	}
	bucket := fixtureOSSBucket(t, config)
	headURL, err := bucket.SignURL(object, oss.HTTPHead, 5)
	if err != nil {
		t.Fatal("HEAD signing failed")
	}
	req, err := http.NewRequest(http.MethodHead, headURL, nil)
	if err != nil {
		t.Fatal("HEAD request construction failed")
	}
	head, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("HEAD request failed")
	}
	head.Body.Close()
	if head.StatusCode != 200 || head.ContentLength != int64(len(rawPDF)) {
		t.Fatal("HEAD metadata mismatch")
	}
	// 实际 provider 时钟下的短链到期，而非只检查查询字符串。
	time.Sleep(3 * time.Second)
	expired, err := http.Get(location)
	if err != nil {
		t.Fatal("expiry probe request failed")
	}
	expired.Body.Close()
	if expired.StatusCode != 403 {
		t.Fatalf("expired signed URL expected403 got%d", expired.StatusCode)
	}
}

func TestTC_S02_AC05_Source404IsNotToken404(t *testing.T) {
	source := httptest.NewTLSServer(http.NotFoundHandler())
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(rawPDF)
	token := issueResource(t, f, input)
	r, err := f.server.Client().Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode < 500 {
		t.Fatalf("missing source must be5xx, got%d", r.StatusCode)
	}
}
