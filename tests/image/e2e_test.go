//go:build image

// 镜像级端到端验证：对实际构建出的应用镜像执行 #25 的 Web 关键旅程、
// 生产路由边界和重启后的授权连续性。由 tests/image/run.sh 编排，
// 依赖同一 CI 网络里的一次性 silo/Valkey/Gotenberg 服务。
package image_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type environment struct {
	baseURL     string
	client      *http.Client
	internalID  string
	internalKey []byte
	adminID     string
	adminKey    []byte
	statePath   string
}

type state struct {
	LiveToken    string `json:"live_token"`
	RevokedToken string `json:"revoked_token"`
	UsedNonce    string `json:"used_nonce"`
	UsedStamp    int64  `json:"used_timestamp"`
	UsedBody     string `json:"used_body"`
}

func setup(t *testing.T) *environment {
	t.Helper()
	dir := requireEnv(t, "PREVIEW_IMAGE_MATERIALS")
	pool := x509.NewCertPool()
	ca, err := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if err != nil || !pool.AppendCertsFromPEM(ca) {
		t.Fatal("缺少一次性 CI 根证书")
	}
	certificate, err := tls.LoadX509KeyPair(filepath.Join(dir, "harness-cert.pem"), filepath.Join(dir, "harness-key.pem"))
	if err != nil {
		t.Fatal("缺少 harness 服务证书")
	}
	upstream, err := url.Parse(requireEnv(t, "PREVIEW_IMAGE_SILO_ENDPOINT"))
	if err != nil {
		t.Fatal("无效的 silo endpoint")
	}
	// 保持 Host：silo 按签名中的 Host 校验 SigV4，这里不改写签名或 CORS。
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	mux := http.NewServeMux()
	mux.HandleFunc("/source/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := fixtures(t)[strings.TrimPrefix(r.URL.Path, "/source/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// 原样图片按扩展名要求精确媒体类型；Office 输入按内容校验。
		contentType := "application/octet-stream"
		if strings.HasSuffix(r.URL.Path, ".png") {
			contentType = "image/png"
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	mux.Handle("/", proxy)
	server := &http.Server{Addr: ":8443", Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}}
	go func() { _ = server.ListenAndServeTLS("", "") }()
	t.Cleanup(func() { _ = server.Close() })

	client := &http.Client{
		Timeout:       90 * time.Second,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	e := &environment{
		baseURL:    strings.TrimRight(requireEnv(t, "PREVIEW_IMAGE_BASE_URL"), "/"),
		client:     client,
		internalID: requireEnv(t, "PREVIEW_IMAGE_INTERNAL_KEY_ID"),
		adminID:    requireEnv(t, "PREVIEW_IMAGE_ADMIN_KEY_ID"),
		statePath:  filepath.Join(dir, "state.json"),
	}
	e.internalKey = []byte(requireEnv(t, "PREVIEW_IMAGE_INTERNAL_KEY"))
	e.adminKey = []byte(requireEnv(t, "PREVIEW_IMAGE_ADMIN_KEY"))
	e.waitReady(t)
	return e
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("镜像端到端验证缺少 %s", name)
	}
	return value
}

var fixtureCache map[string][]byte

func fixtures(t *testing.T) map[string][]byte {
	t.Helper()
	if fixtureCache != nil {
		return fixtureCache
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for x := 0; x < 64; x++ {
		for y := 0; y < 32; y++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 8), B: 0x40, A: 0xff})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		t.Fatal("PNG fixture 生成失败")
	}
	document, err := os.ReadFile(filepath.Join("..", "..", "internal", "module", "preview", "testdata", "office", "document.docx"))
	if err != nil {
		t.Fatal("缺少仓库内 Office fixture")
	}
	fixtureCache = map[string][]byte{"fixture.png": encoded.Bytes(), "document.docx": document}
	return fixtureCache
}

func (e *environment) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for {
		r, err := e.client.Get(e.baseURL + "/readyz")
		if err == nil {
			_ = r.Body.Close()
			if r.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("镜像进程未在期限内就绪")
		}
		time.Sleep(time.Second)
	}
}

