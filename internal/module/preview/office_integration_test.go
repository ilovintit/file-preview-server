//go:build integration

package preview_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/infrastructure"
	pdftext "github.com/ledongthuc/pdf"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type officeFixture struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Pages  int    `json:"pages"`
	Text   string `json:"text"`
}

func TestTC_S03_AC01_CoreOfficeConversion(t *testing.T) {
	testCoreOfficeConversion(t, "aliyun-oss")
}

func TestTC_S04_AC01_SiloCoreOfficeConversion(t *testing.T) {
	testCoreOfficeConversion(t, "silo")
}

func testCoreOfficeConversion(t *testing.T, profile string) {
	endpoint := os.Getenv("GOTENBERG_TEST_URL")
	if endpoint == "" {
		t.Fatal("S03 requires actual GOTENBERG_TEST_URL")
	}
	probe := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for {
		r, err := probe.Get(endpoint + "/health")
		if err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("Gotenberg did not become healthy")
		}
		time.Sleep(250 * time.Millisecond)
	}
	manifest, err := os.ReadFile("testdata/office/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []officeFixture
	if err = json.Unmarshal(manifest, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatal("six core Office fixtures required")
	}
	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata/office", tc.File))
			if err != nil {
				t.Fatal(err)
			}
			if sha256Hex(body) != tc.SHA256 {
				t.Fatal("fixture hash mismatch")
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
			input["storage_profile"] = profile
			input["url"], input["filename"], input["content_sha256"] = source.URL, tc.File, tc.SHA256
			token := issueResource(t, f, input)
			client := f.server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			r, err := client.Get(f.server.URL + "/v/" + token)
			if err != nil {
				t.Fatal("preview request failed")
			}
			r.Body.Close()
			if r.StatusCode != http.StatusFound {
				t.Fatalf("Office conversion expected302 got%d", r.StatusCode)
			}
			object, err := objectClient.Get(r.Header.Get("Location"))
			if err != nil {
				t.Fatal("converted object request failed")
			}
			defer object.Body.Close()
			data, err := io.ReadAll(io.LimitReader(object.Body, 32<<20+1))
			if err != nil || object.StatusCode != 200 || object.Header.Get("Content-Type") != "application/pdf" || bytes.Equal(data, body) {
				t.Fatalf("expected converted PDF in OSS: read_error=%t status=%d content_type=%q disposition=%q bytes=%d", err != nil, object.StatusCode, object.Header.Get("Content-Type"), object.Header.Get("Content-Disposition"), len(data))
			}
			config := model.NewDefaultConfiguration()
			config.Offline, config.Optimize, config.ValidateLinks = true, false, false
			if err := pdfapi.Validate(bytes.NewReader(data), config); err != nil {
				t.Fatal("invalid converted PDF")
			}
			pages, err := pdfapi.PageCount(bytes.NewReader(data), config)
			if err != nil || pages != tc.Pages {
				t.Fatalf("expected %d pages got %d", tc.Pages, pages)
			}
			pdf, err := pdftext.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal("cannot parse converted PDF text")
			}
			plain, err := pdf.GetPlainText()
			if err != nil {
				t.Fatal("cannot extract converted PDF text")
			}
			text, err := io.ReadAll(plain)
			if err != nil {
				t.Fatal(err)
			}
			// PDF text extraction does not synthesize spaces between spreadsheet
			// cells. Require all expected characters in order, ignoring only whitespace.
			if !strings.Contains(strings.Join(strings.Fields(string(text)), ""), strings.Join(strings.Fields(tc.Text), "")) {
				t.Fatalf("PDF content mismatch: expected %q got %q", tc.Text, string(text))
			}
		})
	}
}
