package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
)

type historicalTokenStore struct{ grant entity.Grant }

func (s *historicalTokenStore) Create(context.Context, entity.Grant) (bool, error) {
	return true, nil
}
func (s *historicalTokenStore) List(context.Context) ([]entity.Grant, error) {
	return []entity.Grant{s.grant}, nil
}
func (s *historicalTokenStore) Get(context.Context, string) (*entity.Grant, error) {
	return &s.grant, nil
}
func (*historicalTokenStore) Revoke(context.Context, string) error { return nil }

type historicalCachedPreview struct{ called bool }

func (p *historicalCachedPreview) Prepare(context.Context, entity.Grant) (string, error) {
	p.called = true
	return "https://preview.example/already-cached-object", nil
}

func TestTC_R1_HistoricalTokenCannotExposeExcludedCache(t *testing.T) {
	for _, filename := range []string{"legacy.txt", "legacy.pages", "legacy.avif", "legacy.docm"} {
		t.Run(filename, func(t *testing.T) {
			store := &historicalTokenStore{grant: entity.Grant{
				Token: "0123456789abcdef0123456789abcdef", Filename: filename,
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			}}
			cached := &historicalCachedPreview{}
			service := New(store, cached, time.Now, 86400, []string{"aliyun-oss"})
			location, err := service.PreparePreview(context.Background(), store.grant.Token)
			if !errors.Is(err, entity.ErrInvalid) || location != "" || cached.called {
				t.Fatalf("old token escaped current format policy: location=%q error=%v preparer_called=%v", location, err, cached.called)
			}
		})
	}
}
