package idempotency_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"veyrogram/api/internal/platform/idempotency"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/testdb"
	"veyrogram/api/migrations"
)

func store(t *testing.T) *idempotency.Store {
	t.Helper()
	db := testdb.New(t)
	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&migrate.Runner{DB: db, Migrations: ms}).Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &idempotency.Store{DB: db}
}

type counter struct {
	n      atomic.Int32
	status int
	hold   chan struct{}
}

func (c *counter) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.n.Add(1)
		if c.hold != nil {
			<-c.hold
		}
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		st := c.status
		if st == 0 {
			st = 201
		}
		w.WriteHeader(st)
		_, _ = w.Write([]byte(`{"echo":"` + string(b) + `","n":` + string(rune('0'+c.n.Load())) + `}`))
	})
}

func do(h http.Handler, method, path, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const k1 = "key-0001-abcdef"

func TestReplayReturnsFirstResponseWithoutReexecuting(t *testing.T) {
	c := &counter{}
	h := idempotency.Middleware(store(t), nil, nil)(c.handler())
	first := do(h, "POST", "/v1/messages", k1, "hello")
	second := do(h, "POST", "/v1/messages", k1, "hello")
	if c.n.Load() != 1 {
		t.Fatalf("handler must run once, ran %d times", c.n.Load())
	}
	if second.Code != 201 || second.Body.String() != first.Body.String() || second.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay mismatch: %d %q replayed=%q", second.Code, second.Body.String(), second.Header().Get("Idempotent-Replayed"))
	}
	if first.Header().Get("Idempotent-Replayed") != "" {
		t.Fatal("the first response must not be marked as replayed")
	}
}

func TestSameKeyDifferentBodyIsRejectedAndNeverLeaksTheFirstResponse(t *testing.T) {
	c := &counter{}
	h := idempotency.Middleware(store(t), nil, nil)(c.handler())
	do(h, "POST", "/v1/messages", k1, "secret-first-body")
	rec := do(h, "POST", "/v1/messages", k1, "different")
	if rec.Code != 422 || strings.Contains(rec.Body.String(), "secret-first-body") || c.n.Load() != 1 {
		t.Fatalf("expected 422 without replay, got %d %q (runs=%d)", rec.Code, rec.Body.String(), c.n.Load())
	}
}

func TestInFlightDuplicateGets409(t *testing.T) {
	c := &counter{hold: make(chan struct{})}
	h := idempotency.Middleware(store(t), nil, nil)(c.handler())
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- do(h, "POST", "/v1/messages", k1, "x") }()
	deadline := time.Now().Add(3 * time.Second)
	for c.n.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if rec := do(h, "POST", "/v1/messages", k1, "x"); rec.Code != 409 {
		t.Fatalf("duplicate while in flight must be 409, got %d", rec.Code)
	}
	close(c.hold)
	if rec := <-done; rec.Code != 201 {
		t.Fatalf("first request must finish normally, got %d", rec.Code)
	}
}

func TestServerErrorsAreNotCachedSoClientCanRetry(t *testing.T) {
	c := &counter{status: 500}
	h := idempotency.Middleware(store(t), nil, nil)(c.handler())
	do(h, "POST", "/v1/messages", k1, "x")
	c.status = 201
	rec := do(h, "POST", "/v1/messages", k1, "x")
	if c.n.Load() != 2 || rec.Code != 201 || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("a 5xx must not be cached: runs=%d code=%d", c.n.Load(), rec.Code)
	}
}

func TestPanicReleasesTheKey(t *testing.T) {
	var n atomic.Int32
	h := idempotency.Middleware(store(t), nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			panic("boom")
		}
		w.WriteHeader(204)
	}))
	func() {
		defer func() { _ = recover() }()
		do(h, "POST", "/v1/x", k1, "x")
	}()
	if rec := do(h, "POST", "/v1/x", k1, "x"); rec.Code != 204 {
		t.Fatalf("after a panic the key must be released for retry, got %d", rec.Code)
	}
}

func TestExpiredKeyIsExecutedAgain(t *testing.T) {
	s := store(t)
	now := time.Now().UTC()
	s.Now = func() time.Time { return now }
	s.TTL = time.Hour
	c := &counter{}
	h := idempotency.Middleware(s, nil, nil)(c.handler())
	do(h, "POST", "/v1/messages", k1, "x")
	now = now.Add(2 * time.Hour)
	rec := do(h, "POST", "/v1/messages", k1, "x")
	if c.n.Load() != 2 || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("after expiry the request must run again, runs=%d", c.n.Load())
	}
}

