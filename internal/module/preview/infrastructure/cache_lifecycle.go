package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/util/gconv"
)

const preparationLease = 3 * time.Second

// Lease operations compare both the security epoch and the random owner.
func (s *ValkeyStore) acquirePreparation(ctx context.Context, identity, owner string) (bool, error) {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return false, err
	}
	prefix := s.epochPrefix(st)
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1])~=ARGV[1] then return -1 end
if tonumber(redis.call('GET',KEYS[3]) or '0')<=tonumber(ARGV[4]) then return -1 end
if redis.call('SET',KEYS[2],ARGV[2],'NX','PX',ARGV[3]) then return 1 end;return 0`, 3, s.guardKey(), prefix+"lease:"+identity, prefix+"deadline:"+identity, guard, owner, preparationLease.Milliseconds(), s.clock().Unix())
	if err != nil || r.Int() < 0 {
		return false, unavailable()
	}
	return r.Int() == 1, nil
}

func (s *ValkeyStore) updatePreparation(ctx context.Context, identity, owner string, release bool) error {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	ttl := preparationLease.Milliseconds()
	if release {
		ttl = 0
	}
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1])~=ARGV[1] or redis.call('GET',KEYS[2])~=ARGV[2] then return 0 end
if tonumber(ARGV[3])==0 then redis.call('DEL',KEYS[2]) else redis.call('PEXPIRE',KEYS[2],ARGV[3]) end;return 1`, 2, s.guardKey(), s.epochPrefix(st)+"lease:"+identity, guard, owner, ttl)
	if err != nil || r.Int() != 1 {
		return unavailable()
	}
	return nil
}

type maintenanceRecord struct {
	Identity    string `json:"identity"`
	EpochPrefix string `json:"epoch_prefix"`
	ObjectKey   string `json:"object_key"`
	NotBefore   int64  `json:"not_before"`
}

// Candidates are recorded before upload, independently of expiring cache keys.
func (s *ValkeyStore) registerObject(ctx context.Context, identity, object string, notBefore int64) error {
	st, guard, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(maintenanceRecord{identity, s.epochPrefix(st), object, notBefore})
	r, err := s.db.Do(ctx, "EVAL", `if redis.call('GET',KEYS[1])~=ARGV[1] then return 0 end
redis.call('HSET',KEYS[2],ARGV[2],ARGV[3]);redis.call('ZADD',KEYS[3],ARGV[4],ARGV[2]);return 1`, 3, s.guardKey(), s.namespace+":objects", s.namespace+":objects:due", guard, object, string(data), notBefore)
	if err != nil || r.Int() != 1 {
		return unavailable()
	}
	return nil
}

// A cleanup claim first removes the matching published generation atomically.
// A later grant may extend the deadline or publish a different immutable object.
func (s *ValkeyStore) cleanupCandidates(ctx context.Context, identities ...string) ([]maintenanceRecord, error) {
	now := s.clock().Unix()
	objects, err := s.cleanupObjects(ctx, now, identities)
	if err != nil {
		return nil, unavailable()
	}
	var result []maintenanceRecord
	for _, object := range objects {
		raw, err := s.db.Do(ctx, "HGET", s.namespace+":objects", object)
		if err != nil {
			return nil, unavailable()
		}
		var record maintenanceRecord
		if raw.String() == "" {
			_, _ = s.db.Do(ctx, "ZREM", s.namespace+":objects:due", object)
			continue
		}
		if json.Unmarshal([]byte(raw.String()), &record) != nil {
			return nil, unavailable()
		}
		if len(identities) > 0 {
			owned := false
			for _, identity := range identities {
				if strings.HasPrefix(record.Identity, identity+":") {
					owned = true
					break
				}
			}
			if !owned {
				continue
			}
		}
		claim, err := s.db.Do(ctx, "EVAL", `local due=redis.call('ZSCORE',KEYS[3],ARGV[1]);if not due or tonumber(due)>tonumber(ARGV[2]) then return 0 end
local raw=redis.call('GET',KEYS[1]);local deadline=tonumber(redis.call('GET',KEYS[2]) or '0')
if raw then local current=cjson.decode(raw)
 if current.object_key==ARGV[1] then
  if deadline>tonumber(ARGV[2]) then redis.call('ZADD',KEYS[3],deadline,ARGV[1]);return 0 end
  redis.call('DEL',KEYS[1])
 end
end
redis.call('ZADD',KEYS[3],tonumber(ARGV[2])+30,ARGV[1]);return 1`, 3, record.EpochPrefix+"cache:"+record.Identity, record.EpochPrefix+"deadline:"+record.Identity, s.namespace+":objects:due", object, now)
		if err != nil {
			return nil, unavailable()
		}
		if claim.Int() == 1 {
			result = append(result, record)
		}
	}
	return result, nil
}

// Each active profile has a bounded scan cursor. Old/inactive profile records
// remain untouched and cannot permanently occupy the first due page, starving
// cleanup for a different profile. The existing maintenance schema stays valid.
func (s *ValkeyStore) cleanupObjects(ctx context.Context, now int64, identities []string) ([]string, error) {
	if len(identities) == 0 {
		r, err := s.db.Do(ctx, "ZRANGEBYSCORE", s.namespace+":objects:due", "-inf", now, "LIMIT", 0, 32)
		if err != nil {
			return nil, unavailable()
		}
		return r.Strings(), nil
	}
	var result []string
	quota := 32 / len(identities)
	for _, identity := range identities {
		cursorKey := s.namespace + ":cleanup-cursor:" + identity
		cursor, err := s.db.Do(ctx, "GET", cursorKey)
		if err != nil {
			return nil, unavailable()
		}
		position := cursor.String()
		if position == "" {
			position = "0"
		}
		scan, err := s.db.Do(ctx, "ZSCAN", s.namespace+":objects:due", position, "MATCH", "*/"+identity+"/*", "COUNT", 32)
		if err != nil {
			return nil, unavailable()
		}
		parts := scan.Slice()
		if len(parts) != 2 {
			return nil, unavailable()
		}
		if _, err = s.db.Do(ctx, "SET", cursorKey, gconv.String(parts[0]), "EX", 86400); err != nil {
			return nil, unavailable()
		}
		entries := gconv.Strings(parts[1])
		count := 0
		for i := 0; i+1 < len(entries) && count < quota; i += 2 {
			due, err := strconv.ParseFloat(entries[i+1], 64)
			if err != nil {
				return nil, unavailable()
			}
			if due <= float64(now) {
				result = append(result, entries[i])
				count++
			}
		}
	}
	return result, nil
}

func (s *ValkeyStore) forgetObject(ctx context.Context, object string) error {
	_, err := s.db.Do(ctx, "EVAL", `redis.call('HDEL',KEYS[1],ARGV[1]);redis.call('ZREM',KEYS[2],ARGV[1]);return 1`, 2, s.namespace+":objects", s.namespace+":objects:due", object)
	if err != nil {
		return unavailable()
	}
	return nil
}
