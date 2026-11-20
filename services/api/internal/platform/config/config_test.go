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
