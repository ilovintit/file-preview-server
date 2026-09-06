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
		return nil, err
	}
	service := application.New(store, preparer, clock, cfg.MaxCacheTTL, cfg.Profiles)
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
	return &Module{handler: h, store: store}, nil
}
func (m *Module) Handler() http.Handler           { return m.handler }
func (m *Module) Close(ctx context.Context) error { return m.store.Close(ctx) }
