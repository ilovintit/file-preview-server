package infrastructure

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

type CallerKey struct {
	ID     string
	Role   string
	Secret []byte
}

type Config struct {
	ValkeyAddress    string
	ValkeyUser       string
	ValkeyPassword   string
	ValkeyDB         int
	ValkeyTLS        bool
	Namespace        string
	MaxCacheTTL      int64
	Profiles         []string
	Keys             []CallerKey
	TrustedProxies   []netip.Prefix
	Address          string
	TLSCert          string
	TLSKey           string
	TLSClientCA      string
	AliyunOSS        AliyunOSSConfig
	SourceHTTPClient *http.Client
	OSSHTTPClient    *http.Client
	GotenbergURL     string
	Silo             SiloConfig
	SiloTransport    http.RoundTripper
}

type SiloConfig struct {
	Endpoint        string
	PreviewEndpoint string
	Region          string
	Bucket          string
	PrefixBase      string
	Generation      string
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	SignedURLMaxTTL int64
}

// AliyunOSSConfig holds one explicitly enabled OSS profile. Credentials are
// supplied only through the process environment or CI secret injection.
type AliyunOSSConfig struct {
	Endpoint        string
	PreviewEndpoint string
	Region          string
	Bucket          string
	PrefixBase      string
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	SignedURLMaxTTL int64
}

var namespacePattern = regexp.MustCompile(`^[A-Za-z0-9:_-]{1,96}$`)

func (c Config) Validate() error {
	if err := c.Silo.validate(); err != nil {
		return err
	}
	if c.GotenbergURL != "" {
		u, err := url.Parse(c.GotenbergURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return gerror.New("invalid GOTENBERG_URL")
		}
	}
	if c.MaxCacheTTL < 60 || c.MaxCacheTTL > 31536000 || !namespacePattern.MatchString(c.Namespace) || c.ValkeyDB < 0 || c.ValkeyDB > 15 {
		return gerror.New("invalid namespace, cache TTL or Valkey DB configuration")
	}
	if c.ValkeyAddress != "" {
		if _, _, err := net.SplitHostPort(c.ValkeyAddress); err != nil {
			return gerror.New("invalid VALKEY_ADDR")
		}
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return gerror.New("TLS_CERT_FILE and TLS_KEY_FILE must be configured together")
	}
	seen := map[string]bool{}
	for _, k := range c.Keys {
		if !keyIDPattern.MatchString(k.ID) || seen[k.ID] || (k.Role != "internal" && k.Role != "admin") || len(k.Secret) < 32 || len(k.Secret) > 128 {
			return gerror.New("invalid caller key configuration")
		}
		seen[k.ID] = true
	}
	seen = map[string]bool{}
	for _, p := range c.Profiles {
		if seen[p] || (p != "aliyun-oss" && p != "silo") {
			return gerror.New("invalid STORAGE_PROFILES")
		}
		seen[p] = true
	}
	oss := c.AliyunOSS
	configuredOSS := oss.Endpoint != "" || oss.Region != "" || oss.Bucket != "" || oss.PrefixBase != "" || oss.AccessKeyID != "" || oss.AccessKeySecret != "" || oss.SecurityToken != "" || oss.SignedURLMaxTTL != 0
	// 当前使用 V1 签名，Region 不参与签名计算，允许省略。
	if configuredOSS && (oss.Endpoint == "" || oss.Bucket == "" || oss.PrefixBase == "" || oss.AccessKeyID == "" || oss.AccessKeySecret == "" || oss.SignedURLMaxTTL < 1) {
		return gerror.New("incomplete Aliyun OSS configuration")
	}
	if configuredOSS {
		endpoint, err := url.Parse(oss.Endpoint)
		if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") || strings.Contains(oss.PrefixBase, "..") || strings.HasPrefix(oss.PrefixBase, "/") || strings.TrimSpace(oss.PrefixBase) != oss.PrefixBase || oss.SignedURLMaxTTL > 604800 {
			return gerror.New("invalid OSS endpoint, prefix or signing TTL")
		}
		if oss.PreviewEndpoint != "" {
			preview, err := url.Parse(oss.PreviewEndpoint)
			if err != nil || preview.Scheme != "https" || preview.Hostname() == "" || preview.User != nil || preview.RawQuery != "" || preview.Fragment != "" || (preview.Path != "" && preview.Path != "/") {
				return gerror.New("invalid OSS preview endpoint")
			}
		}
	}
	return nil
}

