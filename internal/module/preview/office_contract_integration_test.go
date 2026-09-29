//go:build integration

package preview_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/infrastructure"
)

func officeInput(t *testing.T, file string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/office/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func officeSetup(t *testing.T, body []byte, filename, converter string) (*fixture, string) {
	t.Helper()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	}))
	t.Cleanup(source.Close)
	f := setupS02(t, source.Client(), func(c *infrastructure.Config) { c.GotenbergURL = converter })
	input := resource()
	input["url"], input["filename"], input["content_sha256"] = source.URL, filename, sha256Hex(body)
	return f, issueResource(t, f, input)
}

func officeStatus(f *fixture, token string, ctx context.Context) (int, error) {
	client := *f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, f.server.URL+"/v/"+token, nil)
	if err != nil {
		return 0, err
	}
	response, err := client.Do(r)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != 302 && response.Header.Get("Location") != "" {
		f.t.Error("failed conversion leaked Location")
	}
	return response.StatusCode, nil
}

func TestTC_S03_AC04_RejectBadOfficeInputs(t *testing.T) {
	var calls atomic.Int32
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer converter.Close()
	t.Run("password-protected-docx", func(t *testing.T) {
		body := officeInput(t, "protected.docx")
		if sha256Hex(body) != "690a393bc9e7f6a370c1644f76f39b6ff54d89649714a50bca7564770c19068e" {
			t.Fatal("protected fixture hash mismatch")
		}
		f, token := officeSetup(t, body, "protected.docx", converter.URL)
		code, err := officeStatus(f, token, context.Background())
		if err != nil || code != 422 {
			t.Fatalf("protected file expected422 got%d", code)
		}
	})
	for _, file := range []string{"document.doc", "document.docx", "sheet.xls", "sheet.xlsx", "slides.ppt", "slides.pptx"} {
		t.Run(file, func(t *testing.T) {
			f, token := officeSetup(t, []byte("not an Office file"), file, converter.URL)
			for i := 0; i < 2; i++ {
				code, err := officeStatus(f, token, context.Background())
				if err != nil || code != 422 {
					t.Fatalf("bad input expected422 got%d", code)
				}
			}
		})
	}
	for _, tc := range []struct{ input, filename string }{{"document.doc", "fake.xls"}, {"document.docx", "fake.xlsx"}, {"slides.ppt", "fake.doc"}, {"sheet.xlsx", "fake.pptx"}} {
		t.Run(tc.input+"-as-"+tc.filename, func(t *testing.T) {
			f, token := officeSetup(t, officeInput(t, tc.input), tc.filename, converter.URL)
			code, err := officeStatus(f, token, context.Background())
			if err != nil || code != 422 {
				t.Fatalf("mismatch expected422 got%d", code)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("invalid content reached converter")
	}
}

func TestTC_S03_AC04_ConverterFailureContract(t *testing.T) {
	for _, tc := range []struct {
		name             string
		status, expected int
		mime, body       string
	}{
		{"password-or-unprocessable", 400, 422, "text/plain", "private converter error"},
		{"unprocessable", 422, 422, "text/plain", "private converter error"},
		{"unavailable", 503, 503, "text/plain", "private converter error"},
		{"redirect", 302, 503, "text/plain", ""},
		{"not-pdf", 200, 503, "application/pdf", "%PDF-1.7\ntruncated"},
		{"wrong-mime", 200, 503, "text/html", "<html>failure</html>"},
		{"oversized-output", 200, 503, "application/pdf", strings.Repeat("x", 32<<20+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", tc.mime)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer converter.Close()
			f, token := officeSetup(t, officeInput(t, "document.docx"), "file.docx", converter.URL)
			for i := 0; i < 2; i++ {
				code, err := officeStatus(f, token, context.Background())
				if err != nil || code != tc.expected {
					t.Fatalf("expected%d got%d", tc.expected, code)
				}
			}
			if calls.Load() != 2 {
				t.Fatal("converter failure cached as ready")
			}
		})
	}
}

func realOfficeProxy(t *testing.T) *httputil.ReverseProxy {
	t.Helper()
	endpoint := os.Getenv("GOTENBERG_TEST_URL")
	if endpoint == "" {
		t.Fatal("actual Gotenberg required")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) { w.WriteHeader(503) }
	return proxy
}

func TestTC_S03_AC03_ConcurrentConversionShared(t *testing.T) {
	proxy := realOfficeProxy(t)
	var calls atomic.Int32
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); proxy.ServeHTTP(w, r) }))
	defer converter.Close()
	f, token := officeSetup(t, officeInput(t, "document.docx"), "file.docx", converter.URL)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, err := officeStatus(f, token, context.Background())
			if err != nil || code != 302 {
				t.Errorf("concurrent conversion expected302 got%d", code)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("same identity converted %d times", calls.Load())
	}
	code, err := officeStatus(f, token, context.Background())
	if err != nil || code != 302 || calls.Load() != 1 {
		t.Fatal("Office cache hit must bypass conversion")
	}
}

