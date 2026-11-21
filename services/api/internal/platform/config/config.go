// Package config loads the typed service configuration from VG_* environment variables and
// refuses insecure values outside development/test (ARCHITECTURE AD-20, NFR-3).
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"veyrogram/api/internal/platform/clientip"
	"veyrogram/api/internal/platform/token"
)

// Env is the deployment environment. An unset or empty VG_ENV is treated as production (fail closed).
type Env string

const (
	Development Env = "development"
	Test        Env = "test"
	Staging     Env = "staging"
	Production  Env = "production"
)

// Config is the complete typed configuration. Add fields here, never read os.Getenv elsewhere.
type Config struct {
	Env                Env
	HTTPAddr           string // public API listener (behind Nginx)
	WorkerInternalAddr string // worker process /metrics + health (must differ from InternalAddr; loopback/private only)
	InternalAddr       string // /healthz, /readyz (and later /metrics): must be loopback/private
	Debug              bool
	SMSFixedCode       string   // upstream-style "test SMS code"; must be empty outside development/test
	AdminPassword      string   // there is no configured admin password: the install wizard creates the first admin
	TokenSigningKey    string   // symmetric key placeholder until asymmetric `kid` keys land (AD-9)
	CORSAllowedOrigins []string // exact origins; "*" is rejected outside development/test
	MetricsToken       string   // bearer token for /metrics on the internal listener; empty disables /metrics
	IMAPIURL           string   // WuKongIM API base URL; loopback/private only (AD-2)
	IMAPIPeerURLs      []string // API URLs of the other nodes of an IM cluster (VG_IM_API_PEER_URLS): device_quit must reach every node; empty for the supported single node
	IMWebhookAddr      string   // listener for IM->business callbacks; loopback/private only, never routed by Nginx
	AuditAnchorURL     string   // off-host store for daily audit-log head anchors (https; object storage with versioning/WORM or a log host)
	AuditAnchorToken   string
	IMWebhookAllow     string // comma-separated CIDRs/IPs allowed to call the IM callback listener (default: loopback only)
	IMWebhookSigning   string // optional HMAC key; when set, callbacks must carry a valid signature (needs the patched WuKongIM)
	IMWebhookSecret    string // path secret for the callback URL (WuKongIM sends no signature); empty disables the callback listener
	MySQLDSN           string
	AuthSMSLogin       bool   // feature flag auth.sms_login: /v1/auth/* is not routed unless true (Story 1.12)
	SMSProvider        string // "http" (generic JSON gateway) or "log" (development/test only; prints codes to the log)
	SMSHTTPURL         string // gateway endpoint for the "http" provider; https outside development/test
	SMSHTTPToken       string // bearer token for the gateway
	SMSBaoUser         string // 短信宝 account (provider "smsbao")
	SMSBaoKey          string // 短信宝 ApiKey (or MD5 of the password); a secret
	SMSBaoBaseURL      string // default https://api.smsbao.com; override only for tests
	TokenKeys          string // kid:base64url(32-byte seed)[,...]; the first key signs access tokens (AD-9)
	PhoneHashKey       string // HMAC key for the phone blind index and one-time-code hashes
	RedisAddr          string // host:port of the Redis 7.x holding the token deny-list; loopback/private only
	RedisPassword      string
	MessageRateLimit   int      // messages one account may send per window before the gate answers 133 (0 disables); VG_MESSAGE_RATE_LIMIT, default 30
	MessageRateWindow  int      // window in seconds; VG_MESSAGE_RATE_WINDOW_SECONDS, default 10
	ManagerEnabled     bool     // admin plane (Story 1.19): a separate listener serving /v1/manager/*; VG_MANAGER_ENABLED
	ManagerAddr        string   // VG_MANAGER_ADDR, default 127.0.0.1:9103; Nginx publishes it on its own hostname
	ManagerAllow       []string // VG_MANAGER_ALLOW: optional CIDR/IP allowlist for the admin plane (empty = any address that can reach the listener)
	ManagerPublicRate  int      // VG_MANAGER_PUBLIC_RATE: login/second-factor requests per minute per address (default 20)
	AdminTOTPKey       string   // VG_ADMIN_TOTP_KEY: encrypts admins' TOTP secrets at rest (>= 32 chars)
	IMTokenKey         string   // HMAC key deriving WuKongIM connection tokens
	TrustedProxies     []string // CIDRs/IPs whose X-Forwarded-For is honoured; empty trusts nobody (AD-16)
}

