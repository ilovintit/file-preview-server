package preview

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/application"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/interfaces"
)

type Module struct {
	handler  http.Handler
	store    *infrastructure.ValkeyStore
	stop     context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	closing  bool
	requests sync.WaitGroup
}

func New(cfg infrastructure.Config, clock func() time.Time) (*Module, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if clock == nil {
		clock = time.Now
	}
	store, err := infrastructure.NewValkeyStore(cfg, clock)
	if err != nil {
		return nil, err
	}
	preparer, err := infrastructure.NewAliyunPreviewStore(cfg, store, clock)
	if err != nil {
		_ = store.Close(context.Background())
		return nil, err
	}
	// 只有已实际装配的存储适配器才能用于签发；声明名称不能替代配置。
	var configuredProfiles []string
	for _, profile := range cfg.Profiles {
		if profile == "aliyun-oss" && preparer != nil {
			configuredProfiles = append(configuredProfiles, profile)
		}
	}
	service := application.New(store, preparer, clock, cfg.MaxCacheTTL, configuredProfiles)
	controller := interfaces.New(service)
	auth := infrastructure.NewAuthenticator(cfg, store, clock)
	issue := auth.Wrap("internal", controller.Issue, controller.Error)
	query := auth.Wrap("admin", controller.Query, controller.Error)
	revoke := auth.Wrap("admin", controller.Revoke, controller.Error)
	lifecycle, stop := context.WithCancel(context.Background())
	m := &Module{store: store, stop: stop, done: make(chan struct{})}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		if m.closing {
			m.mu.Unlock()
			controller.Error(w, r, entity.ErrUnavailable)
			return
		}
		m.requests.Add(1)
		m.mu.Unlock()
		defer m.requests.Done()
		budget := 10 * time.Second
		if cfg.GotenbergURL != "" && r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v/") {
			budget = 45 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		stopCancellation := context.AfterFunc(lifecycle, cancel)
		defer stopCancellation()
		r = r.WithContext(ctx)
		switch {
		case r.Method == "POST" && r.URL.Path == "/internal/tokens":
			issue.ServeHTTP(w, r)
		case r.Method == "POST" && r.URL.Path == "/admin/tokens/query":
			query.ServeHTTP(w, r)
		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/admin/tokens/") && strings.HasSuffix(r.URL.Path, "/revoke"):
			revoke.ServeHTTP(w, r)
		case r.Method == "GET" && r.URL.Path == "/livez":
			controller.Live(w, r)
		case r.Method == "GET" && r.URL.Path == "/readyz":
			controller.Ready(w, r)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v/"):
			controller.Preview(w, r)
		default:
			controller.Error(w, r, entity.ErrNotFound)
		}
	})
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-lifecycle.Done():
				return
			case <-ticker.C:
				if preparer != nil {
					batch, cancel := context.WithTimeout(lifecycle, 3*time.Second)
					_ = preparer.Cleanup(batch)
					cancel()
				}
			}
		}
	}()
	m.handler = h
	return m, nil
}
func (m *Module) Handler() http.Handler { return m.handler }
func (m *Module) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closing = true
	m.stop()
	m.mu.Unlock()
	drained := make(chan struct{})
	go func() { m.requests.Wait(); close(drained) }()
	select {
	case <-drained:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-m.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return m.store.Close(ctx)
}