func (e *environment) signed(t *testing.T, role, path string, body []byte, change func(*http.Request)) *http.Response {
	t.Helper()
	id, key := e.internalID, e.internalKey
	if role == "admin" {
		id, key = e.adminID, e.adminKey
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, e.baseURL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	sign(request, id, key, path, body, time.Now().Unix(), hex.EncodeToString(nonce))
	if change != nil {
		change(request)
	}
	response, err := e.client.Do(request)
	if err != nil {
		t.Fatalf("%s 请求失败: %v", path, err)
	}
	return response
}

func sign(request *http.Request, id string, key []byte, path string, body []byte, stamp int64, nonce string) {
	digest := sha256.Sum256(body)
	canonical := strings.Join([]string{http.MethodPost, path, id, fmt.Sprint(stamp), nonce, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(canonical))
	request.Header.Set("X-Preview-Key-Id", id)
	request.Header.Set("X-Preview-Timestamp", fmt.Sprint(stamp))
	request.Header.Set("X-Preview-Nonce", nonce)
	request.Header.Set("X-Preview-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func decode(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("非 JSON envelope 响应: %s", string(body))
	}
	if response.StatusCode != http.StatusOK || envelope.Code != 0 {
		t.Fatalf("请求未成功: HTTP %d %s", response.StatusCode, string(body))
	}
	return envelope.Data
}

func (e *environment) issue(t *testing.T, name string, profile string) string {
	t.Helper()
	digest := sha256.Sum256(fixtures(t)[name])
	body, err := json.Marshal(map[string]any{
		"url":             "https://harness:8443/source/" + name,
		"storage_profile": profile,
		"content_sha256":  hex.EncodeToString(digest[:]),
		"filename":        name,
		"ttl":             600,
		"cache_ttl":       600,
	})
	if err != nil {
		t.Fatal(err)
	}
	data := decode(t, e.signed(t, "internal", "/internal/tokens", body, nil))
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatal("签发未返回 token")
	}
	return token
}

// preview 跟随一次 302 并读取签名目标，返回目标响应的前若干字节。
func (e *environment) preview(t *testing.T, token string) ([]byte, string) {
	t.Helper()
	r, err := e.client.Get(e.baseURL + "/v/" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusFound {
		t.Fatalf("预览未返回 302：HTTP %d", r.StatusCode)
	}
	if r.Header.Get("Cache-Control") != "no-store" || r.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("302 缺少契约要求的安全响应头")
	}
	location := r.Header.Get("Location")
	if !strings.HasPrefix(location, "https://") {
		t.Fatalf("签名目标不是 HTTPS：%s", location)
	}
	target, err := e.client.Get(location)
	if err != nil {
		t.Fatalf("读取签名目标失败: %v", err)
	}
	defer target.Body.Close()
	if target.StatusCode != http.StatusOK {
		t.Fatalf("签名目标不可读：HTTP %d", target.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(target.Body, 8<<20))
	if err != nil {
		t.Fatal(err)
	}
	return body, target.Header.Get("Content-Type")
}

func (e *environment) status(t *testing.T, path string) int {
	t.Helper()
	r, err := e.client.Get(e.baseURL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	return r.StatusCode
}

// TC:S12-AC05:h5_ci_environment_handoff 与 TC:S12-AC02:deployment_configuration_security
func TestTC_S12_ImageWebJourney(t *testing.T) {
	e := setup(t)

	imageToken := e.issue(t, "fixture.png", "silo")
	body, contentType := e.preview(t, imageToken)
	if !bytes.Equal(body, fixtures(t)["fixture.png"]) || contentType != "image/png" {
		t.Fatalf("图片预览未原样返回：%d 字节 %s", len(body), contentType)
	}

	officeToken := e.issue(t, "document.docx", "silo")
	converted, convertedType := e.preview(t, officeToken)
	if !bytes.HasPrefix(converted, []byte("%PDF")) || convertedType != "application/pdf" {
		t.Fatalf("Office 转换结果不是 PDF：%s", convertedType)
	}

	if code := e.status(t, "/reader/"); code != http.StatusOK {
		t.Fatalf("镜像未提供 H5/PC 阅读页入口：HTTP %d", code)
	}
	// 生产镜像只有六条业务/探针路由，不得包含 demo fixture 或测试辅助入口。
	for _, path := range []string{"/demo/", "/demo/files", "/reader/../internal/tokens", "/fixtures/", "/debug/pprof/"} {
		if code := e.status(t, path); code != http.StatusNotFound {
			t.Fatalf("镜像暴露了非契约入口 %s：HTTP %d", path, code)
		}
	}
	r, err := e.client.Post(e.baseURL+"/internal/tokens", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未签名的内部请求未被拒绝：HTTP %d", r.StatusCode)
	}

	revoked := e.issue(t, "fixture.png", "silo")
	decode(t, e.signed(t, "admin", "/admin/tokens/"+revoked+"/revoke", []byte("{}"), nil))
	if code := e.status(t, "/v/"+revoked); code != http.StatusNotFound {
		t.Fatalf("撤销后预览未返回 404：HTTP %d", code)
	}

	stamp, nonce := time.Now().Unix(), randomNonce(t)
	replayBody := []byte(`{"limit":1,"offset":0}`)
	response := e.signed(t, "admin", "/admin/tokens/query", replayBody, func(request *http.Request) {
		sign(request, e.adminID, e.adminKey, "/admin/tokens/query", replayBody, stamp, nonce)
	})
	decode(t, response)

	persist(t, e.statePath, state{LiveToken: imageToken, RevokedToken: revoked, UsedNonce: nonce, UsedStamp: stamp, UsedBody: string(replayBody)})
}

// TC:S12-AC04:automated_recovery_and_rollback
func TestTC_S12_RecoveryAfterRestart(t *testing.T) {
	e := setup(t)
	var recorded state
	body, err := os.ReadFile(e.statePath)
	if err != nil || json.Unmarshal(body, &recorded) != nil || recorded.LiveToken == "" {
		t.Fatal("缺少上一阶段的真实状态，无法判断恢复行为")
	}
	if code := e.status(t, "/v/"+recorded.RevokedToken); code != http.StatusNotFound {
		t.Fatalf("重启后撤销的 token 复活：HTTP %d", code)
	}
	content, contentType := e.preview(t, recorded.LiveToken)
	if !bytes.Equal(content, fixtures(t)["fixture.png"]) || contentType != "image/png" {
		t.Fatal("重启后有效 token 的缓存对象不可用")
	}
	replay := []byte(recorded.UsedBody)
	response := e.signed(t, "admin", "/admin/tokens/query", replay, func(request *http.Request) {
		sign(request, e.adminID, e.adminKey, "/admin/tokens/query", replay, recorded.UsedStamp, recorded.UsedNonce)
	})
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("重启后已用 nonce 被再次接受：HTTP %d", response.StatusCode)
	}
}

func randomNonce(t *testing.T) string {
	t.Helper()
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func persist(t *testing.T, path string, value state) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}
