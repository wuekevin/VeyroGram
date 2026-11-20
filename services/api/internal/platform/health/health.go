// Package health provides liveness and readiness handlers. They are served only on the internal listener (never the public mux).
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Check reports whether one dependency is ready. It must honour ctx cancellation.
type Check func(ctx context.Context) error

// Registry holds readiness checks registered by modules and adapters.
type Registry struct {
	mu      sync.RWMutex
	checks  map[string]Check
	timeout time.Duration
}

// NewRegistry returns a registry whose readiness probe gives checks at most timeout in total.
func NewRegistry(timeout time.Duration) *Registry {
	return &Registry{checks: map[string]Check{}, timeout: timeout}
}

// Register adds or replaces a named readiness check.
func (r *Registry) Register(name string, c Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = c
}

// Live always answers 200 while the process can serve requests.
func (r *Registry) Live() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { write(w, http.StatusOK, response{Status: "ok"}) })
}

// Ready answers 200 only when every registered check passes, otherwise 503 listing the failing check names (never error details).
func (r *Registry) Ready() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), r.timeout)
		defer cancel()
		r.mu.RLock()
		names := make([]string, 0, len(r.checks))
		for n := range r.checks {
			names = append(names, n)
		}
		r.mu.RUnlock()
		sort.Strings(names)
		var failing []string
		for _, n := range names {
			r.mu.RLock()
			c := r.checks[n]
			r.mu.RUnlock()
			if err := c(ctx); err != nil {
				failing = append(failing, n)
			}
		}
		if len(failing) > 0 {
			write(w, http.StatusServiceUnavailable, response{Status: "unavailable", Failing: failing})
			return
		}
		write(w, http.StatusOK, response{Status: "ok"})
	})
}

// Mux returns the internal handler exposing /healthz and /readyz.
func (r *Registry) Mux() *http.ServeMux {
	m := http.NewServeMux()
	m.Handle("GET /healthz", r.Live())
	m.Handle("GET /readyz", r.Ready())
	return m
}

type response struct {
	Status  string   `json:"status"`
	Failing []string `json:"failing,omitempty"`
}

func write(w http.ResponseWriter, code int, body response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
