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
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
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
			options = append(options, chromedp.ExecPath(matches[0]), chromedp.NoSandbox, chromedp.Flag("headless", false), chromedp.Flag("disable-extensions", false), chromedp.Flag("ignore-certificate-errors-spki-list", base64.StdEncoding.EncodeToString(spki[:])))
			allocator, stop := chromedp.NewExecAllocator(context.Background(), options...)
			defer stop()
			browser, closeBrowser := chromedp.NewContext(allocator)
			ctx, cancel := context.WithTimeout(browser, 25*time.Second)
			defer func() {
				closing, finishClosing := context.WithTimeout(browser, 5*time.Second)
				defer finishClosing()
				_ = chromedp.Cancel(closing)
				cancel()
				closeBrowser()
			}()
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
				if err = chromedp.Run(ctx, chromedp.Navigate(f.server.URL+path)); err != nil {
					t.Fatal("PDF navigation failed before viewer check")
				}
				var shape string
				_ = chromedp.Run(ctx, chromedp.Evaluate(`JSON.stringify(Array.from(document.querySelectorAll('*')).map(e=>({tag:e.tagName,type:e.getAttribute('type')})).slice(0,30))`, &shape))
				t.Logf("PDF viewer DOM: %s", shape)
				_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
					infos, err := target.GetTargets().Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Browser))
					if err != nil {
						return err
					}
					for _, info := range infos {
						parsed, _ := url.Parse(info.URL)
						if parsed != nil {
							t.Logf("PDF target type=%s origin=%s://%s", info.Type, parsed.Scheme, parsed.Host)
						}
					}
					root, err := dom.GetDocument().WithDepth(-1).WithPierce(true).Do(ctx)
					if err != nil {
						return err
					}
					var walk func(*cdp.Node)
					walk = func(n *cdp.Node) {
						if n == nil {
							return
						}
						t.Logf("PDF node=%s children=%d shadow=%d", n.NodeName, len(n.Children), len(n.ShadowRoots))
						for _, child := range n.Children {
							walk(child)
						}
						for _, shadow := range n.ShadowRoots {
							walk(shadow)
						}
						walk(n.ContentDocument)
					}
					walk(root)
					return nil
				}))
				var debug []byte
				if chromedp.Run(ctx, chromedp.CaptureScreenshot(&debug)) == nil {
					_ = os.MkdirAll("../../../.cache/browser-artifacts", 0755)
					_ = os.WriteFile("../../../.cache/browser-artifacts/pdf-before-check.png", debug, 0644)
				}
				if err = chromedp.Run(ctx, chromedp.ActionFunc(waitPDFViewer)); err != nil {
					t.Fatal("PDF viewer element unavailable")
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

// Chromium's PDF MediaDocument puts the viewer inside user-agent shadow DOM.
func waitPDFViewer(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		root, err := dom.GetDocument().WithDepth(-1).WithPierce(true).Do(ctx)
		if err != nil {
			return err
		}
		if hasPDFPlugin(root) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func hasPDFPlugin(n *cdp.Node) bool {
	if n == nil {
		return false
	}
	if n.NodeName == "EMBED" || n.NodeName == "OBJECT" {
		for i := 0; i+1 < len(n.Attributes); i += 2 {
			if n.Attributes[i] == "type" && (n.Attributes[i+1] == "application/pdf" || n.Attributes[i+1] == "application/x-google-chrome-pdf") {
				return true
			}
		}
	}
	for _, child := range n.Children {
		if hasPDFPlugin(child) {
			return true
		}
	}
	for _, shadow := range n.ShadowRoots {
		if hasPDFPlugin(shadow) {
			return true
		}
	}
	return hasPDFPlugin(n.ContentDocument)
}
