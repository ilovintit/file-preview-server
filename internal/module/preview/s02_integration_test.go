//go:build integration

package preview_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var rawPDF = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Count 0/Kids[]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")

func TestTC_S02_AC02_ControlledOSSNavigation(t *testing.T) {
	var downloads atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fixture.pdf" {
			http.NotFound(w, r)
			return
		}
		downloads.Add(1)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())

	resource := resource()
	resource["url"] = source.URL + "/fixture.pdf"
	resource["content_sha256"] = strings.Repeat("a", 64)
	badToken := issueResource(t, f, resource)
	badPreview, err := f.server.Client().Get(f.server.URL + "/v/" + badToken)
	if err != nil {
		t.Fatal(err)
	}
	badPreview.Body.Close()
	if badPreview.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("S02 hash guard expected 422, got %d", badPreview.StatusCode)
	}

	resource["content_sha256"] = sha256Hex(rawPDF)
	token := issueResource(t, f, resource)
	client := f.server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	preview, err := client.Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer preview.Body.Close()
	if preview.StatusCode != http.StatusFound {
		t.Fatalf("TC:S02-AC02:controlled_oss_navigation expected 302, got %d", preview.StatusCode)
	}
	if preview.Header.Get("Cache-Control") != "no-store" || preview.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("redirect security headers are missing")
	}
	location := preview.Header.Get("Location")
	if location == "" || strings.Contains(location, source.URL) {
		t.Fatal("redirect must be a signed OSS target, never the source URL")
	}

	object, err := http.Get(location)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	got, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatal(err)
	}
	if object.StatusCode != http.StatusOK || !bytes.Equal(got, rawPDF) || !strings.HasPrefix(object.Header.Get("Content-Type"), "application/pdf") {
		t.Fatal("OSS target did not return the original PDF")
	}
	corsPolicy := os.Getenv("PREVIEW_CI_OSS_CORS_ALLOWED_ORIGIN")
	if corsPolicy != "*" {
		t.Fatal("S02 integration requires PREVIEW_CI_OSS_CORS_ALLOWED_ORIGIN")
	}
	const browserOrigin = "https://preview-e2e.local"
	rangeRequest, err := http.NewRequest(http.MethodGet, location, nil)
	if err != nil {
		t.Fatal(err)
	}
	rangeRequest.Header.Set("Origin", browserOrigin)
	rangeRequest.Header.Set("Range", "bytes=0-3")
	rangeResponse, err := http.DefaultClient.Do(rangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	rangeResponse.Body.Close()
	if rangeResponse.StatusCode != http.StatusPartialContent {
		t.Errorf("single Range: expected 206, got %d", rangeResponse.StatusCode)
	}
	if origin := rangeResponse.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("CORS: expected wildcard response, got %q", origin)
	}
	exposed := rangeResponse.Header.Get("Access-Control-Expose-Headers")
	readable := false
	for _, header := range strings.Split(exposed, ",") {
		if strings.EqualFold(strings.TrimSpace(header), "Content-Range") || strings.TrimSpace(header) == "*" {
			readable = true
		}
	}
	if !readable {
		t.Errorf("CORS: Content-Range is not exposed; exposed headers=%q", exposed)
	}

	secondToken := issueResource(t, f, resource)
	second, err := client.Get(f.server.URL + "/v/" + secondToken)
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != http.StatusFound || downloads.Load() != 2 {
		t.Fatal("a cache hit must sign the existing OSS object without another source download")
	}
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func TestTC_S02_AC01_RejectSpoofedPDF(t *testing.T) {
	value := []byte("this is not a PDF")
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(value)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(value)
	token := issueResource(t, f, input)
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 422 {
		t.Fatalf("spoofed PDF expected422 got%d", r.StatusCode)
	}
}

func TestTC_S02_AC04_ConcurrentPreparation(t *testing.T) {
	var downloads atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		select {
		case <-time.After(200 * time.Millisecond):
		case <-r.Context().Done():
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
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := client.Get(f.server.URL + "/v/" + token)
			if err != nil {
				t.Error("preview request failed")
				return
			}
			r.Body.Close()
			if r.StatusCode != 302 {
				t.Errorf("preview expected302 got%d", r.StatusCode)
			}
		}()
	}
	wg.Wait()
	if n := downloads.Load(); n != 1 {
		t.Fatalf("same identity downloaded %d times, want1", n)
	}
}

func TestTC_S02_AC05_RejectHTTPSDowngrade(t *testing.T) {
	var plainRequests atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainRequests.Add(1)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer plain.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, 302)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(rawPDF)
	token := issueResource(t, f, input)
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Get(f.server.URL + "/v/" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode < 500 || plainRequests.Load() != 0 {
		t.Fatalf("downgrade must fail before HTTP request: status=%d requests=%d", r.StatusCode, plainRequests.Load())
	}
}

func TestTC_S02_AC03_RevokeDuringDownload(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
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
	result := make(chan int, 1)
	go func() {
		r, err := client.Get(f.server.URL + "/v/" + token)
		if err != nil {
			result <- 0
			return
		}
		r.Body.Close()
		result <- r.StatusCode
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("download did not start")
	}
	r, _, _ := f.call("/admin/tokens/"+token+"/revoke", "admin", map[string]any{})
	close(release)
	if r.StatusCode != 200 {
		t.Fatal("revoke failed")
	}
	select {
	case code := <-result:
		if code != 404 {
			t.Fatalf("revoked download expected404 got%d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("preview did not finish")
	}
}

func issueResource(t *testing.T, f *fixture, resource map[string]any) string {
	t.Helper()
	body, err := json.Marshal(resource)
	if err != nil {
		t.Fatal(err)
	}
	r, response, _ := f.request("/internal/tokens", "internal", body, nil)
	if r.StatusCode != http.StatusOK || response.Code != 0 {
		t.Fatalf("token issue expected 200/code0, got %d/code%d", r.StatusCode, response.Code)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(response.Data, &data); err != nil || data.Token == "" {
		t.Fatal("missing token in issue response")
	}
	return data.Token
}
