//go:build integration

package preview_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	pdftext "github.com/ledongthuc/pdf"
)

func TestTC_S05_AC03_InvalidInputsNeverPublish(t *testing.T) {
	var conversions atomic.Int32
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { conversions.Add(1); w.WriteHeader(500) }))
	defer converter.Close()
	for _, tc := range textFixtures(t) {
		t.Run(tc.Extension, func(t *testing.T) {
			body := []byte{0xff, 0, 0xfe}
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
			defer source.Close()
			f := setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.GotenbergURL = converter.URL })
			input := resource()
			input["url"], input["filename"], input["content_sha256"] = source.URL, "bad."+tc.Extension, sha256Hex(body)
			token := issueResource(t, f, input)
			for i := 0; i < 2; i++ {
				code, location := profileLocation(t, f, token)
				if code != 422 || location != "" {
					t.Fatalf("invalid text input expected422 got%d", code)
				}
			}
			count, err := f.db.Do(context.Background(), "HLEN", f.cfg.Namespace+":objects")
			if err != nil || count.Int() != 0 {
				t.Fatal("invalid text input created stored object")
			}
		})
	}
	if conversions.Load() != 0 {
		t.Fatal("invalid content reached converter")
	}
}

func TestTC_S05_AC03_PasswordProtectedTextNeverPublishes(t *testing.T) {
	body, err := os.ReadFile("testdata/text/negative/protected.docm")
	if err != nil {
		t.Fatal(err)
	}
	if sha256Hex(body) != "6d7e94901d30dbf0ce57207b6136783ff6048a9fc53e2658dceabd4ded1749d1" {
		t.Fatal("encrypted DOCM fixture changed")
	}
	var calls atomic.Int32
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer converter.Close()
	f, token := officeSetup(t, body, "protected.docm", converter.URL)
	for i := 0; i < 2; i++ {
		code, location := profileLocation(t, f, token)
		if code != 422 || location != "" {
			t.Fatalf("encrypted DOCM expected422 got%d", code)
		}
	}
	count, err := f.db.Do(context.Background(), "HLEN", f.cfg.Namespace+":objects")
	if err != nil || count.Int() != 0 || calls.Load() != 0 {
		t.Fatal("encrypted DOCM reached converter or published an object")
	}
}

