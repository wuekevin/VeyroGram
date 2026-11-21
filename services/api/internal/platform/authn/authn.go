// Package authn authenticates API requests: it verifies the access token and consults the deny-list on every request (AD-9).
// When the deny-list is unreachable the request is allowed (revocation then degrades to the access-token TTL, at most 15 minutes) and
// OnDegraded is called so the metric and alert fire; a token that is invalid or denied is never allowed.
package authn

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/httpx"
	"veyrogram/api/internal/platform/token"
)

type ctxKey struct{}

// FromContext returns the verified claims placed by Require.
func FromContext(ctx context.Context) (token.Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(token.Claims)
	return c, ok
}

type Config struct {
	Ring       *token.Ring
	Deny       denylist.List // nil = no deny-list configured: always degraded
	Now        func() time.Time
	OnDegraded func() // called when the deny-list cannot be consulted
}

// Require wraps next, answering 401 invalid_token unless a valid, non-revoked access token is presented.
func Require(c Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				unauthorized(w, r)
				return
			}
			now := time.Now()
			if c.Now != nil {
				now = c.Now()
			}
			claims, err := c.Ring.Verify(strings.TrimPrefix(h, "Bearer "), now)
			if err != nil {
				unauthorized(w, r)
				return
			}
			dev, err := strconv.ParseUint(claims.Device, 10, 64)
			if err != nil {
				unauthorized(w, r)
				return
			}
			if c.Deny == nil {
				c.degraded()
			} else if denied, err := c.Deny.Denied(r.Context(), claims.Tenant, dev); err != nil {
				c.degraded()
			} else if denied {
				unauthorized(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
		})
	}
}

func (c Config) degraded() {
	if c.OnDegraded != nil {
		c.OnDegraded()
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	httpx.WriteError(w, r, http.StatusUnauthorized, "invalid_token", "missing, invalid, expired or revoked access token")
}
