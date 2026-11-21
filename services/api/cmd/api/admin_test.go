package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	adminadapters "veyrogram/api/internal/admin/adapters"
	"veyrogram/api/internal/admin/domain"
)

const adminTOTPKey = "admin-totp-encryption-key-0123456789abcdef"

func adminEnv(dsn string, extra ...string) []string {
	return append([]string{"VG_MYSQL_DSN=" + dsn, "VG_ADMIN_TOTP_KEY=" + adminTOTPKey, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef"}, extra...)
}

type mgr struct {
	t    *testing.T
	base string
}

func (m mgr) call(method, path, token, body string) (int, map[string]any) {
	m.t.Helper()
	req, _ := http.NewRequest(method, m.base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		m.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, out
}

// signIn plays the person with an authenticator app: password, then the code. enroll=true is the first sign-in.
func (m mgr) signIn(user, pw string, secret []byte, at time.Time) (string, int, map[string]any) {
	m.t.Helper()
	c, r := m.call("POST", "/v1/manager/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, user, pw))
	if c != 200 {
		return "", c, r
	}
	path := "/v1/manager/mfa/verify"
	if r["state"] == "enrollment_required" {
		path = "/v1/manager/mfa/enroll"
	}
	c, s := m.call("POST", path, "", fmt.Sprintf(`{"token":%q,"code":%q}`, r["token"], domain.TOTPCode(secret, at)))
	if c != 200 {
		return "", c, s
	}
	return s["session_token"].(string), c, s
}

func actions(m mgr, tok string) []string {
	_, a := m.call("GET", "/v1/manager/audit?limit=200", tok, "")
	var out []string
	for _, x := range a["items"].([]any) {
		out = append(out, x.(map[string]any)["action"].(string))
	}
	return out
}

func has(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// Story 1.19 end to end: CLI bootstrap -> password+TOTP sign-in -> role-checked admin API -> reasoned, audited high-risk actions -> audit chain check.
func TestAdminPlaneEndToEndAgainstRealMySQL(t *testing.T) {
	d, dsn, bin := migrated(t)
	env := adminEnv(dsn, "VG_ENV=development", "VG_MANAGER_PUBLIC_RATE=1000") // the per-address login limit has its own test; this flow makes many sign-ins from one address
	run := func(extraEnv []string, args ...string) (string, string, int) {
		return runBin(t, bin, baseEnv(append(env, extraEnv...)...), args...)
	}

	// 1. weak, default and empty passwords are refused by the first-admin command, and nothing is created
	for name, pw := range map[string]string{"empty": "", "default": "admin", "common": "Password123!", "short": "Ab1!xyz", "one class": "abcdefghijklmnopqrstu"} {
		_, se, code := run([]string{"VG_ADMIN_BOOTSTRAP_PASSWORD=" + pw}, "admin", "bootstrap", "-username", "owner")
		if code == 0 {
			t.Errorf("%s password must be refused: %s", name, se)
		}
	}
	var n int
	_ = d.QueryRow("SELECT COUNT(*) FROM admin_user").Scan(&n)
	if n != 0 {
		t.Fatalf("a refused bootstrap must create nothing: %d admins", n)
	}
	if _, se, code := run([]string{"VG_ADMIN_BOOTSTRAP_PASSWORD=correct-Horse-battery-7"}, "admin", "bootstrap", "-username", "a b"); code == 0 {
		t.Errorf("bad username: %s", se)
	}

	// 2. the real bootstrap prints the TOTP secret once
	const ownerPW = "correct-Horse-battery-7"
	so, se, code := run([]string{"VG_ADMIN_BOOTSTRAP_PASSWORD=" + ownerPW}, "admin", "bootstrap", "-username", "owner")
	if code != 0 {
		t.Fatalf("bootstrap: %d %s", code, se)
	}
	mm := regexp.MustCompile(`secret:\s+(\S+)`).FindStringSubmatch(so)
	if mm == nil || strings.Contains(so, ownerPW) {
		t.Fatalf("the secret must be printed, the password never: %q", so)
	}
	ownerSecret, err := domain.ParseTOTPSecret(mm[1])
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	_ = d.QueryRow("SELECT totp_secret_enc FROM admin_user WHERE username='owner'").Scan(&stored)
	if bytes.Contains(stored, ownerSecret) {
		t.Fatal("the TOTP secret must be encrypted at rest")
	}
	var hash string
	_ = d.QueryRow("SELECT password_hash FROM admin_user WHERE username='owner'").Scan(&hash)
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, ownerPW) {
		t.Fatalf("passwords are stored as Argon2id: %q", hash)
	}

	// 3. initialisation works once, and the lock FILE is irrelevant in both directions
	lock := filepath.Join(t.TempDir(), "install.lock")
	_ = os.WriteFile(lock, []byte("x"), 0o600)
	_ = os.Remove(lock) // an operator deletes install.lock to "start over"
	_, se, code = run([]string{"VG_ADMIN_BOOTSTRAP_PASSWORD=another-Strong-pass-9"}, "admin", "bootstrap", "-username", "intruder")
	if code == 0 || !strings.Contains(se, "already") {
		t.Fatalf("a second initialisation must be refused even with install.lock gone: %d %s", code, se)
	}
	_ = d.QueryRow("SELECT COUNT(*) FROM admin_user").Scan(&n)
	if n != 1 {
		t.Fatalf("still exactly one admin: %d", n)
	}

	// 4. the admin plane runs on its own listener
	addr := freeAddr(t)
	p := start(t, append(env, "VG_MANAGER_ENABLED=true", "VG_MANAGER_ADDR="+addr)...)
	m := mgr{t, "http://" + addr}
	if c, _ := get(t, "http://"+p.pub+"/v1/manager/me", ""); c == 200 || c == 401 {
		t.Fatalf("the user API listener must not serve admin routes: %d", c)
	}
	if c, _ := m.call("GET", "/v1/auth/login", "", ""); c != 404 {
		t.Fatalf("the admin listener must not serve user routes: %d", c)
	}

	// 5. credentials: wrong password and unknown user look the same
	c1, w1 := m.call("POST", "/v1/manager/login", "", `{"username":"owner","password":"wrong-Password-123"}`)
	c2, w2 := m.call("POST", "/v1/manager/login", "", `{"username":"nobody","password":"wrong-Password-123"}`)
	if c1 != 401 || c2 != 401 || w1["code"] != w2["code"] || w1["message"] != w2["message"] {
		t.Fatalf("no user enumeration: %d %v / %d %v", c1, w1, c2, w2)
	}

	// 6. password alone gets nothing; the first sign-in must prove the authenticator app holds the secret
	c, lr := m.call("POST", "/v1/manager/login", "", fmt.Sprintf(`{"username":"owner","password":%q}`, ownerPW))
	if c != 200 || lr["state"] != "enrollment_required" {
		t.Fatalf("first login must demand enrolment: %d %v", c, lr)
	}
	for _, path := range []string{"/v1/manager/me", "/v1/manager/audit", "/v1/manager/admins"} {
		if c, _ := m.call("GET", path, lr["token"].(string), ""); c != 401 {
			t.Fatalf("a password-only login token must open nothing (%s): %d", path, c)
		}
	}
	if c, r := m.call("POST", "/v1/manager/mfa/enroll", "", fmt.Sprintf(`{"token":%q,"code":"000000"}`, lr["token"])); c != 401 {
		t.Fatalf("a wrong code must not enrol: %d %v", c, r)
	}
	if c, _ := m.call("POST", "/v1/manager/mfa/enroll", "", fmt.Sprintf(`{"token":%q,"code":%q}`, lr["token"], domain.TOTPCode(ownerSecret, time.Now()))); c != 401 {
		t.Fatalf("a challenge is single use: after one attempt it is dead: %d", c)
	}
	now := time.Now()
	tok1, c, _ := m.signIn("owner", ownerPW, ownerSecret, now)
	if c != 200 {
		t.Fatalf("enrol and sign in: %d", c)
	}
	if c, me := m.call("GET", "/v1/manager/me", tok1, ""); c != 200 || me["role"] != "owner" {
		t.Fatalf("me: %d %v", c, me)
	}

	// 7. replay: the code that enrolled cannot be used again; the next time step can
	if _, c, _ = m.signIn("owner", ownerPW, ownerSecret, now); c != 401 {
		t.Fatalf("a TOTP code must not be reusable: %d", c)
	}
	tok2, c, _ := m.signIn("owner", ownerPW, ownerSecret, now.Add(30*time.Second))
	if c != 200 {
		t.Fatalf("the next step's code is accepted: %d", c)
	}

	// 8. audit: read-only view, chain verification
	a := actions(m, tok2)
	for _, want := range []string{"admin.bootstrap", "admin.mfa.enrolled", "admin.login", "admin.login.failed", "admin.mfa.failed"} {
		if !has(a, want) {
			t.Errorf("audit must contain %s: %v", want, a)
		}
	}
	if c, v := m.call("GET", "/v1/manager/audit/verify", tok2, ""); c != 200 || v["ok"] != true || v["rows"].(float64) < 5 {
		t.Fatalf("verify: %d %v", c, v)
	}
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		if c, _ := m.call(method, "/v1/manager/audit", tok2, "{}"); c == 200 || c == 204 {
			t.Errorf("the audit view is read-only: %s answered %d", method, c)
		}
	}

	// 9. high-risk actions need a reason and a strong password; both are audited
	mk := func(user, role, pw, reason string) (int, map[string]any) {
		return m.call("POST", "/v1/manager/admins", tok2, fmt.Sprintf(`{"username":%q,"role":%q,"password":%q,"reason":%q}`, user, role, pw, reason))
	}
	if c, r := mk("auditor1", "auditor", "Zq9!mW3#pL8@vR2$", ""); c != 422 || r["code"] != "reason_required" {
		t.Fatalf("no reason: %d %v", c, r)
	}
	if c, r := mk("auditor1", "auditor", "short", "quarterly audit access"); c != 422 || r["code"] != "weak_password" || r["details"] == nil {
		t.Fatalf("weak password: %d %v", c, r)
	}
	if c, _ := mk("auditor1", "superuser", "Zq9!mW3#pL8@vR2$", "quarterly audit access"); c != 400 {
		t.Fatalf("unknown role: %d", c)
	}
	c, cr := mk("auditor1", "auditor", "Zq9!mW3#pL8@vR2$", "quarterly audit access")
	if c != 201 || cr["totp_secret"] == nil {
		t.Fatalf("create admin: %d %v", c, cr)
	}
	if c, _ := mk("auditor1", "auditor", "Zq9!mW3#pL8@vR2$", "duplicate attempt here"); c != 409 {
		t.Fatalf("duplicate username: %d", c)
	}
	audSecret, _ := domain.ParseTOTPSecret(cr["totp_secret"].(string))
	_, rows := m.call("GET", "/v1/manager/audit?action=admin.create", tok2, "")
	first := rows["items"].([]any)[len(rows["items"].([]any))-1].(map[string]any)
	if !strings.Contains(first["actor"].(string), "owner") || first["target"] != "auditor1" || !strings.Contains(first["detail"].(string), "quarterly audit access") || first["created_at"] == "" {
		t.Fatalf("the audit row must record who, what, why and when: %v", first)
	}

	// 10. role separation: the auditor reads the audit but cannot manage admins; their TOTP is mandatory too
	if _, c, _ := m.signIn("auditor1", "Zq9!mW3#pL8@vR2$", audSecret, time.Now().Add(-10*time.Minute)); c != 401 {
		t.Fatalf("a code from a stale time step must not enrol: %d", c)
	}
	audTok, c, _ := m.signIn("auditor1", "Zq9!mW3#pL8@vR2$", audSecret, time.Now())
	if c != 200 {
		t.Fatalf("auditor sign-in: %d", c)
	}
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"GET", "/v1/manager/audit", 200}, {"GET", "/v1/manager/audit/verify", 200}, {"GET", "/v1/manager/me", 200},
		{"GET", "/v1/manager/admins", 403}, {"POST", "/v1/manager/admins", 403}, {"POST", "/v1/manager/admins/1/disable", 403}} {
		if c, _ := m.call(tc.method, tc.path, audTok, `{}`); c != tc.want {
			t.Errorf("auditor %s %s: %d, want %d", tc.method, tc.path, c, tc.want)
		}
	}

	// 11. disabling an admin (reason required, never yourself) kills their sessions at once
	var audID, ownerID float64
	_, lst := m.call("GET", "/v1/manager/admins", tok2, "")
	for _, x := range lst["items"].([]any) {
		it := x.(map[string]any)
		if it["username"] == "auditor1" {
			audID = it["id"].(float64)
		}
		if it["username"] == "owner" {
			ownerID = it["id"].(float64)
		}
	}
	if c, _ := m.call("POST", fmt.Sprintf("/v1/manager/admins/%d/disable", int(ownerID)), tok2, `{"reason":"disabling myself for fun"}`); c != 400 {
		t.Fatalf("self-disable must be refused: %d", c)
	}
	if c, r := m.call("POST", fmt.Sprintf("/v1/manager/admins/%d/disable", int(audID)), tok2, `{"reason":""}`); c != 422 {
		t.Fatalf("disable without a reason: %d %v", c, r)
	}
	if c, _ := m.call("POST", fmt.Sprintf("/v1/manager/admins/%d/disable", int(audID)), tok2, `{"reason":"left the company, ticket 4711"}`); c != 204 {
		t.Fatalf("disable: %d", c)
	}
	if c, _ := m.call("GET", "/v1/manager/audit", audTok, ""); c != 401 {
		t.Fatalf("a disabled admin's session must die immediately: %d", c)
	}
	if _, c, _ := m.signIn("auditor1", "Zq9!mW3#pL8@vR2$", audSecret, time.Now().Add(30*time.Second)); c != 401 {
		t.Fatalf("a disabled admin cannot sign in: %d", c)
	}
	if a := actions(m, tok2); !has(a, "admin.disable") {
		t.Fatalf("disable must be audited: %v", a)
	}

	// 12. lockout: five wrong passwords lock the account, even for the right password afterwards
	if c, _ := mk("locktest", "auditor", "Zq9!mW3#pL8@vR2$", "lockout test account"); c != 201 {
		t.Fatalf("create: %d", c)
	}
	for i := 0; i < 5; i++ {
		m.call("POST", "/v1/manager/login", "", `{"username":"locktest","password":"wrong-Password-123"}`)
	}
	if c, r := m.call("POST", "/v1/manager/login", "", `{"username":"locktest","password":"Zq9!mW3#pL8@vR2$"}`); c != 429 || r["code"] != "locked" {
		t.Fatalf("a locked account refuses even the right password: %d %v", c, r)
	}
	if a := actions(m, tok2); !has(a, "admin.locked") {
		t.Fatalf("lockout must be audited: %v", a)
	}

	// 13. logout ends the session
	if c, _ := m.call("POST", "/v1/manager/logout", tok1, ""); c != 204 {
		t.Fatalf("logout: %d", c)
	}
	if c, _ := m.call("GET", "/v1/manager/me", tok1, ""); c != 401 {
		t.Fatalf("a logged-out session is dead: %d", c)
	}

	// 14. tampering with the audit log is detected by the chain check
	if _, err := d.Exec("UPDATE audit_log SET action='admin.nothing-happened' WHERE action='admin.create' LIMIT 1"); err != nil {
		t.Fatal(err)
	}
	if c, v := m.call("GET", "/v1/manager/audit/verify", tok2, ""); c != 200 || v["ok"] != false {
		t.Fatalf("a rewritten audit row must break verification: %d %v", c, v)
	}

	// 15. refusals are counted for alerting
	_, mt := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	if !strings.Contains(mt, `vg_admin_denied_total{reason="unauthenticated"}`) || !strings.Contains(mt, `vg_admin_denied_total{reason="forbidden"}`) {
		t.Errorf("admin refusals must be exported:\n%s", mt)
	}
}

// The optional IP allowlist (AD-24) refuses everything from outside it, including the login endpoints.
func TestAdminPlaneIPAllowlist(t *testing.T) {
	_, dsn, _ := migrated(t)
	addr := freeAddr(t)
	start(t, adminEnv(dsn, "VG_MANAGER_ENABLED=true", "VG_MANAGER_ADDR="+addr, "VG_MANAGER_ALLOW=10.0.0.0/8")...)
	m := mgr{t, "http://" + addr}
	for _, path := range []string{"/v1/manager/login", "/v1/manager/me"} {
		method := "GET"
		if strings.HasSuffix(path, "login") {
			method = "POST"
		}
		if c, _ := m.call(method, path, "", "{}"); c != 403 {
			t.Errorf("%s from 127.0.0.1 with an allowlist of 10.0.0.0/8: %d, want 403", path, c)
		}
	}
}

func TestAdminPlaneConfigGuards(t *testing.T) {
	bin := build(t)
	for name, env := range map[string][]string{
		"no encryption key":    {"VG_MANAGER_ENABLED=true", "VG_MYSQL_DSN=x:y@tcp(127.0.0.1:3306)/d"},
		"weak encryption key":  {"VG_MANAGER_ENABLED=true", "VG_MYSQL_DSN=x:y@tcp(127.0.0.1:3306)/d", "VG_ADMIN_TOTP_KEY=changeme"},
		"no database":          {"VG_MANAGER_ENABLED=true", "VG_ADMIN_TOTP_KEY=" + adminTOTPKey},
		"bad allowlist":        {"VG_MANAGER_ENABLED=true", "VG_MYSQL_DSN=x:y@tcp(127.0.0.1:3306)/d", "VG_ADMIN_TOTP_KEY=" + adminTOTPKey, "VG_MANAGER_ALLOW=nonsense"},
		"shares the user port": {"VG_MANAGER_ENABLED=true", "VG_MYSQL_DSN=x:y@tcp(127.0.0.1:3306)/d", "VG_ADMIN_TOTP_KEY=" + adminTOTPKey, "VG_HTTP_ADDR=127.0.0.1:9103", "VG_MANAGER_ADDR=127.0.0.1:9103"},
	} {
		_, se, code := runBin(t, bin, append([]string{"VG_ENV=development"}, env...), "-check-config")
		if code == 0 {
			t.Errorf("%s: config must be refused: %s", name, se)
		}
	}
}

// The last active owner can never be disabled (nobody could manage admins afterwards), even by another owner racing to disable each other.
func TestLastActiveOwnerCannotBeDisabled(t *testing.T) {
	d, _, _ := migrated(t)
	st := &adminadapters.MySQLStore{DB: d}
	ctx := context.Background()
	now := time.Now()
	a, err := st.Bootstrap(ctx, 1, "owner-a", "h", []byte("s"), now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.CreateAdmin(ctx, 1, "owner-b", "h", domain.RoleOwner, []byte("s"), a, now)
	if err != nil {
		t.Fatal(err)
	}
	aud, _ := st.CreateAdmin(ctx, 1, "aud", "h", domain.RoleAuditor, []byte("s"), a, now)
	if err := st.DisableAdmin(ctx, 1, aud, now); err != nil {
		t.Fatalf("disabling a non-owner is fine: %v", err)
	}
	// two owners disable each other at the same moment: exactly one may succeed
	errs := make(chan error, 2)
	go func() { errs <- st.DisableAdmin(ctx, 1, a, now) }()
	go func() { errs <- st.DisableAdmin(ctx, 1, b, now) }()
	e1, e2 := <-errs, <-errs
	loser := e1
	if e1 == nil {
		loser = e2
	}
	if (e1 == nil) == (e2 == nil) || !errors.Is(loser, domain.ErrLastOwner) {
		t.Fatalf("exactly one disable may win and the other must be ErrLastOwner: %v / %v", e1, e2)
	}
	var active int
	_ = d.QueryRow("SELECT COUNT(*) FROM admin_user WHERE role_id=1 AND status='active'").Scan(&active)
	if active != 1 {
		t.Fatalf("one owner must remain active: %d", active)
	}
	if _, err := st.Bootstrap(ctx, 1, "again", "h", []byte("s"), now); !errors.Is(err, domain.ErrAlreadyInitialized) {
		t.Fatalf("bootstrap is once-only at the store level too: %v", err)
	}
}
