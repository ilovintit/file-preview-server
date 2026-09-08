//go:build integration

package preview_test

import (
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

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
