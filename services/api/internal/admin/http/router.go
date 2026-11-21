// Package http is the admin plane's API (/v1/manager/*), served on its own listener (AD-24). Every route is registered through Router.Handle, which refuses a route
// without a policy: either a permission from the closed vocabulary, or an explicit Public marker for the few endpoints that exist before a session does (login
// and the second factor). A route that declares neither cannot be registered, so it cannot ship (AD-3: "a route without a policy fails startup").
package http

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"veyrogram/api/internal/admin/app"
	"veyrogram/api/internal/admin/domain"
	"veyrogram/api/internal/admin/ports"
	"veyrogram/api/internal/platform/httpx"
)

// Policy is what a route declares. Exactly one of Permission or Public must be set.
type Policy struct {
	Permission string // one of domain.Perm*
	Public     bool   // reachable without a session (login, second factor); rate limited harder
}

// Route is one entry of the routing table; tests generate their cases from it.
type Route struct {
	Method, Path string
	Policy       Policy
}

type Router struct {
	Svc *app.Service
	// Allow restricts the admin plane to these networks (empty = any address that can reach the listener).
	Allow []*net.IPNet
	// ClientIP resolves the caller's address (trusted-proxy aware).
	ClientIP func(*http.Request) string
	// OnDenied is called for every 401/403 with the reason, for metrics and alerts.
	OnDenied func(reason string)
	Now      func() time.Time
	// PublicPerMinute is the per-address budget for the login endpoints (default 20); the rest of the API gets 300.
	PublicPerMinute int

	mux    *http.ServeMux
	routes []Route
	lim    *limiter
}

type handler func(w http.ResponseWriter, r *http.Request, p ports.Principal)

func NewRouter(svc *app.Service) *Router {
	r := &Router{Svc: svc, mux: http.NewServeMux(), lim: newLimiter()}
	r.mux.HandleFunc("/", httpx.NotFound)
	return r
}

// Handle registers a route. It panics on a missing or unknown policy: that is a programming error found at startup, never at request time.
func (r *Router) Handle(method, path string, pol Policy, h handler) {
	switch {
	case pol.Public && pol.Permission != "":
		panic(fmt.Sprintf("admin route %s %s: a public route cannot also name a permission", method, path))
	case !pol.Public && pol.Permission == "":
		panic(fmt.Sprintf("admin route %s %s: no policy declared", method, path))
	case !pol.Public && !domain.KnownPermission(pol.Permission):
		panic(fmt.Sprintf("admin route %s %s: unknown permission %q", method, path, pol.Permission))
	}
	r.routes = append(r.routes, Route{method, path, pol})
	r.mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, req *http.Request) { r.serve(w, req, pol, h) })
}

// Routes returns the routing table (copy), sorted.
func (r *Router) Routes() []Route {
	out := append([]Route(nil), r.routes...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path+out[i].Method < out[j].Path+out[j].Method })
	return out
}

func (r *Router) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Router) denied(reason string) {
	if r.OnDenied != nil {
		r.OnDenied(reason)
	}
}

func (r *Router) ip(req *http.Request) string {
	if r.ClientIP != nil {
		return r.ClientIP(req)
	}
	h, _, _ := net.SplitHostPort(req.RemoteAddr)
	return h
}

func (r *Router) allowed(ip string) bool {
	if len(r.Allow) == 0 {
		return true
	}
	a := net.ParseIP(ip)
	if a == nil {
		return false
	}
	for _, n := range r.Allow {
		if n.Contains(a) {
			return true
		}
	}
	return false
}

// ServeHTTP is the whole admin plane: allowlist, rate limit, headers, then the routed handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	ip := r.ip(req)
	if !r.allowed(ip) {
		r.denied("ip_not_allowed")
		httpx.WriteError(w, req, http.StatusForbidden, "forbidden", "forbidden")
		return
	}
	r.mux.ServeHTTP(w, req)
}

func bearer(req *http.Request) string {
	v := req.Header.Get("Authorization")
	if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return ""
}

func (r *Router) serve(w http.ResponseWriter, req *http.Request, pol Policy, h handler) {
	ip := r.ip(req)
	limit := 300 // per minute per address on authenticated routes
	if pol.Public {
		limit = 20 // login and second factor: brute force surface
		if r.PublicPerMinute > 0 {
			limit = r.PublicPerMinute
		}
	}
	if ok, wait := r.lim.allow(ip, pol.Public, limit, r.now()); !ok {
		r.denied("rate_limited")
		w.Header().Set("Retry-After", fmt.Sprint(int(wait.Seconds())+1))
		httpx.WriteError(w, req, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return
	}
	if pol.Public {
		h(w, req, ports.Principal{})
		return
	}
	p, err := r.Svc.Authenticate(req.Context(), bearer(req))
	if err != nil {
		if err == domain.ErrUnauthenticated {
			r.denied("unauthenticated")
			httpx.WriteError(w, req, http.StatusUnauthorized, "unauthenticated", "sign in again")
			return
		}
		httpx.WriteError(w, req, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if !p.Perms[pol.Permission] {
		r.denied("forbidden")
		httpx.WriteError(w, req, http.StatusForbidden, "forbidden", "your role does not allow this")
		return
	}
	h(w, req, p)
}

// limiter is a small in-memory fixed-window counter keyed by address. The admin plane is low traffic and may run without Redis; a restart resets it, which
// is acceptable because account lockout (in MySQL) is the durable brute-force control.
type limiter struct {
	mu sync.Mutex
	w  map[string]*win
}
type win struct {
	start time.Time
	n     int
}

func newLimiter() *limiter { return &limiter{w: map[string]*win{}} }

func (l *limiter) allow(ip string, public bool, limit int, now time.Time) (bool, time.Duration) {
	key := ip
	if public {
		key = "p:" + ip
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.w) > 10000 { // bounded memory
		for k, v := range l.w {
			if now.Sub(v.start) > time.Minute {
				delete(l.w, k)
			}
		}
	}
	x := l.w[key]
	if x == nil || now.Sub(x.start) >= time.Minute {
		x = &win{start: now}
		l.w[key] = x
	}
	x.n++
	if x.n > limit {
		return false, time.Minute - now.Sub(x.start)
	}
	return true, 0
}
