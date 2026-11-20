package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestLivenessAlwaysOK(t *testing.T) {
	r := NewRegistry(time.Second)
	r.Register("db", func(context.Context) error { return errors.New("down") })
	if rec := get(r.Mux(), "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("liveness must not depend on dependencies, got %d", rec.Code)
	}
}

func TestReadinessFailsWhenDependencyNotReadyAndHidesErrorDetails(t *testing.T) {
	r := NewRegistry(time.Second)
	r.Register("mysql", func(context.Context) error { return errors.New("dial tcp 10.0.0.5:3306: secret-host refused") })
	r.Register("redis", func(context.Context) error { return nil })
	rec := get(r.Mux(), "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Failing) != 1 || body.Failing[0] != "mysql" {
		t.Fatalf("expected only mysql failing, got %v", body.Failing)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") || strings.Contains(rec.Body.String(), "secret-host") {
		t.Fatal("readiness must not leak error details")
	}
}

func TestReadinessOKWhenAllChecksPassAndWhenNoChecks(t *testing.T) {
	r := NewRegistry(time.Second)
	if rec := get(r.Mux(), "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("no checks must be ready, got %d", rec.Code)
	}
	r.Register("a", func(context.Context) error { return nil })
	if rec := get(r.Mux(), "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("all passing must be ready, got %d", rec.Code)
	}
}

func TestReadinessTimeoutCountsAsFailure(t *testing.T) {
	r := NewRegistry(20 * time.Millisecond)
	r.Register("slow", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	if rec := get(r.Mux(), "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a check that exceeds the timeout must fail readiness, got %d", rec.Code)
	}
}

func TestOnlyGETIsAllowed(t *testing.T) {
	r := NewRegistry(time.Second)
	rec := httptest.NewRecorder()
	r.Mux().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("POST must not be served")
	}
}
