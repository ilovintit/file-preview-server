//go:build integration

package preview_test

import (
	"context"
	"encoding/json"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTC_S02_AC05_LeaseLossFencesPublication(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(rawPDF)
	token := issueResource(t, f, input)
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	result := make(chan int, 1)
	go func() {
		r, err := client.Get(f.server.URL + "/v/" + token)
		if err != nil {
			result <- 0
			return
		}
		r.Body.Close()
		result <- r.StatusCode
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("download not started")
	}
	keys, err := f.db.Do(context.Background(), "KEYS", f.cfg.Namespace+":auth:*:lease:*")
	if err != nil || len(keys.Strings()) != 1 {
		close(release)
		t.Fatal("lease missing")
	}
	_, err = f.db.Do(context.Background(), "SET", keys.Strings()[0], strings.Repeat("e", 32), "PX", 10000)
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-result:
		if code < 500 {
			t.Fatalf("lost owner must not publish: got%d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("request exceeded deadline")
	}
	ready, err := f.db.Do(context.Background(), "KEYS", f.cfg.Namespace+":auth:*:cache:*")
	if err != nil || len(ready.Strings()) != 0 {
		t.Fatal("lost owner published cache")
	}
}

func TestTC_S02_AC04_DeadlineAndCleanup(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(rawPDF)
	}))
	defer source.Close()
	f := setupS02(t, source.Client())
	input := resource()
	input["url"], input["content_sha256"] = source.URL, sha256Hex(rawPDF)
	input["cache_ttl"] = 180
	first := issueResource(t, f, input)
	input["cache_ttl"] = 120
	_ = issueResource(t, f, input)
	ctx := context.Background()
	keys, err := f.db.Do(ctx, "KEYS", f.cfg.Namespace+":auth:*:deadline:*")
	if err != nil || len(keys.Strings()) != 1 {
		t.Fatal("deadline intent missing before first preview")
	}
	deadline, err := f.db.Do(ctx, "GET", keys.Strings()[0])
	if err != nil || deadline.Int64() != f.now.Load()+180 {
		t.Fatal("shorter grant reduced deadline")
	}
	client := f.server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Get(f.server.URL + "/v/" + first)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 302 {
		t.Fatalf("preview got%d", r.StatusCode)
	}
	// 管理记录独立于缓存键，必须能定位并删除真实 OSS 对象。
	objects, err := f.db.Do(ctx, "HVALS", f.cfg.Namespace+":objects")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects.Strings()) != 1 {
		t.Fatal("maintenance record missing")
	}
	for _, raw := range objects.Strings() {
		var v map[string]any
		if json.Unmarshal([]byte(raw), &v) != nil || v["object_key"] == "" {
			t.Fatal("invalid maintenance record")
		}
	}
	store, err := infrastructure.NewValkeyStore(f.cfg, func() time.Time { return time.Unix(f.now.Load(), 0) })
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close(ctx)
	preparer, err := infrastructure.NewAliyunPreviewStore(f.cfg, store, func() time.Time { return time.Unix(f.now.Load(), 0) })
	if err != nil {
		t.Fatal(err)
	}
	f.now.Add(121)
	if err = preparer.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	count, err := f.db.Do(ctx, "HLEN", f.cfg.Namespace+":objects")
	if err != nil || count.Int() != 1 {
		t.Fatal("cleanup ignored longer intent")
	}
	f.now.Add(60)
	if err = preparer.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	count, err = f.db.Do(ctx, "HLEN", f.cfg.Namespace+":objects")
	if err != nil || count.Int() != 0 {
		t.Fatal("expired object was not removed")
	}
}
