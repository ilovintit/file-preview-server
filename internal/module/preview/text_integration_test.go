//go:build integration

package preview_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	pdftext "github.com/ledongthuc/pdf"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestTC_S05_AC02_FormatProfileConversion(t *testing.T) {
	endpoint := os.Getenv("GOTENBERG_TEST_URL")
	if endpoint == "" {
		t.Fatal("S05 requires real Gotenberg")
	}
	for _, tc := range textFixtures(t) {
		for _, profile := range []string{"aliyun-oss", "silo"} {
			t.Run(tc.Extension+"/"+profile, func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join("testdata/text", tc.File))
				if err != nil {
					t.Fatal(err)
				}
				if sha256Hex(body) != tc.SHA256 {
					t.Fatal("fixture hash changed")
				}
				source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/octet-stream")
					_, _ = w.Write(body)
				}))
				defer source.Close()
				var f *fixture
				objectClient := http.DefaultClient
				if profile == "silo" {
					var silo *siloFixture
					f, silo = setupSilo(t, source.Client())
					objectClient = silo.proxy.Client()
				} else {
					f = setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.GotenbergURL = endpoint })
				}
				input := resource()
				input["storage_profile"], input["url"], input["filename"], input["content_sha256"] = profile, source.URL, tc.File, tc.SHA256
				code, location := profileLocation(t, f, issueResource(t, f, input))
				if code != 302 {
					t.Fatalf("text conversion expected302 got%d", code)
				}
				response, err := objectClient.Get(location)
				if err != nil {
					t.Fatal("converted text object read failed")
				}
				defer response.Body.Close()
				data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
				if err != nil || response.StatusCode != 200 || response.Header.Get("Content-Type") != "application/pdf" {
					t.Fatal("expected actual profile PDF")
				}
				config := model.NewDefaultConfiguration()
				config.Offline, config.Optimize, config.ValidateLinks = true, false, false
				if err := pdfapi.Validate(bytes.NewReader(data), config); err != nil {
					t.Fatal("invalid PDF output")
				}
				pages, err := pdfapi.PageCount(bytes.NewReader(data), config)
				if err != nil || pages != tc.Pages {
					t.Fatalf("expected%d pages got%d", tc.Pages, pages)
				}
				reader, err := pdftext.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Fatal("cannot parse PDF text")
				}
				plain, err := reader.GetPlainText()
				if err != nil {
					t.Fatal("cannot extract PDF text")
				}
				text, err := io.ReadAll(plain)
				if err != nil {
					t.Fatal(err)
				}
				normalize := func(s string) string { return strings.Join(strings.Fields(s), "") }
				actual := normalize(string(text))
				for _, expected := range tc.Text {
					if !strings.Contains(actual, normalize(expected)) {
						t.Fatalf("missing expected content %q in %q", expected, string(text))
					}
				}
				if tc.ExactText != "" && actual != normalize(tc.ExactText) {
					t.Fatalf("unexpected visible text: %q", string(text))
				}
				for _, garbage := range []string{"Root Entry", "SW5HDR", "<?xml", "<abiword", "office:document", "####", "\ufffd"} {
					if strings.Contains(string(text), garbage) {
						t.Fatalf("binary/XML text fallback leaked %q", garbage)
					}
				}
			})
		}
	}
}
