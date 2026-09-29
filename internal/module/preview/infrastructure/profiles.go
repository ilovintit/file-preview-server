package infrastructure

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/domain/entity"
)

type ProfilePreparer struct {
	stores       map[string]*PreviewStore
	cache        *ValkeyStore
	complete     bool
	converterURL string
}

func NewProfilePreparer(cfg Config, cache *ValkeyStore, clock func() time.Time) (*ProfilePreparer, error) {
	p := &ProfilePreparer{stores: make(map[string]*PreviewStore), cache: cache, converterURL: cfg.GotenbergURL}
	admission, slots := make(chan struct{}, 36), make(chan struct{}, 4)
	for _, name := range cfg.Profiles {
		var store *PreviewStore
		var err error
		switch name {
		case "aliyun-oss":
			store, err = NewAliyunPreviewStore(cfg, cache, clock)
		case "silo":
			store, err = NewSiloPreviewStore(cfg, cache, clock)
		default:
			return nil, entity.ErrInvalid
		}
		if err != nil {
			return nil, err
		}
		if store != nil {
			store.admission, store.slots = admission, slots
			p.stores[name] = store
		}
	}
	roles := map[string]bool{}
	for _, key := range cfg.Keys {
		roles[key.Role] = true
	}
	p.complete = len(p.stores) > 0 && len(p.stores) == len(cfg.Profiles) && roles["internal"] && roles["admin"]
	return p, nil
}

func (p *ProfilePreparer) Profiles() []string {
	var names []string
	for _, name := range []string{"aliyun-oss", "silo"} {
		if p.stores[name] != nil {
			names = append(names, name)
		}
	}
	return names
}
func (p *ProfilePreparer) Prepare(ctx context.Context, grant entity.Grant) (string, error) {
	store := p.stores[grant.StorageProfile]
	if store == nil {
		return "", entity.ErrUnavailable
	}
	return store.Prepare(ctx, grant)
}
func (p *ProfilePreparer) Cleanup(ctx context.Context) error {
	if len(p.stores) == 0 {
		return nil
	}
	var identities []string
	for _, store := range p.stores {
		identities = append(identities, store.identity)
	}
	records, err := p.cache.cleanupCandidates(ctx, identities...)
	if err != nil {
		return err
	}
	for _, record := range records {
		for _, store := range p.stores {
			if !strings.HasPrefix(record.Identity, store.identity+":") || !strings.HasPrefix(record.ObjectKey, store.prefix+"/"+store.identity+"/") {
				continue
			}
			if store.storage.Delete(ctx, record.ObjectKey) != nil {
				continue
			}
			if err := p.cache.forgetObject(ctx, record.ObjectKey); err != nil {
				return err
			}
			break
		}
	}
	return nil
}
func (p *ProfilePreparer) Check(ctx context.Context) error {
	if !p.complete {
		return entity.ErrUnavailable
	}
	if _, _, err := p.cache.ensure(ctx); err != nil {
		return err
	}
	for _, store := range p.stores {
		if err := store.storage.Check(ctx); err != nil {
			return err
		}
	}
	if p.converterURL != "" {
		probe, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(probe, http.MethodGet, strings.TrimRight(p.converterURL, "/")+"/health", nil)
		if err != nil {
			return entity.ErrUnavailable
		}
		client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(req)
		if err != nil {
			return entity.ErrUnavailable
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			return entity.ErrUnavailable
		}
	}
	return nil
}
