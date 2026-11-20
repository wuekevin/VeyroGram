package logx_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veyrogram/api/internal/platform/logx"
)

const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcDEF123_-xyz"

func TestSecretsNeverReachTheLogOutput(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelDebug)
	log.Info("login with Bearer abcdef1234567890SECRET and "+jwt, "token", "tok-SECRET-1", "access_token", "tok-SECRET-2", "Authorization", "Bearer zzzzzzzz1234",
		"phone", "+8613800138000", "mobile", "13800138000", "message_body", "hello this is private", "content", "private content", "password", "hunter2hunter2",
		"note", "call me on +86 138 0013 8000 or (415) 555-2671", "err", errors.New("dial vg:pw-SECRET@tcp(10.0.0.1:3306)/db failed"),
		"raw", []byte("binary-secret"), "nested", slog.GroupValue(slog.String("sms_code", "123456"), slog.String("ok", "fine")),
		"dsn", "vg:pw-SECRET@tcp(127.0.0.1:3306)/x", "jwt", jwt)
	out := buf.String()
	for _, leak := range []string{"SECRET", "13800138000", "138 0013 8000", "555-2671", "hello this is private", "private content", "hunter2", "binary-secret", "123456", "eyJhbGci", "zzzzzzzz1234"} {
		if strings.Contains(out, leak) {
			t.Errorf("log output leaks %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, `"ok":"fine"`) {
		t.Errorf("non-sensitive attributes must survive: %s", out)
	}
}

func TestWithAttrsAndGroupsAreRedactedToo(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo).With("token", "tok-SECRET").WithGroup("req")
	log.Info("x", "phone", "+8613800138000", "user", "u1")
	if strings.Contains(buf.String(), "SECRET") || strings.Contains(buf.String(), "13800138000") {
		t.Fatalf("leak via With/WithGroup: %s", buf.String())
	}
}

func TestJSONShapeAndTraceID(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo)
	h := logx.Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.InfoContext(r.Context(), "inside", "k", "v")
		w.WriteHeader(204)
	}))
	req := httptest.NewRequest("GET", "/v1/ping?token=URL-SECRET", nil)
	req.Header.Set("X-Request-Id", "deadbeefcafe0001")
	req.Header.Set("Authorization", "Bearer HEADER-SECRET-123456")
	h.ServeHTTP(httptest.NewRecorder(), req)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %s", len(lines), buf.String())
	}
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		if m["trace_id"] != "deadbeefcafe0001" {
			t.Errorf("every record needs the trace_id: %s", l)
		}
	}
	if strings.Contains(buf.String(), "URL-SECRET") || strings.Contains(buf.String(), "HEADER-SECRET") {
		t.Fatalf("access log must not include query string or headers: %s", buf.String())
	}
}

func TestScrubKeepsOrdinaryText(t *testing.T) {
	for _, s := range []string{"request 12345 took 250ms", "version 1.2.3", "id=deadbeef", "2026-11-20T10:00:00Z"} {
		if got := logx.Scrub(s); got != s {
			t.Errorf("Scrub(%q) = %q (false positive)", s, got)
		}
	}
}
