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
	"strings"
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
	InternalAddr       string // /healthz, /readyz (and later /metrics): must be loopback/private
	Debug              bool
	SMSFixedCode       string   // upstream-style "test SMS code"; must be empty outside development/test
	AdminPassword      string   // there is no configured admin password: the install wizard creates the first admin
	TokenSigningKey    string   // symmetric key placeholder until asymmetric `kid` keys land (AD-9)
	CORSAllowedOrigins []string // exact origins; "*" is rejected outside development/test
	MetricsToken       string   // bearer token for /metrics on the internal listener; empty disables /metrics
	IMAPIURL           string   // WuKongIM API base URL; loopback/private only (AD-2)
	IMWebhookAddr      string   // listener for IM->business callbacks; loopback/private only, never routed by Nginx
	AuditAnchorURL     string   // off-host store for daily audit-log head anchors (https; object storage with versioning/WORM or a log host)
	AuditAnchorToken   string
	IMWebhookAllow     string // comma-separated CIDRs/IPs allowed to call the IM callback listener (default: loopback only)
	IMWebhookSecret    string // path secret for the callback URL (WuKongIM sends no signature); empty disables the callback listener
	MySQLDSN           string
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
		SMSFixedCode:       getenv("VG_SMS_FIXED_CODE"),
		AdminPassword:      getenv("VG_ADMIN_PASSWORD"),
		TokenSigningKey:    getenv("VG_TOKEN_SIGNING_KEY"),
		MySQLDSN:           getenv("VG_MYSQL_DSN"),
		IMAPIURL:           orDefault(getenv("VG_IM_API_URL"), "http://127.0.0.1:5001"),
		IMWebhookAddr:      orDefault(getenv("VG_IM_WEBHOOK_ADDR"), "127.0.0.1:9102"),
		IMWebhookSecret:    getenv("VG_IM_WEBHOOK_SECRET"),
		IMWebhookAllow:     getenv("VG_IM_WEBHOOK_ALLOW"),
		AuditAnchorURL:     getenv("VG_AUDIT_ANCHOR_URL"),
		AuditAnchorToken:   getenv("VG_AUDIT_ANCHOR_TOKEN"),
		MetricsToken:       getenv("VG_METRICS_TOKEN"),
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
	if _, err := splitHostPort(c.InternalAddr); err != nil {
		add("VG_INTERNAL_ADDR", err.Error())
	}
	if _, err := splitHostPort(c.IMWebhookAddr); err != nil {
		add("VG_IM_WEBHOOK_ADDR", err.Error())
	}
	if u, err := url.Parse(c.IMAPIURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		add("VG_IM_API_URL", "must be an absolute http(s) URL")
	}
	if !c.Strict() {
		return vs
	}
	if u := c.AuditAnchorURL; u != "" && !strings.HasPrefix(strings.ToLower(u), "https://") {
		add("VG_AUDIT_ANCHOR_URL", "the audit anchor store must be reached over https")
	}
	if s := c.IMWebhookSecret; s != "" && (len(s) < 32 || weakValues[strings.ToLower(s)] || strings.ContainsAny(s, "/?#% ")) {
		add("VG_IM_WEBHOOK_SECRET", "when set must be at least 32 bytes of URL-safe characters and not a known default")
	}
	if u, err := url.Parse(c.IMAPIURL); err == nil && !isInternalHost(u.Host) {
		add("VG_IM_API_URL", "the IM API must be loopback or a private address, never public (AD-2)")
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