func TestTC_S05_AC04_DocumentMacroCannotChangePreview(t *testing.T) {
	data, err := os.ReadFile("testdata/text/source.odt")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range archive.File {
		if entry.Name == "mimetype" {
			// ODF requires the first stored entry to have no ZIP extra fields.
			// CreateHeader adds an extended timestamp to a parsed FileHeader;
			// Copy preserves the original valid local header and raw bytes.
			if err := writer.Copy(entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		stream, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == "content.xml" {
			marker := []byte("<office:scripts/>")
			if bytes.Count(payload, marker) != 1 {
				t.Fatal("ODT fixture script slot changed")
			}
			event := `<office:scripts><office:event-listeners><script:event-listener script:language="ooo:script" script:event-name="dom:load" xlink:href="vnd.sun.star.script:Standard.Module1.Main?language=Basic&amp;location=document" xlink:type="simple"/></office:event-listeners></office:scripts>`
			payload = bytes.Replace(payload, marker, []byte(event), 1)
		}
		if entry.Name == "META-INF/manifest.xml" {
			// A document storage directory is not an extension package. The
			// basic-library media type belongs to the latter, not Basic/ here.
			add := `<manifest:file-entry manifest:full-path="Basic/" manifest:media-type=""/><manifest:file-entry manifest:full-path="Basic/Standard/" manifest:media-type=""/><manifest:file-entry manifest:full-path="Basic/script-lc.xml" manifest:media-type="text/xml"/><manifest:file-entry manifest:full-path="Basic/Standard/script-lb.xml" manifest:media-type="text/xml"/><manifest:file-entry manifest:full-path="Basic/Standard/Module1.xml" manifest:media-type="text/xml"/>`
			payload = bytes.Replace(payload, []byte("</manifest:manifest>"), []byte(add+"</manifest:manifest>"), 1)
		}
		header := entry.FileHeader
		part, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"Basic/script-lc.xml":          `<?xml version="1.0" encoding="UTF-8"?><library:libraries xmlns:library="http://openoffice.org/2000/library" xmlns:xlink="http://www.w3.org/1999/xlink"><library:library library:name="Standard" library:link="false"/></library:libraries>`,
		"Basic/Standard/script-lb.xml": `<?xml version="1.0" encoding="UTF-8"?><library:library xmlns:library="http://openoffice.org/2000/library" library:name="Standard" library:readonly="false" library:passwordprotected="false"><library:element library:name="Module1"/></library:library>`,
		"Basic/Standard/Module1.xml": `<?xml version="1.0" encoding="UTF-8"?><script:module xmlns:script="http://openoffice.org/2000/script" script:name="Module1" script:language="StarBasic"><![CDATA[Sub Main
ThisComponent.Text.String = "MACRO_EXECUTED"
End Sub]]></script:module>`,
	} {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.WriteString(part, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	body := buffer.Bytes()
	if len(body) < 77 || binary.LittleEndian.Uint16(body[8:10]) != zip.Store ||
		binary.LittleEndian.Uint16(body[26:28]) != 8 || binary.LittleEndian.Uint16(body[28:30]) != 0 ||
		string(body[30:38]) != "mimetype" || string(body[38:77]) != "application/vnd.oasis.opendocument.text" {
		t.Fatal("macro fixture violates ODF mimetype ZIP header requirements")
	}
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer source.Close()
	endpoint, err := url.Parse(os.Getenv("GOTENBERG_TEST_URL"))
	if err != nil || endpoint.Host == "" {
		t.Fatal("macro test requires real Gotenberg")
	}
	var calls, converterStatus atomic.Int32
	proxy := httputil.NewSingleHostReverseProxy(endpoint)
	proxy.ModifyResponse = func(r *http.Response) error {
		converterStatus.Store(int32(r.StatusCode))
		if r.StatusCode >= 400 {
			// This isolated converter receives only our synthetic macro fixture.
			// Bound its diagnostic body; never log provider responses or signed URLs.
			detail, err := io.ReadAll(io.LimitReader(r.Body, 4096))
			r.Body.Close()
			if err == nil {
				t.Logf("synthetic macro converter error: %q", string(detail))
			}
			r.Body = io.NopCloser(bytes.NewReader(detail))
			r.ContentLength = int64(len(detail))
			r.Header.Del("Content-Length")
		}
		return nil
	}
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		proxy.ServeHTTP(w, r)
	}))
	defer converter.Close()
	f := setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.GotenbergURL = converter.URL })
	input := resource()
	input["url"], input["filename"], input["content_sha256"] = source.URL, "macro.odt", sha256Hex(body)
	code, location := profileLocation(t, f, issueResource(t, f, input))
	if code != 302 {
		t.Fatalf("macro document expected302 got%d; converter calls=%d status=%d", code, calls.Load(), converterStatus.Load())
	}
	response, err := http.Get(location)
	if err != nil {
		t.Fatal("macro PDF read failed")
	}
	defer response.Body.Close()
	pdf, err := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := pdftext.NewReader(bytes.NewReader(pdf), int64(len(pdf)))
	if err != nil {
		t.Fatal("macro PDF invalid")
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		t.Fatal("macro PDF text invalid")
	}
	text, err := io.ReadAll(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), "MACRO_EXECUTED") || !strings.Contains(string(text), "File preview S05 Text 42") {
		t.Fatal("document macro changed visible content")
	}
}

func TestTC_S05_AC04_ExternalResourcesRejected(t *testing.T) {
	var conversions atomic.Int32
	converter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { conversions.Add(1); w.WriteHeader(500) }))
	defer converter.Close()
	for _, tc := range []struct{ name, extension, body string }{
		{"doctype", "fodt", `<!DOCTYPE document SYSTEM "file:///etc/passwd"><document/>`},
		{"stylesheet", "xml", `<?xml-stylesheet type="text/xsl" href="file:///etc/passwd"?><wordDocument xmlns="http://schemas.microsoft.com/office/word/2003/wordml"/>`},
		{"local-image", "fodt", `<office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:xlink="http://www.w3.org/1999/xlink"><draw:image xlink:href="file:///etc/passwd"/></office:document>`},
		{"remote-image", "fodt", `<office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:xlink="http://www.w3.org/1999/xlink"><draw:image xlink:href="https://resource.invalid/image.png"/></office:document>`},
		{"rtf-include", "rtf", `{\rtf1{\field{\*\fldinst INCLUDETEXT "file:///etc/passwd"}}}`},
		{"rtf-object", "rtf", `{\rtf1{\object\objautlink{\*\objclass Package}}}`},
		{"deep-xml", "fodt", strings.Repeat("<x>", 66) + strings.Repeat("</x>", 66)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer source.Close()
			f := setupWithOSS(t, testAliyunOSS(t), source.Client(), func(c *infrastructure.Config) { c.GotenbergURL = converter.URL })
			input := resource()
			input["url"], input["filename"], input["content_sha256"] = source.URL, "unsafe."+tc.extension, sha256Hex([]byte(tc.body))
			code, _ := profileLocation(t, f, issueResource(t, f, input))
			if code != 422 {
				t.Fatalf("unsafe resource expected422 got%d", code)
			}
		})
	}
	if conversions.Load() != 0 {
		t.Fatal("unsafe content reached converter")
	}
}
