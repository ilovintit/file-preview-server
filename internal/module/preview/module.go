package preview

import (
	"context"
	"net/http"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/infrastructure"
)

// Module is the S01 assembly boundary. This first revision intentionally has no authorization implementation.
type Module struct{}

func New(_ infrastructure.Config, _ func() time.Time) (*Module, error) { return &Module{}, nil }
func (m *Module) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotImplemented) })
}
func (m *Module) Close(context.Context) error { return nil }
