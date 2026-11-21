package adapters

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veyrogram/api/internal/identity/domain"
)

func TestHTTPSender(t *testing.T) {
	var got map[string]string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		if got["message"] == "boom" {
			http.Error(w, "vendor says secret-detail +8613800138000", 500)
		}
	}))
	defer srv.Close()
	s := &HTTPSender{URL: srv.URL, Token: "tok"}
	if err := s.Send(context.Background(), "+8613800138000", "code 123456"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok" || got["to"] != "+8613800138000" || got["message"] != "code 123456" {
		t.Fatalf("unexpected request: %v %q", got, auth)
	}
	err := s.Send(context.Background(), "+8613800138000", "boom")
	if err == nil || strings.Contains(err.Error(), "secret-detail") || strings.Contains(err.Error(), "1380013") {
		t.Fatalf("error must exist and leak neither body nor number: %v", err)
	}
	if err := (&HTTPSender{URL: "http://127.0.0.1:1"}).Send(context.Background(), "+8613800138000", "x"); err == nil {
		t.Fatal("unreachable gateway must error")
	}
}

func TestLogSenderMasksNumber(t *testing.T) {
	var sb strings.Builder
	l := slog.New(slog.NewTextHandler(&sb, nil))
	if err := (LogSender{Log: l}).Send(context.Background(), domain.Phone("+8613800138000"), "code 654321"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sb.String(), "1380013") || !strings.Contains(sb.String(), "8000") {
		t.Fatalf("log must show only last4: %s", sb.String())
	}
}
