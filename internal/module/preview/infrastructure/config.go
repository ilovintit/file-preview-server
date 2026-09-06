package infrastructure

import "net/netip"

type CallerKey struct {
	ID     string
	Role   string
	Secret []byte
}

type Config struct {
	ValkeyAddress  string
	ValkeyUser     string
	ValkeyPassword string
	ValkeyDB       int
	ValkeyTLS      bool
	Namespace      string
	MaxCacheTTL    int64
	Profiles       []string
	Keys           []CallerKey
	TrustedProxies []netip.Prefix
	Address        string
	TLSCert        string
	TLSKey         string
}
