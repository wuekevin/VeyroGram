package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSMSBaoRoutesDomesticAndInternational(t *testing.T) {
	var gotPath string
	var gotQ url.Values
	reply := "0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQ = r.URL.Path, r.URL.Query()
		_, _ = w.Write([]byte(reply + "\n"))
	}))
	defer srv.Close()
	s := &SMSBao{User: "tom", Key: "KEY123", BaseURL: srv.URL}
	msg := "【VeyroGram】验证码 123456，5 分钟内有效。"

	if err := s.Send(context.Background(), "+8613800138000", msg); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/sms" || gotQ.Get("m") != "13800138000" || gotQ.Get("u") != "tom" || gotQ.Get("p") != "KEY123" || gotQ.Get("c") != msg {
		t.Fatalf("domestic: %s %v", gotPath, gotQ)
	}
	if err := s.Send(context.Background(), "+60901234567", msg); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/wsms" || gotQ.Get("m") != "+60901234567" {
		t.Fatalf("international must keep the + prefix: %s %v", gotPath, gotQ)
	}
	// Hong Kong is not mainland: international API
	if err := s.Send(context.Background(), "+85291234567", msg); err != nil || gotPath != "/wsms" {
		t.Fatalf("+852 must use the international API: %s %v", gotPath, err)
	}
}

func TestSMSBaoRawQueryEncodesPlus(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { raw = r.URL.RawQuery; _, _ = w.Write([]byte("0")) }))
	defer srv.Close()
	if err := (&SMSBao{User: "u", Key: "k", BaseURL: srv.URL}).Send(context.Background(), "+60901234567", "x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "m=%2B60901234567") {
		t.Fatalf("+ must be URL-encoded as %%2B: %s", raw)
	}
}

func TestSMSBaoErrorsNeverLeakSecretsOrNumbers(t *testing.T) {
	reply := "41"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(reply)) }))
	s := &SMSBao{User: "tom", Key: "SECRETKEY", BaseURL: srv.URL}
	err := s.Send(context.Background(), "+8613800138000", "x")
	if err == nil || !strings.Contains(err.Error(), "41") || strings.Contains(err.Error(), "SECRETKEY") || strings.Contains(err.Error(), "1380013") {
		t.Fatalf("balance error expected without secrets: %v", err)
	}
	reply = "<html>vendor says SECRETKEY and a long unexpected body</html>"
	if err := s.Send(context.Background(), "+8613800138000", "x"); err == nil || strings.Contains(err.Error(), "SECRETKEY") || strings.Contains(err.Error(), "html") {
		t.Fatalf("unknown reply must not be echoed: %v", err)
	}
	srv.Close() // connection refused: Go's error would contain the URL with the key
	if err := s.Send(context.Background(), "+8613800138000", "x"); err == nil || strings.Contains(err.Error(), "SECRETKEY") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("transport error must be sanitised: %v", err)
	}
}