// Violation is one rejected setting.
type Violation struct {
	Key    string
	Reason string
}

func (v Violation) String() string { return v.Key + ": " + v.Reason }

// weakValues are defaults and well-known secrets seen in the upstream samples and docs (risk register R-02/R-03/R-20).
var weakValues = map[string]bool{
	"123456": true, "admin1234567": true, "aa1234567": true, "aa12345678": true, "wk_secret_123": true,
	"wukongim-dev-manager-secret": true, "change-me": true, "a1234567": true, "secret": true, "password": true, "demo": true,
}

// Load reads configuration through getenv (use os.Getenv in production code, a map lookup in tests) and returns the
// config together with every violation found. A non-empty violation list means the service must not start.
func Load(getenv func(string) string) (Config, []Violation) {
	var vs []Violation
	c := Config{
		HTTPAddr:           orDefault(getenv("VG_HTTP_ADDR"), "127.0.0.1:8080"),
		InternalAddr:       orDefault(getenv("VG_INTERNAL_ADDR"), "127.0.0.1:9101"),
		WorkerInternalAddr: orDefault(getenv("VG_WORKER_INTERNAL_ADDR"), "127.0.0.1:9103"),
		SMSFixedCode:       getenv("VG_SMS_FIXED_CODE"),
		AdminPassword:      getenv("VG_ADMIN_PASSWORD"),
		TokenSigningKey:    getenv("VG_TOKEN_SIGNING_KEY"),
		MySQLDSN:           getenv("VG_MYSQL_DSN"),
		IMAPIURL:           orDefault(getenv("VG_IM_API_URL"), "http://127.0.0.1:5001"),
		IMAPIPeerURLs:      splitList(getenv("VG_IM_API_PEER_URLS")),
		IMWebhookAddr:      orDefault(getenv("VG_IM_WEBHOOK_ADDR"), "127.0.0.1:9102"),
		IMWebhookSecret:    getenv("VG_IM_WEBHOOK_SECRET"),
		IMWebhookSigning:   getenv("VG_IM_WEBHOOK_SIGNING_SECRET"),
		IMWebhookAllow:     getenv("VG_IM_WEBHOOK_ALLOW"),
		AuditAnchorURL:     getenv("VG_AUDIT_ANCHOR_URL"),
		AuditAnchorToken:   getenv("VG_AUDIT_ANCHOR_TOKEN"),
		MetricsToken:       getenv("VG_METRICS_TOKEN"),
		SMSProvider:        strings.ToLower(getenv("VG_SMS_PROVIDER")),
		SMSHTTPURL:         getenv("VG_SMS_HTTP_URL"),
		SMSHTTPToken:       getenv("VG_SMS_HTTP_TOKEN"),
		SMSBaoUser:         getenv("VG_SMSBAO_USER"),
		SMSBaoKey:          getenv("VG_SMSBAO_KEY"),
		SMSBaoBaseURL:      getenv("VG_SMSBAO_BASE_URL"),
		TokenKeys:          getenv("VG_TOKEN_KEYS"),
		PhoneHashKey:       getenv("VG_PHONE_HASH_KEY"),
		RedisAddr:          getenv("VG_REDIS_ADDR"),
		RedisPassword:      getenv("VG_REDIS_PASSWORD"),
		IMTokenKey:         getenv("VG_IM_TOKEN_KEY"),
		ManagerAddr:        orDefault(getenv("VG_MANAGER_ADDR"), "127.0.0.1:9103"),
		ManagerAllow:       splitList(getenv("VG_MANAGER_ALLOW")),
		AdminTOTPKey:       getenv("VG_ADMIN_TOTP_KEY"),
		TrustedProxies:     splitList(getenv("VG_TRUSTED_PROXIES")),
		CORSAllowedOrigins: splitList(getenv("VG_CORS_ALLOWED_ORIGINS")),
	}
	switch e := Env(strings.ToLower(strings.TrimSpace(getenv("VG_ENV")))); e {
	case "":
		c.Env = Production
	case Development, Test, Staging, Production:
		c.Env = e
	default:
		c.Env = Production
		vs = append(vs, Violation{"VG_ENV", fmt.Sprintf("unknown environment %q (use development, test, staging or production)", e)})
	}
	switch strings.ToLower(strings.TrimSpace(getenv("VG_DEBUG"))) {
	case "", "0", "false":
	case "1", "true":
		c.Debug = true
	default:
		vs = append(vs, Violation{"VG_DEBUG", "must be true or false"})
	}
	c.MessageRateLimit, c.MessageRateWindow, c.ManagerPublicRate = 30, 10, 20
	for _, e := range []struct {
		name string
		dst  *int
		min  int
	}{{"VG_MESSAGE_RATE_LIMIT", &c.MessageRateLimit, 0}, {"VG_MESSAGE_RATE_WINDOW_SECONDS", &c.MessageRateWindow, 1}, {"VG_MANAGER_PUBLIC_RATE", &c.ManagerPublicRate, 1}} {
		if v := strings.TrimSpace(getenv(e.name)); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < e.min || n > 100000 {
				vs = append(vs, Violation{e.name, fmt.Sprintf("must be an integer between %d and 100000", e.min)})
			} else {
				*e.dst = n
			}
		}
	}
	switch strings.ToLower(strings.TrimSpace(getenv("VG_MANAGER_ENABLED"))) {
	case "", "0", "false":
	case "1", "true":
		c.ManagerEnabled = true
	default:
		vs = append(vs, Violation{"VG_MANAGER_ENABLED", "must be true or false"})
	}
	switch strings.ToLower(strings.TrimSpace(getenv("VG_AUTH_SMS_LOGIN"))) {
	case "", "0", "false":
	case "1", "true":
		c.AuthSMSLogin = true
	default:
		vs = append(vs, Violation{"VG_AUTH_SMS_LOGIN", "must be true or false"})
	}
	return c, append(vs, c.Validate()...)
}