func TestDefaultTTLIs24Hours(t *testing.T) {
	s := store(t)
	h := idempotency.Middleware(s, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	do(h, "POST", "/v1/a", k1, "x")
	var secs int
	if err := s.DB.QueryRow("SELECT TIMESTAMPDIFF(SECOND, created_at, expires_at) FROM idempotency_key LIMIT 1").Scan(&secs); err != nil || secs != 24*3600 {
		t.Fatalf("default TTL must be 24h, got %ds err=%v", secs, err)
	}
}

func TestAbandonedInProgressRowIsReclaimed(t *testing.T) {
	s := store(t)
	now := time.Now().UTC()
	s.Now = func() time.Time { return now }
	s.StaleAfter = time.Minute
	c := &counter{hold: make(chan struct{})}
	h := idempotency.Middleware(s, nil, nil)(c.handler())
	go func() { do(h, "POST", "/v1/m", k1, "x") }() // never finishes within the test window: simulates a crashed process
	for c.n.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	now = now.Add(5 * time.Minute)
	c2 := &counter{}
	rec := do(idempotency.Middleware(s, nil, nil)(c2.handler()), "POST", "/v1/m", k1, "x")
	close(c.hold)
	if rec.Code != 201 || c2.n.Load() != 1 {
		t.Fatalf("a stale in-progress claim must be reclaimed, got %d runs=%d", rec.Code, c2.n.Load())
	}
}

func TestKeysAreIsolatedByPathAndByTenantAndSubject(t *testing.T) {
	s := store(t)
	c := &counter{}
	tenant := func(r *http.Request) uint64 {
		if r.Header.Get("X-T") == "2" {
			return 2
		}
		return 1
	}
	subject := func(r *http.Request) string { return r.Header.Get("X-U") }
	h := idempotency.Middleware(s, subject, tenant)(c.handler())
	do(h, "POST", "/v1/a", k1, "x")
	do(h, "POST", "/v1/b", k1, "x") // other path
	req := httptest.NewRequest("POST", "/v1/a", strings.NewReader("x"))
	req.Header.Set("Idempotency-Key", k1)
	req.Header.Set("X-T", "2") // other tenant
	h.ServeHTTP(httptest.NewRecorder(), req)
	req = httptest.NewRequest("POST", "/v1/a", strings.NewReader("x"))
	req.Header.Set("Idempotency-Key", k1)
	req.Header.Set("X-U", "someone-else") // other principal
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if c.n.Load() != 4 || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("path, tenant and subject must each isolate keys; runs=%d (want 4)", c.n.Load())
	}
}

func TestPassThroughRulesAndValidation(t *testing.T) {
	c := &counter{}
	h := idempotency.Middleware(store(t), nil, nil)(c.handler())
	do(h, "GET", "/v1/a", k1, "")
	do(h, "GET", "/v1/a", k1, "")
	do(h, "POST", "/v1/a", "", "x")
	do(h, "POST", "/v1/a", "", "x")
	if c.n.Load() != 4 {
		t.Fatalf("GET and header-less requests are never deduplicated, runs=%d", c.n.Load())
	}
	for _, bad := range []string{"short", "has space in it!", strings.Repeat("a", 129), "bad/slash/key1"} {
		if rec := do(h, "POST", "/v1/a", bad, "x"); rec.Code != 400 {
			t.Errorf("invalid key %q must be 400, got %d", bad, rec.Code)
		}
	}
	if rec := do(h, "POST", "/v1/a", "key-large-0001", strings.Repeat("x", (1<<20)+10)); rec.Code != 413 {
		t.Fatalf("oversized body must be 413, got %d", rec.Code)
	}
}

func TestConcurrentIdenticalRequestsExecuteOnce(t *testing.T) {
	c := &counter{}
	h := idempotency.Middleware(store(t), nil, nil)(c.handler())
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func() { defer wg.Done(); codes[i] = do(h, "POST", "/v1/m", "key-race-0001", "same").Code }()
	}
	wg.Wait()
	if c.n.Load() != 1 {
		t.Fatalf("8 concurrent identical requests must run the handler exactly once, ran %d", c.n.Load())
	}
	for _, code := range codes {
		if code != 201 && code != 409 {
			t.Fatalf("each response must be the original 201 or an in-progress 409, got %d", code)
		}
	}
}
