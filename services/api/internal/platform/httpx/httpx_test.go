package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	h := RequestID(http.HandlerFunc(NotFound))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	id := rec.Header().Get("X-Request-Id")
	if len(id) != 16 {
		t.Fatalf("expected generated 16-hex id, got %q", id)
	}
	var e Error
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.TraceID != id || e.Code != "not_found" {
		t.Fatalf("envelope must carry the same trace id, got %+v err=%v", e, err)
	}
}

func TestMalformedRequestIDIsReplaced(t *testing.T) {
	h := RequestID(http.HandlerFunc(NotFound))
	for _, bad := range []string{"x", "has space and <script>", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Request-Id", bad)
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("X-Request-Id"); got == bad {
			t.Fatalf("malformed id %q must not be echoed", bad)
		}
	}
}

func TestWellFormedRequestIDIsKept(t *testing.T) {
	h := RequestID(http.HandlerFunc(NotFound))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "abcdef01-2345")
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-Id"); got != "abcdef01-2345" {
		t.Fatalf("expected the caller's id, got %q", got)
	}
}
