package authn

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"veyrogram/api/internal/platform/token"
)

type fakeDeny struct {
	denied map[uint64]bool
	err    error
}

func (f *fakeDeny) Deny(_ context.Context, _ uint64, d uint64, _ time.Duration) error {
	f.denied[d] = true
	return nil
}
func (f *fakeDeny) Denied(_ context.Context, _ uint64, d uint64) (bool, error) {
	return f.denied[d], f.err
}

func setup(t *testing.T) (*token.Ring, *fakeDeny, time.Time) {
	seed := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdeX"))
	r, err := token.ParseRing("k1:" + seed)
	if err != nil {
		t.Fatal(err)
	}
	return r, &fakeDeny{denied: map[uint64]bool{}}, time.Unix(1_700_000_000, 0)
}

func call(h http.Handler, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/x", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRequireVerifiesAndChecksDenyListOnEveryRequest(t *testing.T) {
	ring, deny, now := setup(t)
	var seen token.Claims
	degraded := 0
	h := Require(Config{Ring: ring, Deny: deny, Now: func() time.Time { return now }, OnDegraded: func() { degraded++ }})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = FromContext(r.Context())
	}))
	tok, _ := ring.Sign(token.Claims{Subject: "u1", Device: "9", Class: "app", Tenant: 1, Expires: now.Add(time.Minute).Unix()})
	if w := call(h, "Bearer "+tok); w.Code != 200 || seen.Subject != "u1" {
		t.Fatalf("valid token: %d", w.Code)
	}
	deny.denied[9] = true // revoke: effective on the very next request, no waiting for expiry
	if w := call(h, "Bearer "+tok); w.Code != 401 {
		t.Fatalf("revoked device's token must be refused immediately, got %d", w.Code)
	}
	for name, a := range map[string]string{"none": "", "no scheme": tok, "basic": "Basic abc", "garbage": "Bearer a.b.c"} {
		if w := call(h, a); w.Code != 401 {
			t.Errorf("%s: %d", name, w.Code)
		}
	}
	expired, _ := ring.Sign(token.Claims{Subject: "u1", Device: "10", Tenant: 1, Expires: now.Add(-time.Second).Unix()})
	if w := call(h, "Bearer "+expired); w.Code != 401 {
		t.Fatalf("expired: %d", w.Code)
	}
	badDev, _ := ring.Sign(token.Claims{Subject: "u1", Device: "x", Tenant: 1, Expires: now.Add(time.Minute).Unix()})
	if w := call(h, "Bearer "+badDev); w.Code != 401 {
		t.Fatalf("non-numeric device: %d", w.Code)
	}
	if degraded != 0 {
		t.Fatal("healthy deny-list must not report degradation")
	}
}

func TestRedisFailureDegradesToTTLAndAlerts(t *testing.T) {
	ring, deny, now := setup(t)
	deny.err = errors.New("redis down")
	degraded := 0
	h := Require(Config{Ring: ring, Deny: deny, Now: func() time.Time { return now }, OnDegraded: func() { degraded++ }})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tok, _ := ring.Sign(token.Claims{Subject: "u1", Device: "9", Tenant: 1, Expires: now.Add(time.Minute).Unix()})
	if w := call(h, "Bearer "+tok); w.Code != 200 || degraded != 1 {
		t.Fatalf("with Redis down a valid token is served and the degradation reported: %d degraded=%d", w.Code, degraded)
	}
	// but invalid tokens are never let through just because Redis is down
	if w := call(h, "Bearer garbage"); w.Code != 401 {
		t.Fatalf("invalid token with Redis down: %d", w.Code)
	}
	// unconfigured deny-list counts as degraded too
	degraded = 0
	h2 := Require(Config{Ring: ring, Now: func() time.Time { return now }, OnDegraded: func() { degraded++ }})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if w := call(h2, "Bearer "+tok); w.Code != 200 || degraded != 1 {
		t.Fatalf("no deny-list: %d %d", w.Code, degraded)
	}
	_ = strconv.Itoa
}
