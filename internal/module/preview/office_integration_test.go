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
	"testing"

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
			f := setupS02(t, source.Client())
			input := resource()
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
			object, err := http.Get(r.Header.Get("Location"))
			if err != nil {
				t.Fatal("converted object request failed")
			}
			defer object.Body.Close()
			data, err := io.ReadAll(io.LimitReader(object.Body, 32<<20+1))
			if err != nil || object.StatusCode != 200 || object.Header.Get("Content-Type") != "application/pdf" || bytes.Equal(data, body) {
				t.Fatal("expected converted PDF in OSS")
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
		})
	}
}
