// Command playground 是只在开发环境部署的演示应用：它携带仓库内示例附件，
// 以集群内 HTTPS 提供源文件，并通过 internal 角色密钥向预览服务签发 token，
// 让使用者在浏览器里走通签发→预览→阅读页的真实链路。
//
// 它是独立镜像，生产预览服务镜像不包含本程序，也不注册 /demo/ 路由。
package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ilovintit/file-preview-server/demo/adapter"
)

//go:embed all:fixtures
var fixtureFiles embed.FS

// sourcePrefix 是预览服务下载示例文件的路径；它只服务仓库内示例，
// 不接受任意路径，也不暴露给浏览器使用者之外的用途。
const sourcePrefix = "/demo/source/"

type fixture struct {
	name    string
	summary string
	body    []byte
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	address := env("PLAYGROUND_ADDR", ":8443")
	certFile, keyFile := os.Getenv("PLAYGROUND_TLS_CERT_FILE"), os.Getenv("PLAYGROUND_TLS_KEY_FILE")
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("需要 PLAYGROUND_TLS_CERT_FILE 与 PLAYGROUND_TLS_KEY_FILE：预览服务只接受 HTTPS 源")
	}
	sourceOrigin := strings.TrimRight(os.Getenv("PLAYGROUND_SOURCE_ORIGIN"), "/")
	if !strings.HasPrefix(sourceOrigin, "https://") {
		return fmt.Errorf("PLAYGROUND_SOURCE_ORIGIN 必须是 https Origin")
	}
	accessKey := os.Getenv("PLAYGROUND_ACCESS_KEY")
	if len(accessKey) < 16 {
		return fmt.Errorf("PLAYGROUND_ACCESS_KEY 至少 16 字节")
	}
	internalKey, err := base64.StdEncoding.DecodeString(os.Getenv("PREVIEW_INTERNAL_KEY_BASE64"))
	if err != nil {
		return fmt.Errorf("PREVIEW_INTERNAL_KEY_BASE64 不是合法 base64")
	}
	profile := env("PLAYGROUND_STORAGE_PROFILE", "aliyun-oss")

	items, err := load()
	if err != nil {
		return err
	}
	files := make(map[string]adapter.File, len(items))
	for id, item := range items {
		digest := sha256.Sum256(item.body)
		files[id] = adapter.File{
			Filename: item.name,
			Summary:  item.summary,
			URL:      sourceOrigin + sourcePrefix + item.name,
			SHA256:   hex.EncodeToString(digest[:]),
			Profile:  profile,
		}
	}
	demo, err := adapter.New(adapter.Config{
		APIOrigin:   os.Getenv("PREVIEW_API_ORIGIN"),
		KeyID:       os.Getenv("PREVIEW_KEY_ID"),
		InternalKey: internalKey,
		AccessKey:   accessKey,
		Files:       files,
	})
	if err != nil {
		return fmt.Errorf("playground 配置无效：%w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	// 源文件不带访问口令：它由预览服务在集群内下载，示例内容本身不敏感，
	// 且只按固定名称提供仓库内文件，不接受任意路径。
	mux.HandleFunc(sourcePrefix, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		item, ok := items[identifier(strings.TrimPrefix(r.URL.Path, sourcePrefix))]
		if !ok || r.URL.RawQuery != "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", mediaType(item.name))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(item.body)
		}
	})
	// 访问口令入口：把口令写进 cookie 后回到列表页，供演示者一次性进入。
	mux.HandleFunc("/demo/enter", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		supplied := r.URL.Query().Get("key")
		if r.Method != http.MethodGet || subtle.ConstantTimeCompare([]byte(supplied), []byte(accessKey)) != 1 {
			http.Error(w, "测试访问未授权", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: adapter.CookieName, Value: accessKey, Path: adapter.CookiePath,
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 8 * 3600,
		})
		http.Redirect(w, r, "/demo/", http.StatusFound)
	})
	mux.Handle("/demo/", demo)

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("playground TLS 证书无效：%w", err)
	}
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}},
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	fmt.Printf("playground listening on %s with %d fixtures\n", address, len(items))
	return server.ListenAndServeTLS("", "")
}

// load 读取内嵌示例文件，并补两个按产品允许范围生成的原样文件。
func load() (map[string]fixture, error) {
	entries, err := fixtureFiles.ReadDir("fixtures")
	if err != nil {
		return nil, err
	}
	items := map[string]fixture{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := fixtureFiles.ReadFile("fixtures/" + entry.Name())
		if err != nil {
			return nil, err
		}
		items[identifier(entry.Name())] = fixture{name: entry.Name(), summary: summary(entry.Name()), body: body}
	}
	sample, err := samplePNG()
	if err != nil {
		return nil, err
	}
	items["sample-png"] = fixture{name: "sample.png", summary: "示例图片（PNG，原样预览）", body: sample}
	items["sample-pdf"] = fixture{name: "sample.pdf", summary: "示例文档（PDF，原样预览）", body: samplePDF()}
	return items, nil
}

// identifier 把文件名转成 adapter 接受的 fixture ID。
func identifier(name string) string {
	return strings.NewReplacer(".", "-", "_", "-", " ", "-").Replace(strings.ToLower(name))
}

func summary(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".docx":
		return "Word 文档（转换为 PDF）"
	case ".xlsx":
		return "Excel 表格（转换为 PDF）"
	case ".pptx":
		return "PowerPoint 演示（转换为 PDF）"
	case ".wps":
		return "金山 WPS 文字（转换为 PDF）"
	case ".et":
		return "金山 WPS 表格（转换为 PDF）"
	case ".dps":
		return "金山 WPS 演示（转换为 PDF）"
	case ".tiff", ".tif":
		return "多页 TIFF 图片（转换为 PDF）"
	}
	return name
}

func mediaType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".pdf":
		return "application/pdf"
	}
	// 办公与 TIFF 输入按内容校验，源站媒体类型不参与判定。
	return "application/octet-stream"
}

func samplePNG() ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for x := 0; x < 320; x++ {
		for y := 0; y < 180; y++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 255 / 320), G: uint8(y * 255 / 180), B: 0x60, A: 0xff})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// samplePDF 生成一页带固定文字、交叉引用表完整的最小 PDF。
func samplePDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	stream := "BT /F1 18 Tf 40 100 Td (File preview playground) Tj ET\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
