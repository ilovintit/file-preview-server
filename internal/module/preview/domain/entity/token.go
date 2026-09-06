package entity

import (
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Domain sentinels stay framework-independent; the HTTP boundary maps them through gcode.
var (
	ErrInvalid      = errors.New("invalid preview input")
	ErrUnauthorized = errors.New("unauthorized preview request")
	ErrUnavailable  = errors.New("preview dependency unavailable")
	ErrNotFound     = errors.New("preview token not found")
)

var (
	TokenPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	HashPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type IssueInput struct {
	URL            string          `json:"url"`
	StorageProfile string          `json:"storage_profile"`
	ContentSHA256  string          `json:"content_sha256"`
	Filename       string          `json:"filename"`
	TTL            *int64          `json:"ttl"`
	CacheTTL       *int64          `json:"cache_ttl"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type Grant struct {
	Token          string          `json:"token"`
	URL            string          `json:"url"`
	StorageProfile string          `json:"storage_profile"`
	ContentSHA256  string          `json:"content_sha256"`
	Filename       string          `json:"filename"`
	CreatedAt      int64           `json:"created_at"`
	ExpiresAt      int64           `json:"expires_at"`
	CacheExpiresAt int64           `json:"cache_expires_at"`
	Caller         string          `json:"caller"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

func (in IssueInput) Validate(maxCacheTTL int64) error {
	if in.TTL == nil || in.CacheTTL == nil || *in.TTL < 60 || *in.TTL > 86400 || *in.CacheTTL < *in.TTL || *in.CacheTTL > maxCacheTTL {
		return ErrInvalid
	}
	u, err := url.ParseRequestURI(in.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(in.URL) > 8192 {
		return ErrInvalid
	}
	if in.StorageProfile != "aliyun-oss" && in.StorageProfile != "silo" {
		return ErrInvalid
	}
	if !HashPattern.MatchString(in.ContentSHA256) || len(in.Filename) == 0 || len(in.Filename) > 255 || strings.ContainsAny(in.Filename, "/\\") || strings.ContainsAny(in.Filename, "\x00\r\n") {
		return ErrInvalid
	}
	for _, r := range in.Filename {
		if r < 32 || r == 127 {
			return ErrInvalid
		}
	}
	if !SupportedExtension(strings.ToLower(path.Ext(in.Filename))) {
		return ErrInvalid
	}
	if len(in.Metadata) > 0 {
		if len(in.Metadata) > 4096 {
			return ErrInvalid
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(in.Metadata, &obj) != nil || obj == nil {
			return ErrInvalid
		}
	}
	return nil
}
