package main

// Story 1.24: generated authorization gate.
//
// The routes are DISCOVERED from the source (every mux registration under internal/*/http), not listed by hand, so a route added tomorrow is
// covered tomorrow. Three tests:
//
//  1. TestEveryRouteIsAuthenticatedOrExplicitlyPublic: a route that is neither wrapped in authentication nor named in publicRoutes fails the build.
//  2. TestRoutesMatchTheOpenAPIContract: the code and packages/contracts/openapi agree on which operations exist and which need a bearer token.
//  3. TestGeneratedAuthorizationNegatives: every authenticated user route is hit with no token, a wrong scheme, garbage, a token signed by another key,
//     an expired token, a tampered one, and one for a revoked device; plus a token that is valid but belongs to another tenant. All must be refused.
//     The live part needs MySQL, Redis and the IM like the other end-to-end tests and skips itself without them.

import (
	"encoding/base64"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	authtoken "veyrogram/api/internal/platform/token"
)

type route struct {
	Method, Path string
	Authed       bool
	Admin        bool // served on the manager listener with its own session scheme
	Src          string
}

func (r route) key() string { return r.Method + " " + r.Path }

// publicRoutes are the only routes allowed to answer without a bearer token. Each says how it is protected instead.
var publicRoutes = map[string]string{
	"POST /v1/auth/sms/code":      "rate-limited one-time-code request; the answer is identical for known and unknown numbers",
	"POST /v1/auth/login":         "authenticated by the one-time code; lockouts and risk events apply",
	"POST /v1/auth/refresh":       "authenticated by the rotating refresh token, with reuse detection",
	"POST /v1/manager/login":      "first factor of the admin sign-in; returns a short-lived MFA challenge, never a session",
	"POST /v1/manager/mfa/verify": "second factor, bound to the challenge token",
	"POST /v1/manager/mfa/enroll": "first-time TOTP enrolment, bound to the challenge token",
}

// discoverRoutes parses the HTTP adapters of every module.
func discoverRoutes(t testing.TB) []route {
	t.Helper()
	files, _ := filepath.Glob("../../internal/*/http/*.go")
	var out []route
	fset := token.NewFileSet()
	str := func(e ast.Expr) (string, bool) {
		if l, ok := e.(*ast.BasicLit); ok && l.Kind == token.STRING {
			s, err := strconv.Unquote(l.Value)
			return s, err == nil
		}
		return "", false
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		admin := strings.Contains(filepath.ToSlash(f), "/admin/")
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			src := fmt.Sprintf("%s:%d", filepath.Base(f), fset.Position(call.Pos()).Line)
			switch {
			case admin && sel.Sel.Name == "Handle" && len(call.Args) == 4: // r.Handle("POST", "/v1/manager/login", policy, handler)
				m, ok1 := str(call.Args[0])
				p, ok2 := str(call.Args[1])
				if ok1 && ok2 {
					id, isID := call.Args[2].(*ast.Ident)
					out = append(out, route{Method: m, Path: p, Authed: !(isID && id.Name == "pub"), Admin: true, Src: src})
				}
			case !admin && (sel.Sel.Name == "Handle" || sel.Sel.Name == "HandleFunc") && len(call.Args) == 2: // mux.Handle("GET /v1/x", h.Auth(...))
				pat, ok := str(call.Args[0])
				parts := strings.SplitN(pat, " ", 2)
				if !ok || len(parts) != 2 { // catch-alls without a method are 404 handlers, not routes
					return true
				}
				authed := false
				if c, ok := call.Args[1].(*ast.CallExpr); ok {
					if s, ok := c.Fun.(*ast.SelectorExpr); ok && (s.Sel.Name == "Auth" || s.Sel.Name == "Require") {
						authed = true
					}
				}
				out = append(out, route{Method: parts[0], Path: parts[1], Authed: authed, Src: src})
			}
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

func TestEveryRouteIsAuthenticatedOrExplicitlyPublic(t *testing.T) {
	rs := discoverRoutes(t)
	if len(rs) < 20 {
		t.Fatalf("route discovery found only %d routes: the scanner no longer understands how routes are registered", len(rs))
	}
	seen := map[string]bool{}
	for _, r := range rs {
		seen[r.key()] = true
		_, public := publicRoutes[r.key()]
		switch {
		case !r.Authed && !public:
			t.Errorf("%s (%s) is not wrapped in authentication and is not in publicRoutes: add the wrapper, or justify it in publicRoutes", r.key(), r.Src)
		case r.Authed && public:
			t.Errorf("%s is listed as public but is authenticated: remove it from publicRoutes", r.key())
		}
		// Membership-scoped resources need a removed-member negative test before they can ship (no group routes exist yet).
		if r.Authed && regexp.MustCompile(`\{(gid|group|group_id|channel|channel_id)\}|/groups?/`).MatchString(r.Path) {
			t.Errorf("%s is scoped to a group or channel: add a removed-member negative to TestGeneratedAuthorizationNegatives before shipping it", r.key())
		}
	}
	for k := range publicRoutes {
		if !seen[k] {
			t.Errorf("publicRoutes lists %s but no such route is registered: remove the stale entry", k)
		}
	}
}

// openAPIOps reads (method path -> has security) from the contract files. The files are regular, so a line scan is enough and keeps the test free of a YAML dependency.
func openAPIOps(t testing.TB) map[string]bool {
	files, _ := filepath.Glob("../../../../packages/contracts/openapi/*.yaml")
	if len(files) == 0 {
		t.Skip("packages/contracts/openapi not found")
	}
	ops := map[string]bool{}
	pathRx, opRx := regexp.MustCompile(`^  (/\S+):\s*$`), regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
	for _, f := range files {
		b, _ := os.ReadFile(f)
		path, op := "", ""
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "components:") {
				path, op = "", ""
			}
			if m := pathRx.FindStringSubmatch(line); m != nil {
				path, op = m[1], ""
			} else if m := opRx.FindStringSubmatch(line); m != nil && path != "" {
				op = strings.ToUpper(m[1]) + " " + path
				ops[op] = false
			} else if op != "" && strings.HasPrefix(line, "      security:") && strings.Contains(line, "bearerAuth") {
				ops[op] = true
			}
		}
	}
	return ops
}

