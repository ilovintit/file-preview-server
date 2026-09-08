package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.shw.top/shw-project/file-preview-server/internal/module/preview/domain/entity"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/errors/gerror"
)

const recoveryWindow int64 = 601

type safetyState struct {
	RunID   string `json:"run_id"`
	Epoch   string `json:"epoch"`
	ReadyAt int64  `json:"ready_at"`
	Evicted int64  `json:"evicted"`
}
type ValkeyStore struct {
	db              *gredis.Redis
	namespace       string
	clock           func() time.Time
	mu              sync.Mutex
	lastTime        int64
	profileIdentity string
}

type cacheRecord struct {
	ObjectKey string `json:"object_key"`
	ExpiresAt int64  `json:"expires_at"`
	MediaType string `json:"media_type"`
}

func NewValkeyStore(cfg Config, clock func() time.Time) (*ValkeyStore, error) {
	s := &ValkeyStore{namespace: cfg.Namespace, clock: clock, profileIdentity: ossProfileIdentity(cfg.AliyunOSS)}
	if cfg.ValkeyAddress == "" {
		return s, nil
	}
	db, err := gredis.New(&gredis.Config{Address: cfg.ValkeyAddress, User: cfg.ValkeyUser, Pass: cfg.ValkeyPassword, Db: cfg.ValkeyDB, TLS: cfg.ValkeyTLS, Protocol: 2, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxActive: 32})
	if err != nil {
		return nil, gerror.Wrap(entity.ErrUnavailable, "invalid Valkey adapter configuration")
	}
	s.db = db
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, _ = s.ensure(ctx)
	return s, nil
}

func (s *ValkeyStore) Close(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close(ctx)
}
func (s *ValkeyStore) guardKey() string { return s.namespace + ":security-state" }
func (s *ValkeyStore) epochPrefix(st safetyState) string {
	return s.namespace + ":auth:" + st.Epoch + ":"
}
func unavailable() error {
	return gerror.Wrap(entity.ErrUnavailable, "authorization state unavailable")
}

func (s *ValkeyStore) ensure(ctx context.Context) (safetyState, string, error) {
	var state safetyState
	if s.db == nil {
		return state, "", unavailable()
	}
	now := s.clock().Unix()
	s.mu.Lock()
	backwards := now < s.lastTime
	if !backwards {
		s.lastTime = now
	}
	s.mu.Unlock()
	if backwards {
		return state, "", unavailable()
	}
	info, err := s.db.Do(ctx, "INFO", "server", "memory", "stats")
	if err != nil {
		return state, "", unavailable()
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(info.String(), "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), ":"); ok {
			fields[key] = value
		}
	}
	runID := fields["run_id"]
	evicted, err := strconv.ParseInt(fields["evicted_keys"], 10, 64)
	if runID == "" || err != nil || fields["maxmemory_policy"] != "noeviction" {
		return state, "", unavailable()
	}
	for attempt := 0; attempt < 3; attempt++ {
		value, err := s.db.Do(ctx, "GET", s.guardKey())
		if err != nil {
			return state, "", unavailable()
		}
		raw := value.String()
		if raw != "" && json.Unmarshal([]byte(raw), &state) == nil && state.RunID == runID && state.Evicted == evicted && entity.TokenPattern.MatchString(state.Epoch) {
			if now < state.ReadyAt {
				return state, raw, unavailable()
			}
			return state, raw, nil
		}
		var entropy [16]byte
		if _, err = rand.Read(entropy[:]); err != nil {
			return state, "", unavailable()
		}
		next := safetyState{RunID: runID, Epoch: hex.EncodeToString(entropy[:]), ReadyAt: now + recoveryWindow, Evicted: evicted}
		encoded, _ := json.Marshal(next)
		result, err := s.db.Do(ctx, "EVAL", `if (redis.call('GET',KEYS[1]) or '') == ARGV[1] then redis.call('SET',KEYS[1],ARGV[2]);return 1 end;return 0`, 1, s.guardKey(), raw, string(encoded))
		if err != nil {
			return state, "", unavailable()
		}
		if result.Int() == 1 {
			return next, string(encoded), unavailable()
		}
	}
	return state, "", unavailable()
}

