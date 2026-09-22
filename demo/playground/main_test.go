package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"regexp"
	"strings"
	"testing"

	"git.shw.top/shw-project/file-preview-server/demo/adapter"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
)

// adapter 对 fixture ID 的约束；playground 生成的 ID 必须直接满足它。
var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func fixtures(t *testing.T) map[string]fixture {
	t.Helper()
	items, err := load()
	if err != nil {
		t.Fatalf("加载内嵌示例失败: %v", err)
	}
	if len(items) < 8 {
		t.Fatalf("示例数量异常：%d", len(items))
	}
	return items
}

// 示例必须全部落在产品允许的 18 个后缀内，playground 不得扩大格式范围。
func TestFixturesStayInProductScope(t *testing.T) {
	for id, item := range fixtures(t) {
		if !identifierPattern.MatchString(id) {
			t.Errorf("fixture ID 不被 adapter 接受: %q", id)
		}
		if !entity.SupportedExtension(path.Ext(item.name)) {
			t.Errorf("%s 的扩展名不在产品允许集合内", item.name)
		}
		if len(item.body) == 0 {
			t.Errorf("%s 内容为空", item.name)
		}
	}
}

// 原样图片与 PDF 要求精确媒体类型；办公输入按内容校验，不声明具体类型。
func TestSourceMediaTypes(t *testing.T) {
	for name, want := range map[string]string{
		"sample.png":    "image/png",
		"sample.pdf":    "application/pdf",
		"document.docx": "application/octet-stream",
		"newfile.wps":   "application/octet-stream",
		"two-page.tiff": "application/octet-stream",
	} {
		if got := mediaType(name); got != want {
			t.Errorf("%s 的媒体类型为 %s，期望 %s", name, got, want)
		}
	}
}

func TestGeneratedPDFIsWellFormed(t *testing.T) {
	body := samplePDF()
	if !bytes.HasPrefix(body, []byte("%PDF-")) || !bytes.Contains(body, []byte("%%EOF")) || !bytes.Contains(body, []byte("xref")) {
		t.Fatal("生成的 PDF 缺少头、交叉引用表或结尾标记")
	}
}

// 把 load 的结果按 main 的方式装配成 adapter 配置，证明签发适配层能接受它：
// URL 为 HTTPS、hash 为 64 位小写十六进制、profile 与扩展名合法。
func TestAdapterAcceptsGeneratedFixtureSet(t *testing.T) {
	items := fixtures(t)
	files := make(map[string]adapter.File, len(items))
	for id, item := range items {
		digest := sha256.Sum256(item.body)
		files[id] = adapter.File{
			Filename: item.name,
			Summary:  item.summary,
			URL:      "https://file-preview-playground:8443" + sourcePrefix + item.name,
			SHA256:   hex.EncodeToString(digest[:]),
			Profile:  "aliyun-oss",
		}
	}
	if _, err := adapter.New(adapter.Config{
		APIOrigin:   "https://file-preview-server:9501",
		KeyID:       "dev-internal",
		InternalKey: bytes.Repeat([]byte{0x2b}, 32),
		AccessKey:   strings.Repeat("k", 16),
		Files:       files,
	}); err != nil {
		t.Fatalf("adapter 拒绝了 playground 生成的示例集合: %v", err)
	}
}
