//go:build integration

package preview_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	pdftext "github.com/ledongthuc/pdf"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func allowedPDF(t *testing.T, body []byte, filename, profile string, expectedPages int) []byte {
	t.Helper()
	endpoint := os.Getenv("GOTENBERG_TEST_URL")
	if endpoint == "" {
		t.Fatal("actual converter required")
	}
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	}))
	t.Cleanup(source.Close)
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
	input["storage_profile"], input["url"], input["filename"], input["content_sha256"] = profile, source.URL, filename, sha256Hex(body)
	code, location := profileLocation(t, f, issueResource(t, f, input))
	if code != 302 {
		t.Fatalf("allowed file %s expected302 got%d", filename, code)
	}
	r, err := objectClient.Get(location)
	if err != nil {
		t.Fatal("profile PDF read failed")
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 32<<20+1))
	if err != nil || r.StatusCode != 200 || r.Header.Get("Content-Type") != "application/pdf" {
		t.Fatal("invalid profile response")
	}
	conf := model.NewDefaultConfiguration()
	conf.Offline, conf.Optimize, conf.ValidateLinks = true, false, false
	if err := pdfapi.Validate(bytes.NewReader(data), conf); err != nil {
		t.Fatal(err)
	}
	pages, err := pdfapi.PageCount(bytes.NewReader(data), conf)
	if err != nil || pages != expectedPages {
		saveFormatArtifacts(t, filename, profile, body, data)
		t.Fatalf("expected %d fixture pages, got %d: %v", expectedPages, pages, err)
	}
	return data
}

func TestTC_R1_WPSProfileConversion(t *testing.T) {
	manifest, err := os.ReadFile("testdata/wps/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		File, SHA256 string
		Blank        bool
		Pages        int
		Text         []string
	}
	if err := json.Unmarshal(manifest, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		for _, profile := range []string{"aliyun-oss", "silo"} {
			t.Run(tc.File+"/"+profile, func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join("testdata/wps", tc.File))
				if err != nil || sha256Hex(body) != tc.SHA256 {
					t.Fatal("native WPS fixture changed")
				}
				data := allowedPDF(t, body, tc.File, profile, tc.Pages)
				reader, err := pdftext.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Fatal(err)
				}
				plain, err := reader.GetPlainText()
				if err != nil {
					t.Fatal(err)
				}
				text, err := io.ReadAll(plain)
				if err != nil {
					t.Fatal(err)
				}
				for _, expected := range tc.Text {
					if !strings.Contains(strings.Join(strings.Fields(string(text)), ""), expected) {
						t.Fatalf("WPS source value %q missing from PDF", expected)
					}
				}
				if !tc.Blank && strings.TrimSpace(string(text)) == "" {
					t.Fatal("nonempty WPS chart lost all labels")
				}
				for _, binary := range []string{"Root Entry", "WordDocument", "ETExtData", "PowerPoint Document"} {
					if strings.Contains(string(text), binary) {
						t.Fatal("native container rendered as binary text")
					}
				}
			})
		}
	}
}

func TestTC_R1_BMPAndTIFFProfileContent(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			im.Set(x, y, color.RGBA{240, 32, 64, 255})
		}
	}
	for _, ext := range []string{"bmp", "tif", "tiff"} {
		var buffer bytes.Buffer
		var err error
		if ext == "bmp" {
			err = bmp.Encode(&buffer, im)
		} else {
			err = tiff.Encode(&buffer, im, &tiff.Options{Compression: tiff.Deflate})
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range []string{"aliyun-oss", "silo"} {
			t.Run(ext+"/"+profile, func(t *testing.T) {
				data := allowedPDF(t, buffer.Bytes(), "pattern."+ext, profile, 1)
				found := pageHasColor(t, data, 1, [3]int{240, 32, 64})
				if !found {
					saveFormatArtifacts(t, "pattern."+ext, profile, buffer.Bytes(), data)
					t.Fatal("converted PDF lost the red fixture image")
				}
			})
		}
	}
}

func TestTC_R1_RejectWorksMasqueradingAsWPS(t *testing.T) {
	body, err := os.ReadFile("testdata/rejected/microsoft-works.wps")
	if err != nil {
		t.Fatal(err)
	}
	f, token := officeSetup(t, body, "legacy.wps", os.Getenv("GOTENBERG_TEST_URL"))
	for i := 0; i < 2; i++ {
		status, err := officeStatus(f, token, context.Background())
		if err != nil || status != 422 {
			t.Fatalf("Microsoft Works must be rejected, got %d", status)
		}
	}
}

func TestTC_R1_SignedAPIRejectsExcludedFormats(t *testing.T) {
	f := setup(t)
	for _, extension := range []string{"avif", "txt", "rtf", "csv", "docm", "xlsm", "pptm", "html", "svg", "pages", "xlw", "pxl"} {
		t.Run(extension, func(t *testing.T) {
			input := resource()
			input["filename"] = "excluded." + extension
			r, result, _ := f.call("/internal/tokens", "internal", input)
			if r.StatusCode != 422 || result.Code == 0 {
				t.Fatalf("excluded format was issued: HTTP %d code %d", r.StatusCode, result.Code)
			}
		})
	}
}

func saveFormatArtifacts(t *testing.T, filename, profile string, input, output []byte) {
	t.Helper()
	folder := "../../../.cache/format-artifacts"
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, profile+"-"+filename), input, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, profile+"-"+filename+".pdf"), output, 0644); err != nil {
		t.Fatal(err)
	}
}

func pageHasColor(t *testing.T, data []byte, page int, want [3]int) bool {
	t.Helper()
	conf := model.NewDefaultConfiguration()
	conf.Offline = true
	pages, err := pdfapi.ExtractImagesRaw(bytes.NewReader(data), []string{strconv.Itoa(page)}, conf)
	if err != nil {
		t.Fatal(err)
	}
	for _, objects := range pages {
		for _, object := range objects {
			if object.Reader == nil {
				continue
			}
			decoded, _, err := image.Decode(object.Reader)
			if err != nil {
				t.Logf("PDF image decode error: %v", err)
				continue
			}
			bounds := decoded.Bounds()
			r, g, b, a := decoded.At(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/2).RGBA()
			values := [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
			t.Logf("PDF page %d center RGBA=%d,%d,%d,%d", page, values[0], values[1], values[2], a>>8)
			matches := true
			for i, v := range values {
				if v < want[i]-30 || v > want[i]+30 {
					matches = false
				}
			}
			if matches {
				return true
			}
		}
	}
	return false
}

func TestTC_R1_MultipageTIFFPreservesOrder(t *testing.T) {
	body, err := os.ReadFile("testdata/images/two-page.tiff")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"aliyun-oss", "silo"} {
		t.Run(profile, func(t *testing.T) {
			data := allowedPDF(t, body, "two-page.tiff", profile, 2)
			if !pageHasColor(t, data, 1, [3]int{240, 32, 64}) || !pageHasColor(t, data, 2, [3]int{32, 64, 240}) {
				saveFormatArtifacts(t, "two-page.tiff", profile, body, data)
				t.Fatal("TIFF lost a frame or changed page order")
			}
		})
	}
}
