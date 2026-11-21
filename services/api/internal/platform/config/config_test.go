package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func good() map[string]string {
	return map[string]string{
		"VG_ENV":                  "production",
		"VG_TOKEN_SIGNING_KEY":    "0123456789abcdef0123456789abcdef-strong",
		"VG_MYSQL_DSN":            "vg_app:s3cret@tcp(127.0.0.1:3306)/veyrogram",
		"VG_CORS_ALLOWED_ORIGINS": "https://app.example.com, https://admin.example.com",
		"VG_INTERNAL_ADDR":        "127.0.0.1:9101",
	}
}

func keys(vs []Violation) string {
	var ks []string
	for _, v := range vs {
		ks = append(ks, v.Key)
	}
	return strings.Join(ks, ",")
}

func TestGoodProductionConfigIsAccepted(t *testing.T) {
	_, vs := Load(env(good()))
	if len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

// One case per unsafe value (AD-20, R-02/R-03/R-20): each must be rejected in production AND staging.
func TestInsecureValuesRejectedInStrictEnvironments(t *testing.T) {
	cases := []struct {
		name, key, val, wantKey string
	}{
		{"fixed SMS code", "VG_SMS_FIXED_CODE", "123456", "VG_SMS_FIXED_CODE"},
		{"configured admin password", "VG_ADMIN_PASSWORD", "admin1234567", "VG_ADMIN_PASSWORD"},
		{"short signing key", "VG_TOKEN_SIGNING_KEY", "short", "VG_TOKEN_SIGNING_KEY"},
		{"known default signing key", "VG_TOKEN_SIGNING_KEY", "wukongim-dev-manager-secret", "VG_TOKEN_SIGNING_KEY"},
		{"empty signing key", "VG_TOKEN_SIGNING_KEY", "", "VG_TOKEN_SIGNING_KEY"},
		{"wildcard CORS", "VG_CORS_ALLOWED_ORIGINS", "https://a.example.com,*", "VG_CORS_ALLOWED_ORIGINS"},
		{"debug mode", "VG_DEBUG", "true", "VG_DEBUG"},
		{"root database user", "VG_MYSQL_DSN", "root:demo@tcp(127.0.0.1:3306)/test", "VG_MYSQL_DSN"},
		{"public health listener", "VG_INTERNAL_ADDR", "0.0.0.0:9101", "VG_INTERNAL_ADDR"},
		{"empty-host health listener", "VG_INTERNAL_ADDR", ":9101", "VG_INTERNAL_ADDR"},
		{"routable public IP health listener", "VG_INTERNAL_ADDR", "203.0.113.7:9101", "VG_INTERNAL_ADDR"},
	}
	for _, e := range []string{"production", "staging"} {
		for _, tc := range cases {
			t.Run(e+"/"+tc.name, func(t *testing.T) {
				m := good()
				m["VG_ENV"] = e
				m[tc.key] = tc.val
				_, vs := Load(env(m))
				if !strings.Contains(keys(vs), tc.wantKey) {
					t.Fatalf("expected a violation for %s, got %q", tc.wantKey, keys(vs))
				}
			})
		}
	}
}

func TestUnsetEnvFailsClosedAsProduction(t *testing.T) {
	m := good()
	delete(m, "VG_ENV")
	m["VG_SMS_FIXED_CODE"] = "123456"
	c, vs := Load(env(m))
	if c.Env != Production || !strings.Contains(keys(vs), "VG_SMS_FIXED_CODE") {
		t.Fatalf("unset VG_ENV must behave as production, got env=%s violations=%q", c.Env, keys(vs))
	}
}

func TestUnknownEnvIsRejectedNotTreatedAsDev(t *testing.T) {
	m := good()
	m["VG_ENV"] = "prod" // a typo must not silently relax checks
	c, vs := Load(env(m))
	if c.Env != Production || !strings.Contains(keys(vs), "VG_ENV") {
		t.Fatalf("unknown env must be a violation and stay strict, got env=%s %q", c.Env, keys(vs))
	}
}

func TestDevelopmentAllowsLocalConvenienceButNotMalformedAddresses(t *testing.T) {
	m := map[string]string{"VG_ENV": "development", "VG_SMS_FIXED_CODE": "123456", "VG_DEBUG": "true", "VG_MYSQL_DSN": "root:demo@tcp(127.0.0.1)/test", "VG_INTERNAL_ADDR": "0.0.0.0:9101"}
	if _, vs := Load(env(m)); len(vs) != 0 {
		t.Fatalf("development should accept local conveniences, got %v", vs)
	}
	m["VG_HTTP_ADDR"] = "not-an-address"
	if _, vs := Load(env(m)); !strings.Contains(keys(vs), "VG_HTTP_ADDR") {
		t.Fatalf("malformed address must always be rejected, got %q", keys(vs))
	}
}

func TestErrorListsEveryViolationSorted(t *testing.T) {
	m := good()
	m["VG_SMS_FIXED_CODE"], m["VG_DEBUG"] = "123456", "true"
	_, vs := Load(env(m))
	err := Error(vs)
	if err == nil || !strings.Contains(err.Error(), "VG_DEBUG") || !strings.Contains(err.Error(), "VG_SMS_FIXED_CODE") {
		t.Fatalf("error must list both violations, got %v", err)
	}
	if Error(nil) != nil {
		t.Fatal("no violations must yield a nil error")
	}
}

func TestInvalidDebugValueRejected(t *testing.T) {
	m := good()
	m["VG_DEBUG"] = "maybe"
	if _, vs := Load(env(m)); !strings.Contains(keys(vs), "VG_DEBUG") {
		t.Fatalf("invalid VG_DEBUG must be rejected, got %q", keys(vs))
	}
}

func authOn() map[string]string {
	m := good()
	m["VG_AUTH_SMS_LOGIN"] = "true"
	m["VG_SMS_PROVIDER"] = "http"
	m["VG_SMS_HTTP_URL"] = "https://sms.example.com/send"
	m["VG_TOKEN_KEYS"] = "k1:AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA"
	m["VG_PHONE_HASH_KEY"] = "phone-hash-key-0123456789abcdef-strong"
	m["VG_TRUSTED_PROXIES"] = "127.0.0.1, 10.0.0.0/8"
	m["VG_IM_TOKEN_KEY"] = "im-token-key-0123456789abcdef-strong-x"
	m["VG_REDIS_ADDR"] = "127.0.0.1:6379"
	return m
}

func TestAuthSMSLoginRequiresSafeSettings(t *testing.T) {
	if c, vs := Load(env(authOn())); len(vs) != 0 || !c.AuthSMSLogin {
		t.Fatalf("good auth config rejected: %v", vs)
	}
	mut := func(k, v string) map[string]string { m := authOn(); m[k] = v; return m }
	for name, tc := range map[string]struct {
		m   map[string]string
		key string
	}{
		"log provider in production": {mut("VG_SMS_PROVIDER", "log"), "VG_SMS_PROVIDER"},
		"no provider":                {mut("VG_SMS_PROVIDER", ""), "VG_SMS_PROVIDER"},
		"plain http gateway":         {mut("VG_SMS_HTTP_URL", "http://sms.example.com/send"), "VG_SMS_HTTP_URL"},
		"no token keys":              {mut("VG_TOKEN_KEYS", ""), "VG_TOKEN_KEYS"},
		"trivial token seed":         {mut("VG_TOKEN_KEYS", "k1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), "VG_TOKEN_KEYS"},
		"short phone key":            {mut("VG_PHONE_HASH_KEY", "short"), "VG_PHONE_HASH_KEY"},
		"trust everyone":             {mut("VG_TRUSTED_PROXIES", "0.0.0.0/0"), "VG_TRUSTED_PROXIES"},
		"bad proxy entry":            {mut("VG_TRUSTED_PROXIES", "nonsense"), "VG_TRUSTED_PROXIES"},
		"no im key":                  {mut("VG_IM_TOKEN_KEY", ""), "VG_IM_TOKEN_KEY"},
		"im key equals phone key":    {mut("VG_IM_TOKEN_KEY", "phone-hash-key-0123456789abcdef-strong"), "VG_IM_TOKEN_KEY"},
		"no redis in production":     {mut("VG_REDIS_ADDR", ""), "VG_REDIS_ADDR"},
		"public redis":               {mut("VG_REDIS_ADDR", "203.0.113.5:6379"), "VG_REDIS_ADDR"},
		"bad flag":                   {mut("VG_AUTH_SMS_LOGIN", "maybe"), "VG_AUTH_SMS_LOGIN"},
	} {
		if _, vs := Load(env(tc.m)); !strings.Contains(keys(vs), tc.key) {
			t.Errorf("%s: expected violation on %s, got %v", name, tc.key, vs)
		}
	}
	sb := authOn()
	sb["VG_SMS_PROVIDER"] = "smsbao"
	if _, vs := Load(env(sb)); !strings.Contains(keys(vs), "VG_SMSBAO_KEY") {
		t.Errorf("smsbao without credentials must be rejected: %v", vs)
	}
	sb["VG_SMSBAO_USER"], sb["VG_SMSBAO_KEY"] = "tom", "apikey-0123456789"
	if _, vs := Load(env(sb)); len(vs) != 0 {
		t.Errorf("smsbao with credentials must be accepted: %v", vs)
	}
	sb["VG_SMSBAO_BASE_URL"] = "http://api.smsbao.com"
	if _, vs := Load(env(sb)); !strings.Contains(keys(vs), "VG_SMSBAO_BASE_URL") {
		t.Errorf("plain http base must be rejected in production: %v", vs)
	}
	dev := authOn()
	dev["VG_ENV"], dev["VG_SMS_PROVIDER"] = "development", "log"
	if _, vs := Load(env(dev)); len(vs) != 0 {
		t.Fatalf("log provider must be allowed in development: %v", vs)
	}
	off := good()
	if c, vs := Load(env(off)); len(vs) != 0 || c.AuthSMSLogin {
		t.Fatalf("auth must be off by default: %v", vs)
	}
}

func TestMessageRateLimitSettings(t *testing.T) {
	c, vs := Load(env(good()))
	if len(vs) != 0 || c.MessageRateLimit != 30 || c.MessageRateWindow != 10 {
		t.Fatalf("defaults 30 per 10 s: %+v %v", c, vs)
	}
	m := good()
	m["VG_MESSAGE_RATE_LIMIT"], m["VG_MESSAGE_RATE_WINDOW_SECONDS"] = "0", "60"
	if c, vs = Load(env(m)); len(vs) != 0 || c.MessageRateLimit != 0 || c.MessageRateWindow != 60 {
		t.Fatalf("0 disables, window is configurable: %+v %v", c, vs)
	}
	for k, v := range map[string]string{"VG_MESSAGE_RATE_LIMIT": "-1", "VG_MESSAGE_RATE_WINDOW_SECONDS": "0"} {
		m = good()
		m[k] = v
		if _, vs = Load(env(m)); !strings.Contains(keys(vs), k) {
			t.Errorf("%s=%s must be rejected: %s", k, v, keys(vs))
		}
	}
	m = good()
	m["VG_MESSAGE_RATE_LIMIT"] = "lots"
	if _, vs = Load(env(m)); !strings.Contains(keys(vs), "VG_MESSAGE_RATE_LIMIT") {
		t.Errorf("non-numeric must be rejected")
	}
}