// Strict reports whether the insecure-default checks apply (staging and production).
func (c Config) Strict() bool { return c.Env == Production || c.Env == Staging }

// Validate returns every insecure or malformed setting. In strict environments weak values are rejected;
// in development/test only structural problems are reported.
func (c Config) Validate() []Violation {
	var vs []Violation
	add := func(k, r string) { vs = append(vs, Violation{k, r}) }
	if _, err := splitHostPort(c.HTTPAddr); err != nil {
		add("VG_HTTP_ADDR", err.Error())
	}
	if _, err := splitHostPort(c.WorkerInternalAddr); err != nil {
		add("VG_WORKER_INTERNAL_ADDR", "must be host:port")
	}
	if _, err := splitHostPort(c.InternalAddr); err != nil {
		add("VG_INTERNAL_ADDR", err.Error())
	}
	if _, err := splitHostPort(c.IMWebhookAddr); err != nil {
		add("VG_IM_WEBHOOK_ADDR", err.Error())
	}
	if c.ManagerEnabled {
		if _, err := splitHostPort(c.ManagerAddr); err != nil {
			add("VG_MANAGER_ADDR", err.Error())
		} else if c.Strict() && !isInternalHost(c.ManagerAddr) {
			add("VG_MANAGER_ADDR", "the admin listener must bind loopback or a private address; publish it through Nginx on its own hostname (AD-24)")
		}
		if c.MySQLDSN == "" {
			add("VG_MANAGER_ENABLED", "the admin plane needs VG_MYSQL_DSN")
		}
		if k := c.AdminTOTPKey; len(k) < 32 || weakValues[strings.ToLower(k)] {
			add("VG_ADMIN_TOTP_KEY", "must be set to at least 32 random characters when the admin plane is enabled")
		}
		if _, err := clientip.ParseTrusted(c.ManagerAllow); err != nil {
			add("VG_MANAGER_ALLOW", err.Error())
		}
		if c.ManagerAddr == c.HTTPAddr || c.ManagerAddr == c.InternalAddr || c.ManagerAddr == c.IMWebhookAddr {
			add("VG_MANAGER_ADDR", "the admin plane must have its own listener, not share an address with another one")
		}
	}
	if u, err := url.Parse(c.IMAPIURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		add("VG_IM_API_URL", "must be an absolute http(s) URL")
	}
	if _, err := clientip.ParseTrusted(c.TrustedProxies); err != nil {
		add("VG_TRUSTED_PROXIES", err.Error())
	}
	if c.RedisAddr != "" {
		if _, err := splitHostPort(c.RedisAddr); err != nil {
			add("VG_REDIS_ADDR", err.Error())
		} else if c.Strict() && !isInternalHost(c.RedisAddr) {
			add("VG_REDIS_ADDR", "Redis must be loopback or a private address, never public")
		}
	}
	if c.AuthSMSLogin {
		if len(c.IMTokenKey) < 32 || weakValues[strings.ToLower(c.IMTokenKey)] || c.IMTokenKey == c.PhoneHashKey {
			add("VG_IM_TOKEN_KEY", "auth.sms_login needs a key of at least 32 bytes, not a known default and different from VG_PHONE_HASH_KEY")
		}
		if c.Strict() && c.RedisAddr == "" {
			add("VG_REDIS_ADDR", "auth.sms_login needs the Redis token deny-list in "+string(c.Env))
		}
		if c.MySQLDSN == "" {
			add("VG_MYSQL_DSN", "auth.sms_login needs the database")
		}
		if _, err := token.ParseRing(c.TokenKeys); err != nil {
			add("VG_TOKEN_KEYS", err.Error())
		}
		if len(c.PhoneHashKey) < 32 || weakValues[strings.ToLower(c.PhoneHashKey)] {
			add("VG_PHONE_HASH_KEY", "auth.sms_login needs a key of at least 32 bytes that is not a known default")
		}
		switch c.SMSProvider {
		case "http":
			u, err := url.Parse(c.SMSHTTPURL)
			if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && !c.Strict())) {
				add("VG_SMS_HTTP_URL", "must be an absolute https URL (http only in development/test)")
			}
		case "smsbao":
			if c.SMSBaoUser == "" || c.SMSBaoKey == "" {
				add("VG_SMSBAO_KEY", "the smsbao provider needs VG_SMSBAO_USER and VG_SMSBAO_KEY")
			}
			if u := c.SMSBaoBaseURL; u != "" && c.Strict() && !strings.HasPrefix(strings.ToLower(u), "https://") {
				add("VG_SMSBAO_BASE_URL", "must be https in "+string(c.Env))
			}
		case "log":
			if c.Strict() {
				add("VG_SMS_PROVIDER", "the log provider prints verification codes and must not be used in "+string(c.Env))
			}
		case "":
			add("VG_SMS_PROVIDER", "auth.sms_login needs an SMS provider (smsbao or http)")
		default:
			add("VG_SMS_PROVIDER", "unknown provider (use smsbao or http; log is for development/test)")
		}
	}
	if !c.Strict() {
		return vs
	}
	for _, p := range c.TrustedProxies {
		if p == "0.0.0.0/0" || p == "::/0" {
			add("VG_TRUSTED_PROXIES", "trusting every address defeats client-IP resolution (AD-16); list the proxy addresses")
		}
	}
	if u := c.AuditAnchorURL; u != "" && !strings.HasPrefix(strings.ToLower(u), "https://") {
		add("VG_AUDIT_ANCHOR_URL", "the audit anchor store must be reached over https")
	}
	if s := c.IMWebhookSecret; s != "" && (len(s) < 32 || weakValues[strings.ToLower(s)] || strings.ContainsAny(s, "/?#% ")) {
		add("VG_IM_WEBHOOK_SECRET", "when set must be at least 32 bytes of URL-safe characters and not a known default")
	}
	if s := c.IMWebhookSigning; s != "" && (len(s) < 32 || weakValues[strings.ToLower(s)] || s == c.IMWebhookSecret) {
		add("VG_IM_WEBHOOK_SIGNING_SECRET", "when set must be at least 32 bytes, not a known default, and different from VG_IM_WEBHOOK_SECRET")
	}
	if u, err := url.Parse(c.IMAPIURL); err == nil && !isInternalHost(u.Host) {
		add("VG_IM_API_URL", "the IM API must be loopback or a private address, never public (AD-2)")
	}
	for _, pu := range c.IMAPIPeerURLs {
		if u, err := url.Parse(pu); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			add("VG_IM_API_PEER_URLS", "every entry must be an absolute http(s) URL")
		} else if !isInternalHost(u.Host) {
			add("VG_IM_API_PEER_URLS", "IM node APIs must be loopback or private addresses, never public (AD-2)")
		}
	}
	if !isInternalHost(c.IMWebhookAddr) {
		add("VG_IM_WEBHOOK_ADDR", "the IM callback listener must bind loopback or a private address, never a public or wildcard one")
	}
	if t := c.MetricsToken; t != "" && (len(t) < 24 || weakValues[strings.ToLower(t)]) {
		add("VG_METRICS_TOKEN", "when set must be at least 24 bytes and not a known default")
	}
	if c.SMSFixedCode != "" {
		add("VG_SMS_FIXED_CODE", "a fixed SMS verification code must not be configured in "+string(c.Env))
	}
	if c.AdminPassword != "" {
		add("VG_ADMIN_PASSWORD", "no admin password may be configured; create the first admin with the install wizard")
	}
	if key := c.TokenSigningKey; len(key) < 32 || weakValues[strings.ToLower(key)] {
		add("VG_TOKEN_SIGNING_KEY", "must be at least 32 bytes and not a known default")
	}
	for _, o := range c.CORSAllowedOrigins {
		if o == "*" {
			add("VG_CORS_ALLOWED_ORIGINS", "wildcard \"*\" is not allowed; list exact origins")
			break
		}
	}
	if c.Debug {
		add("VG_DEBUG", "debug mode must be off in "+string(c.Env))
	}
	if u := dsnUser(c.MySQLDSN); strings.EqualFold(u, "root") {
		add("VG_MYSQL_DSN", "the database user must not be root; use a least-privilege application account")
	}
	if !isInternalHost(c.WorkerInternalAddr) {
		add("VG_WORKER_INTERNAL_ADDR", "the worker metrics listener must bind loopback or a private address")
	}
	if !isInternalHost(c.InternalAddr) {
		add("VG_INTERNAL_ADDR", "health/metrics listener must bind loopback or a private address, never a public or wildcard one")
	}
	return vs
}

// Error formats violations for the startup log, one per line, sorted for stable output.
func Error(vs []Violation) error {
	if len(vs) == 0 {
		return nil
	}
	lines := make([]string, len(vs))
	for i, v := range vs {
		lines[i] = "  - " + v.String()
	}
	sort.Strings(lines)
	return errors.New("refusing to start, invalid configuration:\n" + strings.Join(lines, "\n"))
}

// FromEnv is Load over the process environment.
func FromEnv() (Config, []Violation) { return Load(os.Getenv) }

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitHostPort(addr string) (host string, err error) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("must be host:port: %v", err)
	}
	if p == "" {
		return "", errors.New("port is required")
	}
	return h, nil
}

// isInternalHost is true for loopback and private-range literals and for "localhost".
func isInternalHost(addr string) bool {
	h, err := splitHostPort(addr)
	if err != nil || h == "" {
		return false // an empty host means all interfaces
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && !ip.IsUnspecified() && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

// dsnUser extracts the user from a MySQL DSN of the form user:pass@tcp(host)/db.
func dsnUser(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at <= 0 {
		return ""
	}
	cred := dsn[:at]
	if i := strings.Index(cred, ":"); i >= 0 {
		cred = cred[:i]
	}
	return cred
}
