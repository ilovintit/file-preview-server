//go:build integration

package preview_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 固定 16x16 RGB(240,32,64) 图案：PNG/JPEG/GIF 使用 Go 编码器；
// WebP 来自 cwebp，AVIF 来自 gen2brain/avif v0.4.4，源码保存编码字节。
func rawFixtures(t *testing.T) map[string]struct {
	body []byte
	mime string
} {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			im.Set(x, y, color.RGBA{240, 32, 64, 255})
		}
	}
	result := map[string]struct {
		body []byte
		mime string
	}{"pdf": {rawPDF, "application/pdf"}}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	result["png"] = struct {
		body []byte
		mime string
	}{append([]byte(nil), b.Bytes()...), "image/png"}
	b.Reset()
	if err := jpeg.Encode(&b, im, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	result["jpg"] = struct {
		body []byte
		mime string
	}{append([]byte(nil), b.Bytes()...), "image/jpeg"}
	result["jpeg"] = result["jpg"]
	b.Reset()
	if err := gif.Encode(&b, im, nil); err != nil {
		t.Fatal(err)
	}
	result["gif"] = struct {
		body []byte
		mime string
	}{append([]byte(nil), b.Bytes()...), "image/gif"}
	for ext, encoded := range map[string]string{
		"webp": "UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoQABAAAgA0JaACdLoAA5gA/u0HL18AU9Uf/eM//uM//uM/+RAAAA==",
		"avif": "AAAAIGZ0eXBhdmlmAAAAAGF2aWZtaWYxbWlhZk1BMUIAAADybWV0YQAAAAAAAAAoaGRscgAAAAAAAAAAcGljdAAAAAAAAAAAAAAAAGxpYmF2aWYAAAAADnBpdG0AAAAAAAEAAAAeaWxvYwAAAABEAAABAAEAAAABAAABGgAAAB0AAAAoaWluZgAAAAAAAQAAABppbmZlAgAAAAABAABhdjAxQ29sb3IAAAAAamlwcnAAAABLaXBjbwAAABRpc3BlAAAAAAAAABAAAAAQAAAAEHBpeGkAAAAAAwgICAAAAAxhdjFDgQAMAAAAABNjb2xybmNseAACAAIAAoAAAAAXaXBtYQAAAAAAAAABAAEEAQKDBAAAACVtZGF0EgAKBhgM/9gQgDIRFkAGGGGEADlSB5LbI3y3F+k=",
	} {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		result[ext] = struct {
			body []byte
			mime string
		}{data, "image/" + ext}
	}
	return result
}

func TestTC_S02_AC01_RawFormatMatrix(t *testing.T) {
	for ext, value := range rawFixtures(t) {
		t.Run(ext, func(t *testing.T) {
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", value.mime)
				_, _ = w.Write(value.body)
			}))
			defer source.Close()
			f := setupS02(t, source.Client())
			input := resource()
			input["url"] = source.URL
			input["filename"] = "fixture." + ext
			input["content_sha256"] = sha256Hex(value.body)
			token := issueResource(t, f, input)
			client := f.server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			r, err := client.Get(f.server.URL + "/v/" + token)
			if err != nil {
				t.Fatal("preview request failed")
			}
			r.Body.Close()
			if r.StatusCode != 302 {
				t.Fatalf("format %s expected302 got%d", ext, r.StatusCode)
			}
			object, err := http.Get(r.Header.Get("Location"))
			if err != nil {
				t.Fatal("object request failed")
			}
			defer object.Body.Close()
			data, err := io.ReadAll(io.LimitReader(object.Body, 1<<20))
			if err != nil || object.StatusCode != 200 || !bytes.Equal(data, value.body) {
				t.Fatal("raw bytes changed")
			}
			if object.Header.Get("Content-Type") != value.mime {
				t.Fatal("incorrect MIME")
			}
		})
	}
}

func TestTC_S02_AC01_TruncatedImages(t *testing.T) {
	fixtures := rawFixtures(t)
	for _, ext := range []string{"webp", "avif"} {
		t.Run(ext, func(t *testing.T) {
			value := fixtures[ext]
			value.body = value.body[:16]
			source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", value.mime)
				_, _ = w.Write(value.body)
			}))
			defer source.Close()
			f := setupS02(t, source.Client())
			input := resource()
			input["url"] = source.URL
			input["filename"] = "truncated." + ext
			input["content_sha256"] = sha256Hex(value.body)
			token := issueResource(t, f, input)
			client := f.server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			r, err := client.Get(f.server.URL + "/v/" + token)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Body.Close()
			if r.StatusCode != 422 {
				t.Fatalf("truncated %s expected422 got%d", ext, r.StatusCode)
			}
		})
	}
}