func TestRoutesMatchTheOpenAPIContract(t *testing.T) {
	ops := openAPIOps(t)
	rs := discoverRoutes(t)
	code := map[string]route{}
	for _, r := range rs {
		code[r.key()] = r
	}
	for k, secured := range ops {
		r, ok := code[k]
		if !ok {
			t.Errorf("the contract declares %s but no handler is registered for it", k)
			continue
		}
		if secured != r.Authed {
			t.Errorf("%s: the contract says bearer-protected=%v, the code says authenticated=%v", k, secured, r.Authed)
		}
	}
	for k, r := range code {
		if _, ok := ops[k]; !ok {
			t.Errorf("%s (%s) is registered but missing from packages/contracts/openapi: an undocumented route", k, r.Src)
		}
	}
}

// authzCase is one hostile way of presenting (or not presenting) a token.
type authzCase struct {
	name, header string
}

func forgedTokens(t *testing.T, uid, device string) []authzCase {
	t.Helper()
	real, err := authtoken.ParseRing("k1:" + authSeed)
	if err != nil {
		t.Fatal(err)
	}
	other, err := authtoken.ParseRing("k1:" + base64.RawURLEncoding.EncodeToString([]byte("another-signing-seed-0123456789!")))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	claims := func(exp time.Time) authtoken.Claims {
		return authtoken.Claims{Subject: uid, Device: device, Class: "app", Tenant: 1, ID: "jti-forged", IssuedAt: now.Add(-time.Hour).Unix(), Expires: exp.Unix(), Issuer: "veyrogram"}
	}
	sign := func(r *authtoken.Ring, c authtoken.Claims) string {
		s, err := r.Sign(c)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	good := sign(real, claims(now.Add(10*time.Minute)))
	parts := strings.Split(good, ".")
	tampered := good
	if len(parts) == 3 { // flip the subject inside a correctly shaped token; the signature no longer matches
		p, _ := base64.RawURLEncoding.DecodeString(parts[1])
		p = []byte(strings.Replace(string(p), uid, "u-someone-else", 1))
		tampered = parts[0] + "." + base64.RawURLEncoding.EncodeToString(p) + "." + parts[2]
	}
	return []authzCase{
		{"no Authorization header", ""},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"empty bearer", "Bearer "},
		{"garbage bearer", "Bearer not-a-token"},
		{"signed by another key", "Bearer " + sign(other, claims(now.Add(10*time.Minute)))},
		{"expired", "Bearer " + sign(real, claims(now.Add(-time.Minute)))},
		{"payload tampered after signing", "Bearer " + tampered},
	}
}

