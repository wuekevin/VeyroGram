package anchorstore_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"veyrogram/api/internal/anchorstore"
	"veyrogram/api/internal/platform/audit"
)

const wt, rt = "write-token-0123456789abcdef012345", "read-token-0123456789abcdef0123456"

func do(t *testing.T, srv *httptest.Server, method, path, tok, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestWriteOnceAndTokenSeparation(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(anchorstore.Handler(anchorstore.Config{Dir: dir, WriteToken: wt, ReadToken: rt}))
	defer srv.Close()
	p := "/audit-anchor/2026-11-20.json"
	if c, _ := do(t, srv, "PUT", p, "", `{"a":1}`); c != 401 {
		t.Fatalf("no token: %d", c)
	}
	if c, _ := do(t, srv, "PUT", p, rt, `{"a":1}`); c != 401 {
		t.Fatalf("the read token must not write: %d", c)
	}
	if c, _ := do(t, srv, "GET", p, wt, ""); c != 401 {
		t.Fatalf("the write token must not read: %d", c)
	}
	if c, _ := do(t, srv, "PUT", p, wt, `{"a":1}`); c != 201 {
		t.Fatalf("first write: %d", c)
	}
	if c, _ := do(t, srv, "PUT", p, wt, `{"a":1}`); c != 200 {
		t.Fatalf("identical retry must be idempotent: %d", c)
	}
	if c, _ := do(t, srv, "PUT", p, wt, `{"a":2}`); c != 409 {
		t.Fatalf("overwrite with different content must be refused: %d", c)
	}
	if c, b := do(t, srv, "GET", p, rt, ""); c != 200 || b != `{"a":1}` {
		t.Fatalf("original must be intact: %d %q", c, b)
	}
	for _, m := range []string{"DELETE", "POST", "PATCH"} {
		if c, _ := do(t, srv, m, p, wt, "x"); c != 405 {
			t.Errorf("%s must not exist: %d", m, c)
		}
	}
	for _, bad := range []string{"/audit-anchor/../x.json", "/audit-anchor/2026-11-20.txt", "/audit-anchor/%2e%2e%2fetc.json", "/other"} {
		if c, _ := do(t, srv, "PUT", bad, wt, "x"); c != 404 && c != 400 {
			t.Errorf("%s must be refused: %d", bad, c)
		}
	}
	if st, _ := os.Stat(filepath.Join(dir, "2026-11-20.json")); st == nil || st.Mode().Perm() != 0o444 {
		t.Fatalf("stored anchors must be read-only on disk: %v", st)
	}
	if c, _ := do(t, srv, "PUT", p, "", ""); c == 201 {
		t.Fatal("empty token config must never be open")
	}
	closed := httptest.NewServer(anchorstore.Handler(anchorstore.Config{Dir: t.TempDir()}))
	defer closed.Close()
	if c, _ := do(t, closed, "PUT", p, "", "x"); c != 401 {
		t.Fatalf("with no tokens configured every request must be refused, got %d", c)
	}
}

// The audit package's own HTTP client must interoperate with the store: Put, Get, idempotent re-put, refused overwrite.
func TestAuditHTTPSinkInteroperatesWithTheStore(t *testing.T) {
	srv := httptest.NewServer(anchorstore.Handler(anchorstore.Config{Dir: t.TempDir(), WriteToken: wt, ReadToken: rt}))
	defer srv.Close()
	w := audit.HTTPSink{Base: srv.URL, Token: wt}
	r := audit.HTTPSink{Base: srv.URL, Token: rt}
	ctx := context.Background()
	k := "audit-anchor/2026-11-21.json"
	if err := w.Put(ctx, k, []byte(`{"head_id":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(ctx, k, []byte(`{"head_id":1}`)); err != nil {
		t.Fatalf("the daily job may re-run: %v", err)
	}
	if err := w.Put(ctx, k, []byte(`{"head_id":9}`)); err == nil {
		t.Fatal("a second, different anchor for the same day must be refused (that is the point of the store)")
	}
	b, found, err := r.Get(ctx, k)
	if err != nil || !found || string(b) != `{"head_id":1}` {
		t.Fatalf("%q %v %v", b, found, err)
	}
	if _, found, err := r.Get(ctx, "audit-anchor/2026-01-01.json"); err != nil || found {
		t.Fatalf("a missing anchor is 'not found', not an error: %v %v", found, err)
	}
}
