// Package adapter provides a fixture-only integration example. Production server
// does not import or register this handler.
package adapter

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/ilovintit/file-preview-server/internal/module/preview/domain/entity"
)

const CookieName = "preview_demo"
const CookiePath = "/demo/"

//go:embed index.html
var indexHTML []byte

type File struct {
	Filename string
	Summary  string
	URL      string
	SHA256   string
	Profile  string
}

type Config struct {
	APIOrigin   string
	KeyID       string
	InternalKey []byte
	AccessKey   string
	Client      *http.Client
	Clock       func() time.Time
	Files       map[string]File
}

type Handler struct{ cfg Config }

type Display struct {
	Token       string `json:"token"`
	Filename    string `json:"filename"`
	PreviewType string `json:"preview_type"`
	ExpiresAt   int64  `json:"expires_at"`
}

type row struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}

func New(cfg Config) (*Handler, error) {
	u, err := url.Parse(cfg.APIOrigin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || len(cfg.InternalKey) < 32 || len(cfg.AccessKey) < 16 || cfg.KeyID == "" {
		return nil, gerror.New("invalid demo adapter configuration")
	}
	cfg.APIOrigin = strings.TrimRight(cfg.APIOrigin, "/")
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	client := http.DefaultClient
	if cfg.Client != nil {
		client = cfg.Client
	}
	copyClient := *client
	copyClient.Timeout = 10 * time.Second
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cfg.Client = &copyClient
	cfg.InternalKey = append([]byte(nil), cfg.InternalKey...)
	files := make(map[string]File, len(cfg.Files))
	for id, file := range cfg.Files {
		source, err := url.Parse(file.URL)
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`).MatchString(id) || err != nil || source.Scheme != "https" || source.Host == "" || source.User != nil || source.Fragment != "" || !entity.HashPattern.MatchString(file.SHA256) || !entity.SupportedExtension(path.Ext(file.Filename)) || (file.Profile != "aliyun-oss" && file.Profile != "silo") {
			return nil, gerror.New("invalid demo fixture definition")
		}
		files[id] = file
	}
	cfg.Files = files
	return &Handler{cfg: cfg}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	cookie, err := r.Cookie(CookieName)
	if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(h.cfg.AccessKey)) != 1 {
		reply(w, 401, "测试访问未授权", struct{}{})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/demo/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/demo/fixtures" && r.URL.RawQuery == "" {
		rows := make([]row, 0, len(h.cfg.Files))
		for id, file := range h.cfg.Files {
			name := file.Summary
			if name == "" {
				name = file.Filename
			}
			rows = append(rows, row{id, name})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
		reply(w, 200, "成功", rows)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/demo/fixtures/"), "/preview")
	file, ok := h.cfg.Files[id]
	if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, "/demo/fixtures/") || !strings.HasSuffix(r.URL.Path, "/preview") || r.URL.RawQuery != "" || !ok {
		reply(w, 404, "示例附件不存在", struct{}{})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1025))
	var parameters map[string]json.RawMessage
	if err != nil || len(raw) > 1024 || json.Unmarshal(raw, &parameters) != nil || parameters == nil || len(parameters) != 0 {
		reply(w, 422, "仅接受附件ID", struct{}{})
		return
	}
	display, err := h.issue(r, file)
	if err != nil {
		reply(w, 503, "获取预览失败，请重试", struct{}{})
		return
	}
	reply(w, 200, "成功", display)
}

func (h *Handler) issue(request *http.Request, file File) (Display, error) {
	body, err := json.Marshal(map[string]any{"url": file.URL, "filename": file.Filename, "storage_profile": file.Profile, "content_sha256": file.SHA256, "ttl": 300, "cache_ttl": 3600})
	if err != nil {
		return Display{}, err
	}
	r, err := http.NewRequestWithContext(request.Context(), "POST", h.cfg.APIOrigin+"/internal/tokens", bytes.NewReader(body))
	if err != nil {
		return Display{}, err
	}
	var entropy [16]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		return Display{}, err
	}
	nonce := hex.EncodeToString(entropy[:])
	timestamp := strconv.FormatInt(h.cfg.Clock().Unix(), 10)
	digest := sha256.Sum256(body)
	canonical := strings.Join([]string{"POST", "/internal/tokens", h.cfg.KeyID, timestamp, nonce, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, h.cfg.InternalKey)
	_, _ = mac.Write([]byte(canonical))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Preview-Key-Id", h.cfg.KeyID)
	r.Header.Set("X-Preview-Timestamp", timestamp)
	r.Header.Set("X-Preview-Nonce", nonce)
	r.Header.Set("X-Preview-Signature", hex.EncodeToString(mac.Sum(nil)))
	response, err := h.cfg.Client.Do(r)
	if err != nil {
		return Display{}, gerror.New("demo signing request failed")
	}
	defer response.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			Token     string `json:"token"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 || response.StatusCode != 200 || json.Unmarshal(data, &result) != nil || result.Code != 0 || !entity.TokenPattern.MatchString(result.Data.Token) || result.Data.ExpiresAt <= h.cfg.Clock().Unix() {
		return Display{}, gerror.New("demo signing response invalid")
	}
	kind := "pdf"
	switch strings.ToLower(path.Ext(file.Filename)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		kind = "image"
	}
	return Display{result.Data.Token, file.Filename, kind, result.Data.ExpiresAt}, nil
}

func reply(w http.ResponseWriter, status int, message string, data any) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		http.Error(w, "Demo unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	code := 0
	if status != 200 {
		code = status*100 + 1
	}
	_ = json.NewEncoder(w).Encode(struct {
		Trace   string `json:"traceId"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}{hex.EncodeToString(random[:]), code, message, data})
}