func fill(path, uid string) string {
	return regexp.MustCompile(`\{[^}]+\}`).ReplaceAllStringFunc(path, func(p string) string {
		if p == "{uid}" {
			return uid
		}
		return "1"
	})
}

func do(t *testing.T, method, url, auth, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return resp.StatusCode, sb.String()
}

func TestGeneratedAuthorizationNegatives(t *testing.T) {
	r := newRig(t)
	accA, uidA, tokA := r.user("+8613800138601", "device-aaaaaaaaaaaaaaaa")
	_, uidB, tokB := r.user("+8613800138602", "device-bbbbbbbbbbbbbbbb")
	_, uidC, _ := r.user("+8613800138603", "device-cccccccccccccccc")

	var userRoutes []route
	for _, rt := range discoverRoutes(t) {
		if rt.Authed && !rt.Admin {
			userRoutes = append(userRoutes, rt)
		}
	}
	if len(userRoutes) < 10 {
		t.Fatalf("only %d authenticated user routes discovered", len(userRoutes))
	}

	// 1. every authenticated route refuses every hostile token, before looking at the body or touching any state
	var device string
	if c, b := do(t, "GET", r.base+"/v1/devices", "Bearer "+accA, ""); c == 200 {
		if m := regexp.MustCompile(`"id"\s*:\s*"?(\d+)`).FindStringSubmatch(b); m != nil {
			device = m[1]
		}
	}
	if device == "" {
		device = "1"
	}
	cases := forgedTokens(t, uidA, device)
	checked := 0
	for _, rt := range userRoutes {
		for _, c := range cases {
			code, body := do(t, rt.Method, r.base+fill(rt.Path, uidB), c.header, `{"to_uid":"`+uidB+`","client_msg_no":"x","payload":{"type":1,"content":"x"}}`)
			checked++
			if code != 401 || !strings.Contains(body, "invalid_token") {
				t.Errorf("%s with %s: got %d %.80s, want 401 invalid_token", rt.key(), c.name, code, body)
			}
		}
	}

	// 2. cross-tenant: a token that is perfectly valid (right key, unexpired) but for tenant 2 must not see tenant 1's data, even naming the same uid.
	wsA, wsB := r.ws(uidA, tokA, "im-a"), r.ws(uidB, tokB, "im-b")
	_ = wsB
	if a := wsA.send(uidB, ulid(), text("tenant one secret")); a.Reason != 1 {
		t.Fatalf("seed message: %+v", a)
	}
	var mine int
	eventually(t, "message visible to its owner", 8*time.Second, func() bool {
		c, m := jget(t, r.base+"/v1/sync/messages?peer="+uidB+"&after_seq=0&limit=50", accA)
		if items, ok := m["messages"].([]any); c == 200 && ok {
			mine = len(items)
		}
		return mine > 0
	})
	real, _ := authtoken.ParseRing("k1:" + authSeed)
	foreign, _ := real.Sign(authtoken.Claims{Subject: uidA, Device: device, Class: "app", Tenant: 2, ID: "jti-t2", IssuedAt: time.Now().Add(-time.Minute).Unix(), Expires: time.Now().Add(10 * time.Minute).Unix(), Issuer: "veyrogram"})
	for _, path := range []string{"/v1/sync/messages?peer=" + uidB + "&after_seq=0&limit=50", "/v1/sync/conversations", "/v1/blocks", "/v1/devices"} {
		c, body := get(t, r.base+path, "Bearer "+foreign)
		if c == 200 && (strings.Contains(body, "tenant one secret") || strings.Contains(body, uidB)) {
			t.Errorf("cross-tenant read of %s returned tenant 1 data: %.120s", path, body)
		}
	}
	if c, _ := jsend(t, "PUT", r.base+"/v1/blocks/"+uidC, foreign, ""); c == 204 {
		var n int
		_ = r.d.QueryRow("SELECT COUNT(*) FROM user_block WHERE tenant_id=1 AND user_uid=? AND blocked_uid=?", uidA, uidC).Scan(&n)
		if n != 0 {
			t.Errorf("a tenant-2 token wrote into tenant 1's block list")
		}
	}

	// 3. non-owner: C addressing the A-B conversation by name sees only C's own (empty) conversation with the peer
	accC2, _, _ := r.user("+8613800138603", "device-cccccccccccccccc")
	if c, m := jget(t, r.base+"/v1/sync/messages?peer="+uidA+"&after_seq=0&limit=50", accC2); c == 200 {
		if items, _ := m["messages"].([]any); len(items) != 0 {
			t.Errorf("a third party read %d message(s) of someone else's conversation", len(items))
		}
	}

	// 4. a blocked sender is refused (covered in depth by TestBlockListThroughRealIMAndAPI); here only that the API path answers like a missing account
	accB := mustAccess(t, r, "+8613800138602", "device-bbbbbbbbbbbbbbbb")
	if c, _ := jsend(t, "PUT", r.base+"/v1/blocks/"+uidA, accB, ""); c != 204 {
		t.Fatalf("block: %d", c)
	}
	if c, _ := jpost(t, r.base+"/v1/messages", fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"x"}}`, uidB, ulid()), "Authorization", "Bearer "+accA); c != 404 {
		t.Errorf("a blocked sender must be refused like a missing account (404), got %d", c)
	}

	// 5. a revoked device's token dies at once, on every authenticated route (replay of a captured token after logout)
	if c, _ := jsend(t, "POST", r.base+"/v1/auth/logout", accA, ""); c != 204 && c != 200 {
		t.Fatalf("logout: %d", c)
	}
	for _, rt := range userRoutes {
		code, _ := do(t, rt.Method, r.base+fill(rt.Path, uidB), "Bearer "+accA, `{}`)
		checked++
		if code != 401 {
			t.Errorf("%s with a token captured before logout: got %d, want 401", rt.key(), code)
		}
	}
	t.Logf("generated authorization negatives: %d user routes x %d hostile tokens, plus tenant, owner, blocked and revoked-device cases: %d requests", len(userRoutes), len(cases), checked)
}

// mustAccess signs a returning user in again (new code) and returns the access token.
func mustAccess(t *testing.T, r *rig, phone, device string) string {
	t.Helper()
	acc, _, _ := r.user(phone, device)
	return acc
}

// The admin plane has its own listener and opaque session tokens; every route that is not part of the sign-in flow refuses a missing, malformed or unknown
// session, whatever the method, and before it reads the body.
func TestGeneratedAdminAuthorizationNegatives(t *testing.T) {
	_, dsn, _ := migrated(t)
	addr := freeAddr(t)
	start(t, adminEnv(dsn, "VG_MANAGER_ENABLED=true", "VG_MANAGER_ADDR="+addr)...)
	n := 0
	for _, rt := range discoverRoutes(t) {
		if !rt.Admin || !rt.Authed {
			continue
		}
		for _, h := range []string{"", "Basic dXNlcjpwYXNz", "Bearer ", "Bearer not-a-session", "Bearer " + strings.Repeat("a", 64)} {
			code, body := do(t, rt.Method, "http://"+addr+fill(rt.Path, "x"), h, `{"username":"x","reason":"x"}`)
			n++
			if code != 401 {
				t.Errorf("admin %s with %q: got %d %.80s, want 401", rt.key(), h, code, body)
			}
		}
	}
	if n < 20 {
		t.Fatalf("only %d admin requests were generated", n)
	}
	t.Logf("generated admin authorization negatives: %d requests", n)
}
