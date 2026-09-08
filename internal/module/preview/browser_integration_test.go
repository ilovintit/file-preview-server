//go:build integration && browser

package preview_test

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestTC_S02_BrowserNavigation(t *testing.T) {
	matches, err := filepath.Glob("/ms-playwright/chromium-*/chrome-linux*/chrome")
	if err != nil || len(matches) != 1 {
		t.Fatal("CI requires one pinned Chromium installation")
	}
	for ext, value := range rawFixtures(t) {
		t.Run(ext, func(t *testing.T) {
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", value.mime)
				_, _ = w.Write(value.body)
			}))
			defer source.Close()
			f := setupS02(t, source.Client())
			input := resource()
			input["url"], input["content_sha256"], input["filename"] = source.URL, sha256Hex(value.body), "fixture."+ext
			token := issueResource(t, f, input)
			cert, err := x509.ParseCertificate(f.server.TLS.Certificates[0].Certificate[0])
			if err != nil {
				t.Fatal("invalid fixture certificate")
			}
			spki := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
			options := append([]chromedp.ExecAllocatorOption(nil), chromedp.DefaultExecAllocatorOptions[:]...)
			options = append(options, chromedp.ExecPath(matches[0]), chromedp.NoSandbox, chromedp.UserDataDir(t.TempDir()), chromedp.Flag("disable-extensions", false), chromedp.Flag("ignore-certificate-errors-spki-list", base64.StdEncoding.EncodeToString(spki[:])))
			allocator, stop := chromedp.NewExecAllocator(context.Background(), options...)
			defer stop()
			browser, closeBrowser := chromedp.NewContext(allocator)
			defer func() {
				closing, cancel := context.WithTimeout(browser, 5*time.Second)
				defer cancel()
				_ = chromedp.Cancel(closing)
				closeBrowser()
			}()
			ctx, cancel := context.WithTimeout(browser, 25*time.Second)
			defer cancel()
			// Pin only the generated preview-server certificate; OSS TLS is verified normally.
			if err = chromedp.Run(ctx, chromedp.Navigate(f.server.URL+"/livez")); err != nil {
				t.Fatal("browser fixture origin navigation failed")
			}
			path := "/v/" + token
			var passed bool
			if ext == "pdf" {
				// Actual cross-origin fetch after token302, with CORS-enforced Range access.
				script := fmt.Sprintf(`(async()=>{const r=await fetch(%q,{headers:{Range:"bytes=0-3"},credentials:"omit"});return r.status===206 && r.headers.get("Content-Range")!==null && await r.text()==="%%PDF";})()`, path)
				err = chromedp.Run(ctx, chromedp.Evaluate(script, &passed, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
			} else {
				script := fmt.Sprintf(`(async()=>{const im=new Image();im.src=%q;document.body.replaceChildren(im);await im.decode();return im.naturalWidth===16&&im.naturalHeight===16;})()`, path)
				err = chromedp.Run(ctx, chromedp.Evaluate(script, &passed, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
			}
			if err != nil || !passed {
				t.Fatalf("browser %s preview failed", ext)
			}
			if ext == "pdf" {
				if err = chromedp.Run(ctx, chromedp.Navigate(f.server.URL+path), chromedp.WaitReady(`embed[type="application/pdf"]`, chromedp.ByQuery)); err != nil {
					t.Fatal("browser PDF viewer navigation failed")
				}
			}
			var screenshot []byte
			if err = chromedp.Run(ctx, chromedp.CaptureScreenshot(&screenshot)); err != nil {
				t.Fatal("browser screenshot failed")
			}
			if err = os.MkdirAll("../../../.cache/browser-artifacts", 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile("../../../.cache/browser-artifacts/"+ext+".png", screenshot, 0644); err != nil {
				t.Fatal(err)
			}
		})
	}
}
