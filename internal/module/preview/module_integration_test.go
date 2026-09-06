//go:build integration

package preview_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	preview "git.shw.top/shw-project/file-preview-server/internal/module/preview"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
)

var internalSecret = []byte("fixture-internal-key-32-bytes-long!")
var adminSecret = []byte("fixture-admin-key-32-bytes-long!!!!")

type fixture struct {
	t      *testing.T
	now    atomic.Int64
	nonce  atomic.Int64
	cfg    infrastructure.Config
	db     *gredis.Redis
	module *preview.Module
	server *httptest.Server
}

type response struct {
	Trace   string          `json:"traceId"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setup(t *testing.T) *fixture {
	return setupWithOSS(t, infrastructure.AliyunOSSConfig{}, nil)
}

func setupS02(t *testing.T, sourceClient *http.Client) *fixture {
	return setupWithOSS(t, testAliyunOSS(t), sourceClient)
}

func setupWithOSS(t *testing.T, ossConfig infrastructure.AliyunOSSConfig, sourceClient *http.Client) *fixture {
	t.Helper()
	address := os.Getenv("VALKEY_TEST_ADDR")
	if address == "" {
		t.Fatal("integration requires a dedicated VALKEY_TEST_ADDR; never silently skip")
	}
	f := &fixture{t: t}
	f.now.Store(time.Now().Unix())
	f.cfg = infrastructure.Config{ValkeyAddress: address, Namespace: fmt.Sprintf("test-preview-%d", time.Now().UnixNano()), MaxCacheTTL: 86400, Profiles: []string{"aliyun-oss", "silo"}, Keys: []infrastructure.CallerKey{{ID: "internal", Role: "internal", Secret: internalSecret}, {ID: "internal-next", Role: "internal", Secret: internalSecret}, {ID: "admin", Role: "admin", Secret: adminSecret}}, AliyunOSS: ossConfig, SourceHTTPClient: sourceClient}
	var err error
	f.db, err = gredis.New(&gredis.Config{Address: address, Db: 0, Protocol: 2})
	if err != nil {
		t.Fatal(err)
	}
	f.module, err = preview.New(f.cfg, func() time.Time { return time.Unix(f.now.Load(), 0) })
	if err != nil {
		t.Fatal(err)
	}
	f.server = httptest.NewTLSServer(f.module.Handler())
	// Cold or lost shared authorization state must outlive every previously valid signature.
	f.now.Add(601)
	t.Cleanup(func() {
		f.server.Close()
		_ = f.module.Close(context.Background())
		keys, e := f.db.Do(context.Background(), "KEYS", f.cfg.Namespace+":*")
		if e == nil {
			for _, key := range keys.Strings() {
				_, _ = f.db.Do(context.Background(), "DEL", key)
			}
		}
		_ = f.db.Close(context.Background())
	})
	return f
}

func testAliyunOSS(t *testing.T) infrastructure.AliyunOSSConfig {
	t.Helper()
	keys := []string{
		"PREVIEW_CI_ALIYUN_OSS_ENDPOINT",
		"PREVIEW_CI_ALIYUN_OSS_REGION",
		"PREVIEW_CI_ALIYUN_OSS_BUCKET",
		"PREVIEW_CI_ALIYUN_OSS_PREFIX_BASE",
		"PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_ID",
		"PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_SECRET",
		"PREVIEW_CI_OSS_SIGNED_URL_MAX_TTL_SECONDS",
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = os.Getenv(key)
		if values[key] == "" {
			t.Fatalf("S02 integration requires %s", key)
		}
	}
	maxTTL, err := strconv.ParseInt(values["PREVIEW_CI_OSS_SIGNED_URL_MAX_TTL_SECONDS"], 10, 64)
	if err != nil || maxTTL < 1 {
		t.Fatal("invalid PREVIEW_CI_OSS_SIGNED_URL_MAX_TTL_SECONDS")
	}
	return infrastructure.AliyunOSSConfig{
		Endpoint:        values["PREVIEW_CI_ALIYUN_OSS_ENDPOINT"],
		Region:          values["PREVIEW_CI_ALIYUN_OSS_REGION"],
		Bucket:          values["PREVIEW_CI_ALIYUN_OSS_BUCKET"],
		PrefixBase:      values["PREVIEW_CI_ALIYUN_OSS_PREFIX_BASE"],
		AccessKeyID:     values["PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_ID"],
		AccessKeySecret: values["PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_SECRET"],
		SecurityToken:   os.Getenv("PREVIEW_CI_ALIYUN_OSS_SECURITY_TOKEN"),
		SignedURLMaxTTL: maxTTL,
	}
}

func resource() map[string]any {
	return map[string]any{"url": "https://source.invalid/private.pdf?Signature=source-fixture", "storage_profile": "aliyun-oss", "content_sha256": strings.Repeat("a", 64), "filename": "sample.pdf", "ttl": 60, "cache_ttl": 120}
}

func signature(key []byte, method, path, id, timestamp, nonce string, body []byte) string {
	hash := sha256.Sum256(body)
	canonical := strings.Join([]string{method, path, id, timestamp, nonce, hex.EncodeToString(hash[:])}, "\n")
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(canonical))
	return hex.EncodeToString(h.Sum(nil))
}

func (f *fixture) request(path, id string, body []byte, change func(*http.Request)) (*http.Response, response, []byte) {
	f.t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	timestamp := fmt.Sprint(f.now.Load())
	nonce := fmt.Sprintf("%032x", f.nonce.Add(1))
	secret := internalSecret
	if strings.HasPrefix(id, "admin") {
		secret = adminSecret
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Preview-Key-Id", id)
	req.Header.Set("X-Preview-Timestamp", timestamp)
	req.Header.Set("X-Preview-Nonce", nonce)
	req.Header.Set("X-Preview-Signature", signature(secret, "POST", req.URL.Path, id, timestamp, nonce, body))
	if change != nil {
		change(req)
	}
	r, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	var result response
	_ = json.Unmarshal(raw, &result)
	return r, result, raw
}

func (f *fixture) call(path, id string, value any) (*http.Response, response, []byte) {
	b, e := json.Marshal(value)
	if e != nil {
		f.t.Fatal(e)
	}
	return f.request(path, id, b, nil)
}
func (f *fixture) issue() string {
	f.t.Helper()
	r, b, _ := f.call("/internal/tokens", "internal", resource())
	if r.StatusCode != 200 || b.Code != 0 {
		f.t.Fatalf("issue expected 200/code0, got %d/code%d", r.StatusCode, b.Code)
	}
	var data struct {
		Token string `json:"token"`
	}
	if e := json.Unmarshal(b.Data, &data); e != nil {
		f.t.Fatal(e)
	}
	return data.Token
}

func TestTC_S01_AC01_IssueTokenResponse(t *testing.T) {
	f := setup(t)
	r, b, raw := f.call("/internal/tokens", "internal", resource())
	if r.StatusCode != 200 || b.Code != 0 {
		t.Fatalf("TC:S01-AC01:issue_token_response expected successful issue, got HTTP %d/code %d", r.StatusCode, b.Code)
	}
	var data map[string]any
	if e := json.Unmarshal(b.Data, &data); e != nil {
		t.Fatal(e)
	}
	token, _ := data["token"].(string)
	decoded, e := hex.DecodeString(token)
	if e != nil || len(decoded) != 16 || token != strings.ToLower(token) {
		t.Fatal("token must be random 128-bit lowercase hex")
	}
	if data["expires_at"] != float64(f.now.Load()+60) || len(data) != 2 || b.Trace == "" {
		t.Fatal("unexpected token DTO")
	}
	if bytes.Contains(raw, []byte("source.invalid")) || bytes.Contains(raw, internalSecret) {
		t.Fatal("response leaked resource or key")
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("token response must not be cached")
	}
	for _, bad := range []string{`{"url":`, `{"url":"https://source.invalid/x.pdf","unknown":true}`, `{"ttl":60} {"ttl":60}`} {
		r, _, _ := f.request("/internal/tokens", "internal", []byte(bad), nil)
		if r.StatusCode != 422 {
			t.Errorf("invalid body expected 422 got %d", r.StatusCode)
		}
	}
	for _, path := range []string{"/preview", "/img/a", "/redact", "/health"} {
		r, e := f.server.Client().Get(f.server.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Errorf("legacy %s should be 404", path)
		}
	}
}

func TestTC_S01_AC02_SignedRequestSecurity(t *testing.T) {
	f := setup(t)
	body, _ := json.Marshal(resource())
	cases := []struct {
		name   string
		change func(*http.Request)
	}{
		{"missing", func(r *http.Request) { r.Header.Del("X-Preview-Signature") }},
		{"unknown-key", func(r *http.Request) { r.Header.Set("X-Preview-Key-Id", "unknown") }},
		{"bad-signature", func(r *http.Request) { r.Header.Set("X-Preview-Signature", strings.Repeat("0", 64)) }},
		{"body-tampered", func(r *http.Request) {
			r.Body = io.NopCloser(bytes.NewReader(append(body, ' ')))
			r.ContentLength = int64(len(body) + 1)
		}},
		{"duplicate-header", func(r *http.Request) { r.Header.Add("X-Preview-Key-Id", "admin") }},
		{"bad-nonce", func(r *http.Request) { r.Header.Set("X-Preview-Nonce", "short") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _, _ := f.request("/internal/tokens", "internal", body, tc.change)
			if r.StatusCode != 401 {
				t.Fatalf("TC:S01-AC02:signed_request_security %s expected401 got%d", tc.name, r.StatusCode)
			}
		})
	}
	for _, offset := range []int64{-301, -300, 300, 301} {
		r, _, _ := f.request("/internal/tokens", "internal", body, func(r *http.Request) {
			ts := fmt.Sprint(f.now.Load() + offset)
			r.Header.Set("X-Preview-Timestamp", ts)
			r.Header.Set("X-Preview-Signature", signature(internalSecret, "POST", r.URL.Path, "internal", ts, r.Header.Get("X-Preview-Nonce"), body))
		})
		want := 401
		if offset >= -300 && offset <= 300 {
			want = 200
		}
		if r.StatusCode != want {
			t.Errorf("offset %d want%d got%d", offset, want, r.StatusCode)
		}
	}
	r, _, _ := f.call("/internal/tokens", "admin", resource())
	if r.StatusCode != 401 {
		t.Fatal("admin cannot issue")
	}
	r, _, _ = f.call("/admin/tokens/query", "internal", map[string]any{})
	if r.StatusCode != 401 {
		t.Fatal("internal cannot query")
	}
	r, _, _ = f.call("/internal/tokens", "internal-next", resource())
	if r.StatusCode != 200 {
		t.Fatal("active next key must work")
	}
	// Future timestamp stays valid after a naive 300-second nonce TTL would expire.
	ts := fmt.Sprint(f.now.Load() + 300)
	nonce := strings.Repeat("f", 32)
	change := func(r *http.Request) {
		r.Header.Set("X-Preview-Timestamp", ts)
		r.Header.Set("X-Preview-Nonce", nonce)
		r.Header.Set("X-Preview-Signature", signature(internalSecret, "POST", r.URL.Path, "internal", ts, nonce, body))
	}
	r, _, _ = f.request("/internal/tokens", "internal", body, change)
	if r.StatusCode != 200 {
		t.Fatal("first future request rejected")
	}
	f.now.Add(301)
	r, _, _ = f.request("/internal/tokens", "internal", body, change)
	if r.StatusCode != 401 {
		t.Fatalf("future replay expected401 got%d", r.StatusCode)
	}
	// Atomic single-use across simultaneous requests.
	ts = fmt.Sprint(f.now.Load())
	nonce = strings.Repeat("e", 32)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _, _ := f.request("/internal/tokens", "internal", body, change)
			if r.StatusCode == 200 {
				accepted.Add(1)
			} else if r.StatusCode != 401 {
				t.Errorf("replay got%d", r.StatusCode)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("nonce accepted %d times", accepted.Load())
	}
}

func TestTC_S01_AC03_ExplicitTTLBounds(t *testing.T) {
	f := setup(t)
	for _, tc := range []struct {
		name       string
		ttl, cache any
		want       int
	}{{"minimum", 60, 60, 200}, {"maximum", 86400, 86400, 200}, {"short", 59, 60, 422}, {"long", 86401, 86401, 422}, {"cache-short", 60, 59, 422}, {"cache-long", 60, 86401, 422}, {"fraction", 60.5, 120, 422}, {"zero", 0, 120, 422}, {"missing", nil, 120, 422}, {"cache-missing", 60, nil, 422}} {
		t.Run(tc.name, func(t *testing.T) {
			value := resource()
			value["ttl"] = tc.ttl
			value["cache_ttl"] = tc.cache
			r, _, _ := f.call("/internal/tokens", "internal", value)
			if r.StatusCode != tc.want {
				t.Fatalf("TC:S01-AC03:explicit_ttl_bounds want%d got%d", tc.want, r.StatusCode)
			}
		})
	}
	for key, value := range map[string]any{"url": "http://source.invalid/x.pdf", "filename": "../a.pdf", "content_sha256": "ABC", "storage_profile": "unknown", "expires_at": 123, "metadata": []int{1}} {
		bad := resource()
		bad[key] = value
		r, _, _ := f.call("/internal/tokens", "internal", bad)
		if r.StatusCode != 422 {
			t.Errorf("invalid %s expected422 got%d", key, r.StatusCode)
		}
	}
}

func TestTC_S01_AC04_ActiveQueryIdempotentRevoke(t *testing.T) {
	f := setup(t)
	first := f.issue()
	second := f.issue()
	indexes, err := f.db.Do(context.Background(), "KEYS", f.cfg.Namespace+":auth:*:tokens:index")
	if err != nil || len(indexes.Strings()) != 1 {
		t.Fatal("token index was not created")
	}
	score, err := f.db.Do(context.Background(), "ZSCORE", indexes.Strings()[0], first)
	if err != nil || score.Int64() != f.now.Load() {
		t.Fatal("token index must retain the grant creation time as its ordering score")
	}
	r, b, _ := f.call("/admin/tokens/query", "admin", map[string]any{"limit": 1, "offset": 0})
	if r.StatusCode != 200 {
		t.Fatalf("TC:S01-AC04:active_query_idempotent_revoke query got%d", r.StatusCode)
	}
	var page struct {
		Items  []map[string]any `json:"items"`
		Total  int              `json:"total"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
	}
	if e := json.Unmarshal(b.Data, &page); e != nil {
		t.Fatal(e)
	}
	if page.Total != 2 || len(page.Items) != 1 || page.Limit != 1 {
		t.Fatal("bad page DTO")
	}
	if _, ok := page.Items[0]["url"]; ok {
		t.Fatal("management DTO must not expose source URL")
	}
	for _, token := range []string{first, first, strings.Repeat("0", 32)} {
		r, _, _ := f.call("/admin/tokens/"+token+"/revoke", "admin", map[string]any{})
		if r.StatusCode != 200 {
			t.Fatal("revoke not idempotent")
		}
	}
	r, b, _ = f.call("/admin/tokens/query", "admin", map[string]any{})
	_ = json.Unmarshal(b.Data, &page)
	if r.StatusCode != 200 || page.Total != 1 || page.Items[0]["token"] != second {
		t.Fatal("revoke/query mismatch")
	}
	f.now.Add(61)
	r, b, _ = f.call("/admin/tokens/query", "admin", map[string]any{})
	_ = json.Unmarshal(b.Data, &page)
	if r.StatusCode != 200 || page.Total != 0 || len(page.Items) != 0 {
		t.Fatal("expired item returned")
	}
	for _, q := range []map[string]any{{"limit": 0}, {"limit": 101}, {"offset": -1}, {"filter": "all"}} {
		r, _, _ := f.call("/admin/tokens/query", "admin", q)
		if r.StatusCode != 422 {
			t.Fatal("query bounds not enforced")
		}
	}
}

