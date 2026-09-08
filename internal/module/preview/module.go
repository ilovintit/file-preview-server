package preview

import (
	"context"
	"net/http"
	"strings"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/application"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/interfaces"
)

type Module struct {
	handler http.Handler
	store   *infrastructure.ValkeyStore
	stop    context.CancelFunc
	done    chan struct{}
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
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
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
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if preparer != nil {
					batch, cancel := context.WithTimeout(ctx, 3*time.Second)
					_ = preparer.Cleanup(batch)
					cancel()
				}
			}
		}
	}()
	return &Module{handler: h, store: store, stop: stop, done: done}, nil
}
func (m *Module) Handler() http.Handler { return m.handler }
func (m *Module) Close(ctx context.Context) error {
	m.stop()
	select {
	case <-m.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return m.store.Close(ctx)
}
