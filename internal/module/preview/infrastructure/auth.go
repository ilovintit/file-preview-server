package infrastructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ilovintit/file-preview-server/internal/module/preview/domain/entity"
)

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type NonceStore interface {
	ConsumeNonce(context.Context, string, string, int64) error
}
type Authenticator struct {
	keys    map[string]CallerKey
	roles   map[string]bool
	trusted []netip.Prefix
	store   NonceStore
	clock   func() time.Time
}
type SignedHandler func(http.ResponseWriter, *http.Request, string)

func NewAuthenticator(cfg Config, store NonceStore, clock func() time.Time) *Authenticator {
	a := &Authenticator{keys: make(map[string]CallerKey), roles: make(map[string]bool), trusted: append([]netip.Prefix(nil), cfg.TrustedProxies...), store: store, clock: clock}
	for _, k := range cfg.Keys {
		k.Secret = append([]byte(nil), k.Secret...)
		a.keys[k.ID] = k
		a.roles[k.Role] = true
	}
	return a
}

func (a *Authenticator) Secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	values := r.Header.Values("X-Forwarded-Proto")
	if len(values) != 1 || values[0] != "https" {
		return false
	}
	for _, p := range a.trusted {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

func (a *Authenticator) Wrap(role string, next SignedHandler, writeError func(http.ResponseWriter, *http.Request, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.roles[role] {
			writeError(w, r, entity.ErrUnavailable)
			return
		}
		if !a.Secure(r) || r.Method != "POST" || r.URL.RawQuery != "" || r.URL.EscapedPath() != r.URL.Path {
			writeError(w, r, entity.ErrUnauthorized)
			return
		}
		headers := []string{"X-Preview-Key-Id", "X-Preview-Timestamp", "X-Preview-Nonce", "X-Preview-Signature"}
		for _, h := range headers {
			if len(r.Header.Values(h)) != 1 {
				writeError(w, r, entity.ErrUnauthorized)
				return
			}
		}
		id := r.Header.Get(headers[0])
		ts := r.Header.Get(headers[1])
		nonce := r.Header.Get(headers[2])
		signed := r.Header.Get(headers[3])
		key, ok := a.keys[id]
		timestamp, err := strconv.ParseInt(ts, 10, 64)
		now := a.clock().Unix()
		if !ok || key.Role != role || !keyIDPattern.MatchString(id) || err != nil || strconv.FormatInt(timestamp, 10) != ts || timestamp < now-300 || timestamp > now+300 || !entity.TokenPattern.MatchString(nonce) || !entity.HashPattern.MatchString(signed) {
			writeError(w, r, entity.ErrUnauthorized)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 65537))
		if err != nil || len(raw) > 65536 {
			writeError(w, r, entity.ErrInvalid)
			return
		}
		digest := sha256.Sum256(raw)
		canonical := strings.Join([]string{r.Method, r.URL.Path, id, ts, nonce, hex.EncodeToString(digest[:])}, "\n")
		mac := hmac.New(sha256.New, key.Secret)
		_, _ = mac.Write([]byte(canonical))
		received, _ := hex.DecodeString(signed)
		if !hmac.Equal(mac.Sum(nil), received) {
			writeError(w, r, entity.ErrUnauthorized)
			return
		}
		if err = a.store.ConsumeNonce(r.Context(), id, nonce, timestamp); err != nil {
			writeError(w, r, err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		next(w, r, id)
	})
}