func TestTC_S01_AC05_StateLossFailClosed(t *testing.T) {
	f := setup(t)
	token := f.issue()
	ctx := context.Background()
	// Removing the shared continuity marker simulates a lost authorization database.
	_, e := f.db.Do(ctx, "DEL", f.cfg.Namespace+":security-state")
	if e != nil {
		t.Fatal(e)
	}
	r, _, raw := f.call("/internal/tokens", "internal", resource())
	if r.StatusCode != 503 {
		t.Fatalf("TC:S01-AC05:authorization_state_fail_closed expected503 after state loss got%d", r.StatusCode)
	}
	if bytes.Contains(raw, internalSecret) || bytes.Contains(raw, []byte("Signature=source-fixture")) {
		t.Fatal("sensitive response")
	}
	f.now.Add(601)
	_ = f.issue()
	res, e := f.server.Client().Get(f.server.URL + "/v/" + token)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal("old epoch token revived")
	}
}

func TestUntrustedForwardedProtoDoesNotGrantHTTPS(t *testing.T) {
	f := setup(t)
	plain := httptest.NewServer(f.module.Handler())
	defer plain.Close()
	req, _ := http.NewRequest("POST", plain.URL+"/internal/tokens", strings.NewReader(`{}`))
	req.Header.Set("X-Forwarded-Proto", "https")
	r, e := plain.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if r.StatusCode != 401 {
		t.Fatal("untrusted forwarded proto granted access")
	}
}