func TestTC_S03_AC05_AuthorizationDuringRealConversion(t *testing.T) {
	for _, action := range []string{"revoke", "expire", "cancel", "lease-loss", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			proxy := realOfficeProxy(t)
			proxy.ModifyResponse = func(r *http.Response) error {
				if r.StatusCode != 200 {
					t.Error("real conversion must succeed before race injection")
				}
				close(entered)
				select {
				case <-release:
					return nil
				case <-r.Request.Context().Done():
					return r.Request.Context().Err()
				}
			}
			converter := httptest.NewServer(proxy)
			defer converter.Close()
			f, token := officeSetup(t, officeInput(t, "document.docx"), "file.docx", converter.URL)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan int, 1)
			go func() { code, _ := officeStatus(f, token, ctx); result <- code }()
			select {
			case <-entered:
			case <-time.After(35 * time.Second):
				once.Do(func() { close(release) })
				t.Fatal("conversion not reached")
			}
			switch action {
			case "revoke":
				r, _, _ := f.call("/admin/tokens/"+token+"/revoke", "admin", map[string]any{})
				if r.StatusCode != 200 {
					t.Error("revoke failed")
				}
			case "expire":
				f.now.Add(61)
			case "cancel":
				cancel()
			case "lease-loss":
				keys, err := f.db.Do(context.Background(), "KEYS", f.cfg.Namespace+":auth:*:lease:*")
				if err != nil || len(keys.Strings()) != 1 {
					once.Do(func() { close(release) })
					t.Fatal("missing owner lease")
				}
				_, err = f.db.Do(context.Background(), "SET", keys.Strings()[0], "new-owner", "PX", 5000)
				if err != nil {
					once.Do(func() { close(release) })
					t.Fatal("cannot inject lease loss")
				}
				time.Sleep(1500 * time.Millisecond)
			case "shutdown":
				shutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
				if err := f.module.Close(shutdown); err != nil {
					t.Error("shutdown did not drain conversion")
				}
				cancelShutdown()
			}
			once.Do(func() { close(release) })
			select {
			case code := <-result:
				if action == "cancel" {
					if code != 0 {
						t.Errorf("canceled request returned%d", code)
					}
				} else if action == "lease-loss" || action == "shutdown" {
					if code != 503 {
						t.Errorf("stopped conversion returned%d", code)
					}
				} else if code != 404 {
					t.Errorf("invalid token returned%d", code)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("conversion did not stop")
			}
		})
	}
}

func TestTC_S03_AC02_RawBypassesConverter(t *testing.T) {
	var calls atomic.Int32
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer converter.Close()
	for ext, value := range rawFixtures(t) {
		t.Run(ext, func(t *testing.T) {
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", value.mime)
				_, _ = w.Write(value.body)
			}))
			defer source.Close()
			f := setupS02(t, source.Client(), func(c *infrastructure.Config) { c.GotenbergURL = converter.URL })
			input := resource()
			input["url"], input["filename"], input["content_sha256"] = source.URL, "file."+ext, sha256Hex(value.body)
			code, err := officeStatus(f, issueResource(t, f, input), context.Background())
			if err != nil || code != 302 {
				t.Fatalf("raw bypass expected302 got%d", code)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("raw format entered converter")
	}
}

func TestTC_S03_AC03_CancellationBoundsConversion(t *testing.T) {
	entered, stopped := make(chan struct{}), make(chan struct{})
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(stopped)
	}))
	defer converter.Close()
	f, token := officeSetup(t, officeInput(t, "document.docx"), "file.docx", converter.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan struct{})
	go func() { _, _ = officeStatus(f, token, ctx); close(result) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("conversion not entered")
	}
	start := time.Now()
	cancel()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("converter context not canceled")
	}
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("caller remained blocked")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("cancel exceeded bound")
	}
}

func TestTC_S03_AC03_ConverterDeadlineAndRecovery(t *testing.T) {
	var calls atomic.Int32
	stopped := make(chan struct{})
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 1 {
			<-r.Context().Done()
			close(stopped)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer converter.Close()
	f, token := officeSetup(t, officeInput(t, "document.docx"), "file.docx", converter.URL)
	start := time.Now()
	code, err := officeStatus(f, token, context.Background())
	elapsed := time.Since(start)
	if err != nil || code != 503 || elapsed < 29*time.Second || elapsed > 35*time.Second {
		t.Fatalf("converter deadline expected503 at30s got%d after%s", code, elapsed)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("timed-out converter remained active")
	}
	code, err = officeStatus(f, token, context.Background())
	if err != nil || code != 302 || calls.Load() != 2 {
		t.Fatal("failed conversion did not release owner and recover")
	}
}