func (s *ValkeyStore) ConsumeNonce(ctx context.Context, keyID, nonce string, timestamp int64) error {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	now := s.clock().Unix()
	if timestamp < now-300 || timestamp > now+300 {
		return entity.ErrUnauthorized
	}
	result, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] then return -1 end;if redis.call('SET',KEYS[2],'1','NX','EXAT',ARGV[2]) then return 1 end;return 0`, 2, s.guardKey(), s.epochPrefix(st)+"nonce:"+keyID+":"+nonce, guard, timestamp+301)
	if err != nil || result.Int() < 0 {
		return unavailable()
	}
	if result.Int() == 0 {
		return entity.ErrUnauthorized
	}
	return nil
}

func (s *ValkeyStore) Create(ctx context.Context, g entity.Grant) (bool, error) {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return false, err
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return false, entity.ErrInvalid
	}
	prefix := s.epochPrefix(st)
	identity := s.profileIdentity + ":" + outputVersion(g.Filename) + ":" + g.ContentSHA256
	result, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] then return -1 end
for _,id in ipairs(redis.call('ZRANGE',KEYS[2],0,-1)) do if not redis.call('GET',ARGV[6]..id) then redis.call('ZREM',KEYS[2],id) end end
if redis.call('ZCARD',KEYS[2]) >= 10000 then return -1 end
if not redis.call('SET',KEYS[3],ARGV[2],'NX','EXAT',ARGV[4]) then return 0 end
redis.call('ZADD',KEYS[2],ARGV[5],ARGV[3]);if redis.call('TTL',KEYS[2]) == -1 then redis.call('EXPIREAT',KEYS[2],ARGV[4]) else redis.call('EXPIREAT',KEYS[2],ARGV[4],'GT') end
if ARGV[8]=='aliyun-oss' then
local deadline=math.max(tonumber(redis.call('GET',KEYS[4]) or '0'),tonumber(ARGV[7]))
redis.call('SET',KEYS[4],deadline,'EXAT',deadline)
end;return 1`, 4, s.guardKey(), prefix+"tokens:index", prefix+"token:"+g.Token, prefix+"deadline:"+identity, guard, string(raw), g.Token, g.ExpiresAt, g.CreatedAt, prefix+"token:", g.CacheExpiresAt, g.StorageProfile)
	if err != nil || result.Int() < 0 {
		return false, unavailable()
	}
	return result.Int() == 1, nil
}

func (s *ValkeyStore) List(ctx context.Context) ([]entity.Grant, error) {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return nil, err
	}
	prefix := s.epochPrefix(st)
	now := s.clock().Unix()
	result, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] then return {'!guard'} end
local ids=redis.call('ZRANGE',KEYS[2],0,-1);local result={}
for _,id in ipairs(ids) do local raw=redis.call('GET',ARGV[2]..id);if raw then table.insert(result,raw) else redis.call('ZREM',KEYS[2],id) end end;return result`, 2, s.guardKey(), prefix+"tokens:index", guard, prefix+"token:")
	if err != nil {
		return nil, unavailable()
	}
	out := make([]entity.Grant, 0)
	for _, raw := range result.Strings() {
		var g entity.Grant
		if json.Unmarshal([]byte(raw), &g) != nil {
			return nil, unavailable()
		}
		if g.ExpiresAt > now {
			out = append(out, g)
		}
	}
	return out, nil
}

func (s *ValkeyStore) Get(ctx context.Context, token string) (*entity.Grant, error) {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return nil, err
	}
	prefix := s.epochPrefix(st)
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] then return '!guard' end;return redis.call('GET',KEYS[2]) or ''`, 2, s.guardKey(), prefix+"token:"+token, guard)
	if err != nil {
		return nil, unavailable()
	}
	if r.String() == "" {
		return nil, entity.ErrNotFound
	}
	var g entity.Grant
	if json.Unmarshal([]byte(r.String()), &g) != nil {
		return nil, unavailable()
	}
	if g.ExpiresAt <= s.clock().Unix() {
		return nil, entity.ErrNotFound
	}
	return &g, nil
}

func (s *ValkeyStore) Revoke(ctx context.Context, token string) error {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	prefix := s.epochPrefix(st)
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] then return -1 end;redis.call('DEL',KEYS[2]);redis.call('ZREM',KEYS[3],ARGV[2]);return 1`, 3, s.guardKey(), prefix+"token:"+token, prefix+"tokens:index", guard, token)
	if err != nil || r.Int() < 0 {
		return unavailable()
	}
	return nil
}

func (s *ValkeyStore) GetCache(ctx context.Context, identity string) (*cacheRecord, error) {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return nil, err
	}
	prefix := s.epochPrefix(st)
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] then return '!guard' end
local raw=redis.call('GET',KEYS[2]);if not raw then return '' end
local record=cjson.decode(raw);local deadline=tonumber(redis.call('GET',KEYS[3]) or '0')
if deadline<=tonumber(ARGV[2]) then return '' end
record.expires_at=deadline;return cjson.encode(record)`, 3, s.guardKey(), prefix+"cache:"+identity, prefix+"deadline:"+identity, guard, s.clock().Unix())
	if err != nil || r.String() == "!guard" {
		return nil, unavailable()
	}
	if r.String() == "" {
		return nil, entity.ErrNotFound
	}
	var value cacheRecord
	if json.Unmarshal([]byte(r.String()), &value) != nil || value.ExpiresAt <= s.clock().Unix() {
		return nil, entity.ErrNotFound
	}
	return &value, nil
}

func (s *ValkeyStore) PutCache(ctx context.Context, identity, owner string, value cacheRecord) error {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return unavailable()
	}
	prefix := s.epochPrefix(st)
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1]) ~= ARGV[1] or redis.call('GET',KEYS[3]) ~= ARGV[3] then return -1 end
local deadline=tonumber(redis.call('GET',KEYS[4]) or '0');if deadline<=tonumber(ARGV[4]) then return -1 end
redis.call('SET',KEYS[2],ARGV[2]);return 1`, 4, s.guardKey(), prefix+"cache:"+identity, prefix+"lease:"+identity, prefix+"deadline:"+identity, guard, string(raw), owner, s.clock().Unix())
	if err != nil || r.Int() != 1 {
		return unavailable()
	}
	return nil
}
