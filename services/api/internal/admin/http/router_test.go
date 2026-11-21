package http_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/admin/app"
	"veyrogram/api/internal/admin/domain"
	adminhttp "veyrogram/api/internal/admin/http"
	"veyrogram/api/internal/admin/ports"
	"veyrogram/api/internal/platform/audit"
)

// fakeStore only knows sessions: it is enough to drive the authentication and authorisation layer, which is what the generated matrix exercises.
type fakeStore struct {
	ports.Store // every method not overridden panics: the matrix must never reach business logic through an unauthorised request
	sessions    map[string]ports.Principal
	touched     int
}

func (f *fakeStore) Authenticate(_ context.Context, hash string, _ time.Time, _ time.Duration) (ports.Principal, bool, error) {
	for tok, p := range f.sessions {
		if hashOf(tok) == hash {
			f.touched++
			return p, true, nil
		}
	}
	return ports.Principal{}, false, nil
}

// The few store methods an AUTHORISED request legitimately reaches with the matrix' empty bodies. Everything else still panics.
func (f *fakeStore) ListAdmins(context.Context, uint64) ([]ports.Admin, error) { return nil, nil }
func (f *fakeStore) ListAudit(context.Context, uint64, ports.AuditFilter) ([]ports.AuditRow, error) {
	return nil, nil
}
func (f *fakeStore) AdminByUsername(context.Context, uint64, string) (ports.Admin, bool, error) {
	return ports.Admin{}, false, nil
}
func (f *fakeStore) ConsumeChallenge(context.Context, string, string, time.Time) (uint64, bool, error) {
	return 0, false, nil
}
func (f *fakeStore) RevokeSession(context.Context, uint64, time.Time) error { return nil }

type nopHasher struct{}

func (nopHasher) Hash(string) (string, error) { return "h", nil }
func (nopHasher) Verify(string, string) bool  { return false }
func (nopHasher) DummyVerify(string)          {}

type nopAudit struct{}

func (nopAudit) Append(context.Context, audit.Entry) error  { return nil }
func (nopAudit) Verify(context.Context) (audit.Head, error) { return audit.Head{}, nil }

func hashOf(t string) string { return sha256hex(t) }

func newRouter(t *testing.T, st ports.Store) *adminhttp.Router {
	t.Helper()
	svc := &app.Service{Store: st, Hasher: nopHasher{}, Audit: nopAudit{}, Policy: app.DefaultPolicy()}
	r := adminhttp.NewRouter(svc)
	r.Mount(nil)
	return r
}

func tok(n int) string { return fmt.Sprintf("%064x", n) } // 64 hex characters, the shape of a real session token

// roles under test: the real ones plus one that holds nothing, so "403 unless the permission is held" is checked for every permission.
func rolePerms() map[string]map[string]bool {
	all := func(ps ...string) map[string]bool {
		m := map[string]bool{}
		for _, p := range ps {
			m[p] = true
		}
		return m
	}
	return map[string]map[string]bool{
		"owner":    all(domain.PermSelf, domain.PermAuditRead, domain.PermAuditVerify, domain.PermAdminRead, domain.PermAdminWrite),
		"auditor":  all(domain.PermSelf, domain.PermAuditRead, domain.PermAuditVerify),
		"nobody":   all(),
		"selfonly": all(domain.PermSelf),
	}
}

func pathOf(p string) string { return strings.ReplaceAll(p, "{id}", "7") }

func do(h http.Handler, method, path, auth, remote string) int {
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.RemoteAddr = remote
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}

