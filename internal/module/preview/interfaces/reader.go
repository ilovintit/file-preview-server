package interfaces

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

//go:embed all:reader-assets
var readerFiles embed.FS

func Reader() http.Handler {
	assets, _ := fs.Sub(readerFiles, "reader-assets")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		name := strings.TrimPrefix(r.URL.Path, "/reader/")
		if name == "" || name == "index.html" {
			data, err := fs.ReadFile(assets, "index.html")
			if err != nil {
				http.Error(w, "Web reader assets unavailable", http.StatusServiceUnavailable)
				return
			}
			var random [18]byte
			if _, err = rand.Read(random[:]); err != nil {
				http.Error(w, "Reader unavailable", http.StatusServiceUnavailable)
				return
			}
			nonce := base64.RawStdEncoding.EncodeToString(random[:])
			data = bytes.ReplaceAll(data, []byte("<script"), []byte("<script nonce=\""+nonce+"\""))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'nonce-"+nonce+"' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' https: blob: data:; connect-src 'self' https:; worker-src 'self' blob:; font-src 'self' blob: data:; object-src 'none'; base-uri 'self'; form-action 'none'")
			if r.Method != "HEAD" {
				_, _ = w.Write(data)
			}
			return
		}
		if !fs.ValidPath(name) || strings.HasSuffix(name, ".map") {
			http.NotFound(w, r)
			return
		}
		media := map[string]string{".js": "text/javascript", ".mjs": "text/javascript", ".css": "text/css", ".wasm": "application/wasm", ".json": "application/json", ".bcmap": "application/octet-stream", ".pfb": "application/octet-stream", ".ttf": "font/ttf", ".otf": "font/otf", ".woff": "font/woff", ".woff2": "font/woff2", ".png": "image/png", ".svg": "image/svg+xml", ".ico": "image/x-icon", ".txt": "text/plain"}[path.Ext(name)]
		if media == "" {
			http.NotFound(w, r)
			return
		}
		file, err := assets.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		reader, ok := file.(io.ReadSeeker)
		if !ok {
			http.Error(w, "Reader asset unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", media)
		if strings.HasPrefix(name, "_nuxt/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, r, path.Base(name), time.Time{}, reader)
	})
}
