package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"path"
	"sort"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/repository"
	"github.com/gogf/gf/v2/errors/gerror"
)

type Service struct {
	store       repository.TokenStore
	preparer    repository.PreviewPreparer
	clock       func() time.Time
	maxCacheTTL int64
	profiles    map[string]bool
	NewToken    func() (string, error)
}

func New(store repository.TokenStore, preparer repository.PreviewPreparer, clock func() time.Time, max int64, profiles []string) *Service {
	s := &Service{store: store, preparer: preparer, clock: clock, maxCacheTTL: max, profiles: make(map[string]bool), NewToken: RandomToken}
	for _, p := range profiles {
		s.profiles[p] = true
	}
	return s
}

func RandomToken() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", gerror.Wrap(entity.ErrUnavailable, "random source unavailable")
	}
	return hex.EncodeToString(b[:]), nil
}

type Issued struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}
type Item struct {
	Token          string          `json:"token"`
	Filename       string          `json:"filename"`
	StorageProfile string          `json:"storage_profile"`
	CreatedAt      int64           `json:"created_at"`
	ExpiresAt      int64           `json:"expires_at"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}
type Page struct {
	Items  []Item `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (s *Service) Issue(ctx context.Context, caller string, input entity.IssueInput) (Issued, error) {
	if err := input.Validate(s.maxCacheTTL); err != nil {
		return Issued{}, gerror.Wrap(err, "invalid issue input")
	}
	if !s.profiles[input.StorageProfile] {
		return Issued{}, gerror.Wrap(entity.ErrUnavailable, "storage profile disabled")
	}
	now := s.clock().Unix()
	for n := 0; n < 5; n++ {
		token, err := s.NewToken()
		if err != nil {
			return Issued{}, err
		}
		if !entity.TokenPattern.MatchString(token) {
			return Issued{}, entity.ErrUnavailable
		}
		grant := entity.Grant{Token: token, URL: input.URL, StorageProfile: input.StorageProfile, ContentSHA256: input.ContentSHA256, Filename: input.Filename, CreatedAt: now, ExpiresAt: now + *input.TTL, CacheExpiresAt: now + *input.CacheTTL, Caller: caller, Metadata: input.Metadata}
		ok, err := s.store.Create(ctx, grant)
		if err != nil {
			return Issued{}, err
		}
		if ok {
			return Issued{token, grant.ExpiresAt}, nil
		}
	}
	return Issued{}, gerror.Wrap(entity.ErrUnavailable, "token collision limit reached")
}

func (s *Service) Query(ctx context.Context, limit, offset int) (Page, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return Page{}, entity.ErrInvalid
	}
	grants, err := s.store.List(ctx)
	if err != nil {
		return Page{}, err
	}
	sort.Slice(grants, func(i, j int) bool {
		if grants[i].CreatedAt == grants[j].CreatedAt {
			return grants[i].Token > grants[j].Token
		}
		return grants[i].CreatedAt > grants[j].CreatedAt
	})
	p := Page{Items: make([]Item, 0), Total: len(grants), Limit: limit, Offset: offset}
	if offset >= len(grants) {
		return p, nil
	}
	end := offset + limit
	if end < offset || end > len(grants) {
		end = len(grants)
	}
	for _, g := range grants[offset:end] {
		p.Items = append(p.Items, Item{g.Token, g.Filename, g.StorageProfile, g.CreatedAt, g.ExpiresAt, g.Metadata})
	}
	return p, nil
}
func (s *Service) Revoke(ctx context.Context, token string) error {
	if !entity.TokenPattern.MatchString(token) {
		return entity.ErrInvalid
	}
	return s.store.Revoke(ctx, token)
}
func (s *Service) Resolve(ctx context.Context, token string) (*entity.Grant, error) {
	if !entity.TokenPattern.MatchString(token) {
		return nil, entity.ErrNotFound
	}
	return s.store.Get(ctx, token)
}

func (s *Service) PreparePreview(ctx context.Context, token string) (string, error) {
	grant, err := s.Resolve(ctx, token)
	if err != nil {
		return "", err
	}
	if !entity.SupportedExtension(path.Ext(grant.Filename)) {
		return "", entity.ErrInvalid
	}
	if s.preparer == nil {
		return "", entity.ErrUnavailable
	}
	return s.preparer.Prepare(ctx, *grant)
}

func (s *Service) Ready(ctx context.Context) error {
	checker, ok := s.preparer.(repository.HealthChecker)
	if !ok {
		return entity.ErrUnavailable
	}
	return checker.Check(ctx)
}
