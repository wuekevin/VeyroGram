package imwebhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imwebhook"
)

const signKey = "signing-key-0123456789abcdef0123456789"

func sign(key, ts, body string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(ts + "." + body))
	return "v1=" + hex.EncodeToString(m.Sum(nil))
}

func TestSigningRequiredWhenConfiguredAndForgeriesAreRejected(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	r := newRig(t, func(c *imwebhook.Config) {
		c.SigningSecret, c.Now = signKey, func() time.Time { return now }
		c.Handlers.BeforeSend = nil // denies; we only care whether the request gets past authentication (200) or is refused (403)
	})
	do := func(hts, hsig, body string) int {
		req := httptest.NewRequest("POST", okPath, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:5000"
		req.Header.Set("Content-Type", "application/json")
		if hts != "" {
			req.Header.Set(imwebhook.HeaderTimestamp, hts)
		}
		if hsig != "" {
			req.Header.Set(imwebhook.HeaderSignature, hsig)
		}
		rec := httptest.NewRecorder()
		r.h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do(ts, sign(signKey, ts, okBody), okBody); c != 200 {
		t.Fatalf("a correctly signed callback must pass authentication, got %d", c)
	}
	stale := strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10)
	for name, c := range map[string]int{
		"unsigned":         do("", "", okBody),
		"wrong key":        do(ts, sign("another-key-another-key-another-key", ts, okBody), okBody),
		"tampered body":    do(ts, sign(signKey, ts, okBody), strings.Replace(okBody, "u1", "u9", 1)),
		"replayed (stale)": do(stale, sign(signKey, stale, okBody), okBody),
		"bad timestamp":    do("abc", sign(signKey, "abc", okBody), okBody),
		"missing sig":      do(ts, "", okBody),
	} {
		if c != 403 {
			t.Errorf("%s must be 403, got %d", name, c)
		}
	}
	for _, rj := range r.rejects {
		if rj != "bad_signature" {
			t.Errorf("unexpected reject reason %q; forgeries must be reported as bad_signature", rj)
		}
	}
	if len(r.rejects) != 6 {
		t.Errorf("every forged request must be reported, got %v", r.rejects)
	}
}

func TestWithoutSigningSecretUnsignedCallbacksStillWork(t *testing.T) {
	r := newRig(t, nil)
	if rec := r.post(okPath, "application/json", "127.0.0.1:5000", okBody); rec.Code != 200 {
		t.Fatalf("backward compatible: %d", rec.Code)
	}
}