// The generated authorisation matrix (AD-3: "covered by generated negative tests"): the cases come from the routing table itself, so a new route is covered
// the moment it is registered, with no test edit.
func TestEveryRouteEnforcesAuthenticationAndTheDeclaredPermission(t *testing.T) {
	st := &fakeStore{sessions: map[string]ports.Principal{}}
	n := 1
	byRole := map[string]string{}
	for role, perms := range rolePerms() {
		n++
		token := tok(n)
		byRole[role] = token
		st.sessions[token] = ports.Principal{AdminID: uint64(n), Username: role, Perms: perms, SessionID: uint64(n)}
	}
	r := newRouter(t, st)
	routes := r.Routes()
	if len(routes) < 8 {
		t.Fatalf("suspiciously small routing table: %d", len(routes))
	}
	for _, rt := range routes {
		label := rt.Method + " " + rt.Path
		path := pathOf(rt.Path)
		if rt.Policy.Public {
			// reachable without a session, but never able to produce one without the second factor: that is tested in the integration test
			if c := do(r, rt.Method, path, "", "10.0.0.1:1"); c == http.StatusUnauthorized && rt.Path != "/v1/manager/login" && !strings.HasPrefix(rt.Path, "/v1/manager/mfa/") {
				t.Errorf("%s: public route answered 401 without credentials", label)
			}
			continue
		}
		// 1. nothing presented
		for name, auth := range map[string]string{
			"no credentials":          "",
			"wrong scheme":            "Basic " + tok(2),
			"empty bearer":            "Bearer ",
			"short garbage":           "Bearer abc",
			"unknown well-formed":     "Bearer " + tok(99999),
			"a login challenge token": "Bearer " + tok(123456), // challenge tokens look exactly like session tokens but are not sessions
			"sql-ish":                 "Bearer ' OR '1'='1",
			"overlong":                "Bearer " + strings.Repeat("a", 5000),
		} {
			if c := do(r, rt.Method, path, auth, "10.0.0.1:1"); c != http.StatusUnauthorized {
				t.Errorf("%s with %s: %d, want 401", label, name, c)
			}
		}
		// 2. every role: allowed exactly when it holds the declared permission
		for role, perms := range rolePerms() {
			c := do(r, rt.Method, path, "Bearer "+byRole[role], "10.0.0.1:1")
			if perms[rt.Policy.Permission] {
				if c == http.StatusUnauthorized || c == http.StatusForbidden {
					t.Errorf("%s as %s holds %q but was refused: %d", label, role, rt.Policy.Permission, c)
				}
			} else if c != http.StatusForbidden {
				t.Errorf("%s as %s lacks %q: %d, want 403", label, role, rt.Policy.Permission, c)
			}
		}
	}
	if st.touched == 0 {
		t.Fatal("the matrix never reached an authenticated request")
	}
}

func TestRoutesWithoutAPolicyCannotBeRegistered(t *testing.T) {
	r := adminhttp.NewRouter(&app.Service{})
	h := func(http.ResponseWriter, *http.Request, ports.Principal) {}
	for name, pol := range map[string]adminhttp.Policy{
		"no policy":               {},
		"unknown permission":      {Permission: "audit:delete"},
		"public and a permission": {Public: true, Permission: domain.PermSelf},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: registration must fail at startup", name)
				}
			}()
			r.Handle("GET", "/v1/manager/x", pol, h)
		}()
	}
	r.Handle("GET", "/v1/manager/ok", adminhttp.Policy{Permission: domain.PermSelf}, h) // a valid one still works
}

// Nothing in the admin plane may register a route behind the router's back: every Handle/HandleFunc call outside router.go is a bypass of the policy check.
func TestNoRouteIsRegisteredOutsideTheRouter(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "router.go" {
			continue
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok {
					switch s.Sel.Name {
					case "Handle", "HandleFunc":
						// r.Handle(...) on the Router is the sanctioned way; anything on a mux is a bypass. The Router is the only receiver named r.
						if id, ok := s.X.(*ast.Ident); !ok || id.Name != "r" {
							t.Errorf("%s: %s registers a route without a policy", f, s.Sel.Name)
						}
					}
				}
			}
			return true
		})
	}
}

func TestAllowlistAndRateLimit(t *testing.T) {
	st := &fakeStore{sessions: map[string]ports.Principal{tok(5): {AdminID: 5, Username: "o", Perms: rolePerms()["owner"], SessionID: 5}}}
	r := newRouter(t, st)
	_, cidr, _ := net.ParseCIDR("10.0.0.0/8")
	r.Allow = []*net.IPNet{cidr}
	var denied []string
	r.OnDenied = func(reason string) { denied = append(denied, reason) }
	for _, rt := range r.Routes() {
		if c := do(r, rt.Method, pathOf(rt.Path), "Bearer "+tok(5), "192.168.1.1:1"); c != http.StatusForbidden {
			t.Errorf("%s %s from outside the allowlist (even with a valid session): %d, want 403", rt.Method, rt.Path, c)
		}
	}
	if len(denied) == 0 || denied[0] != "ip_not_allowed" {
		t.Errorf("denials must be reported: %v", denied)
	}
	if c := do(r, "GET", "/v1/manager/me", "Bearer "+tok(5), "10.1.2.3:1"); c != 200 {
		t.Errorf("inside the allowlist: %d", c)
	}
	// the login endpoints are rate limited per address, much tighter than the rest
	var last int
	for i := 0; i < 25; i++ {
		last = do(r, "POST", "/v1/manager/login", "", "10.9.9.9:1")
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("the 25th login attempt within a minute must be rate limited: %d", last)
	}
	if c := do(r, "POST", "/v1/manager/login", "", "10.9.9.10:1"); c == http.StatusTooManyRequests {
		t.Error("another address has its own budget")
	}
	// unknown paths are 404, never a handler
	if c := do(r, "GET", "/v1/auth/login", "", "10.1.1.1:1"); c != http.StatusNotFound {
		t.Errorf("the user API's paths do not exist on the admin listener: %d", c)
	}
}