func LoadConfig() (Config, error) {
	c := Config{ValkeyAddress: os.Getenv("VALKEY_ADDR"), ValkeyUser: os.Getenv("VALKEY_USERNAME"), ValkeyPassword: os.Getenv("VALKEY_PASSWORD"), Namespace: os.Getenv("KEY_NAMESPACE"), Address: os.Getenv("LISTEN_ADDR"), TLSCert: os.Getenv("TLS_CERT_FILE"), TLSKey: os.Getenv("TLS_KEY_FILE"), TLSClientCA: os.Getenv("TLS_CLIENT_CA_FILE"), MaxCacheTTL: 86400}
	c.GotenbergURL = os.Getenv("GOTENBERG_URL")
	c.Silo = SiloConfig{
		Endpoint: os.Getenv("SILO_ENDPOINT"), PreviewEndpoint: os.Getenv("SILO_PREVIEW_ENDPOINT"),
		Region: os.Getenv("SILO_REGION"), Bucket: os.Getenv("SILO_BUCKET"), PrefixBase: os.Getenv("SILO_PREFIX_BASE"),
		Generation: os.Getenv("SILO_CONFIG_GENERATION"), AccessKeyID: os.Getenv("SILO_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("SILO_ACCESS_KEY_SECRET"), SecurityToken: os.Getenv("SILO_SECURITY_TOKEN"),
	}
	if raw := os.Getenv("SILO_SIGNED_URL_MAX_TTL_SECONDS"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c, gerror.New("invalid SILO_SIGNED_URL_MAX_TTL_SECONDS")
		}
		c.Silo.SignedURLMaxTTL = n
	}
	if c.Namespace == "" {
		c.Namespace = "preview:v1"
	}
	if c.Address == "" {
		c.Address = ":9501"
	}
	if value := os.Getenv("MAX_CACHE_TTL"); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return c, gerror.New("invalid MAX_CACHE_TTL")
		}
		c.MaxCacheTTL = n
	}
	if value := os.Getenv("VALKEY_DB"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			return c, gerror.New("invalid VALKEY_DB")
		}
		c.ValkeyDB = n
	}
	if value := os.Getenv("VALKEY_TLS"); value != "" {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return c, gerror.New("invalid VALKEY_TLS")
		}
		c.ValkeyTLS = v
	}
	if value := os.Getenv("STORAGE_PROFILES"); value != "" {
		for _, p := range strings.Split(value, ",") {
			c.Profiles = append(c.Profiles, strings.TrimSpace(p))
		}
	}
	if value := os.Getenv("TRUSTED_PROXY_CIDRS"); value != "" {
		for _, p := range strings.Split(value, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(p))
			if err != nil {
				return c, gerror.New("invalid TRUSTED_PROXY_CIDRS")
			}
			c.TrustedProxies = append(c.TrustedProxies, prefix.Masked())
		}
	}
	if raw := os.Getenv("CALLER_KEYS_JSON"); raw != "" {
		var keys []struct {
			ID     string `json:"id"`
			Role   string `json:"role"`
			Secret string `json:"secret_base64"`
		}
		d := json.NewDecoder(strings.NewReader(raw))
		d.DisallowUnknownFields()
		if err := d.Decode(&keys); err != nil {
			return c, gerror.New("invalid CALLER_KEYS_JSON")
		}
		for _, k := range keys {
			secret, err := base64.StdEncoding.DecodeString(k.Secret)
			if err != nil {
				return c, gerror.New("invalid caller key encoding")
			}
			c.Keys = append(c.Keys, CallerKey{ID: k.ID, Role: k.Role, Secret: secret})
		}
	}
	c.AliyunOSS = AliyunOSSConfig{
		Endpoint:        os.Getenv("PREVIEW_CI_ALIYUN_OSS_ENDPOINT"),
		PreviewEndpoint: os.Getenv("PREVIEW_CI_ALIYUN_OSS_PREVIEW_ENDPOINT"),
		Region:          os.Getenv("PREVIEW_CI_ALIYUN_OSS_REGION"),
		Bucket:          os.Getenv("PREVIEW_CI_ALIYUN_OSS_BUCKET"),
		PrefixBase:      os.Getenv("PREVIEW_CI_ALIYUN_OSS_PREFIX_BASE"),
		AccessKeyID:     os.Getenv("PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("PREVIEW_CI_ALIYUN_OSS_ACCESS_KEY_SECRET"),
		SecurityToken:   os.Getenv("PREVIEW_CI_ALIYUN_OSS_SECURITY_TOKEN"),
	}
	if value := os.Getenv("PREVIEW_CI_OSS_SIGNED_URL_MAX_TTL_SECONDS"); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return c, gerror.New("invalid PREVIEW_CI_OSS_SIGNED_URL_MAX_TTL_SECONDS")
		}
		c.AliyunOSS.SignedURLMaxTTL = n
	}
	return c, c.Validate()
}

func (c SiloConfig) configured() bool {
	return c.Endpoint != "" || c.PreviewEndpoint != "" || c.Region != "" || c.Bucket != "" || c.PrefixBase != "" || c.Generation != "" || c.AccessKeyID != "" || c.AccessKeySecret != "" || c.SecurityToken != "" || c.SignedURLMaxTTL != 0
}

func (c SiloConfig) validate() error {
	if !c.configured() {
		return nil
	}
	if c.Endpoint == "" || c.Bucket == "" || c.PrefixBase == "" || c.AccessKeyID == "" || c.AccessKeySecret == "" || c.SignedURLMaxTTL < 1 || c.SignedURLMaxTTL > 604800 || strings.Contains(c.PrefixBase, "..") || strings.HasPrefix(c.PrefixBase, "/") || strings.TrimSpace(c.PrefixBase) != c.PrefixBase || (c.Generation != "" && !namespacePattern.MatchString(c.Generation)) {
		return gerror.New("incomplete or invalid silo configuration")
	}
	for _, endpoint := range []string{c.Endpoint, c.previewEndpoint()} {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return gerror.New("invalid silo endpoint")
		}
	}
	preview, _ := url.Parse(c.previewEndpoint())
	if preview.Scheme != "https" {
		return gerror.New("silo preview endpoint requires HTTPS")
	}
	return nil
}

func (c SiloConfig) previewEndpoint() string {
	if c.PreviewEndpoint != "" {
		return c.PreviewEndpoint
	}
	return c.Endpoint
}
