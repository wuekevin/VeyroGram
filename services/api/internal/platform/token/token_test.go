package token

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func seed(b byte) string {
	s := make([]byte, 32)
	for i := range s {
		s[i] = b + byte(i)
	}
	return base64.RawURLEncoding.EncodeToString(s)
}

func TestSignVerifyAndRotation(t *testing.T) {
	old, err := ParseRing("k1:" + seed(1))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	tok, err := old.Sign(Claims{Subject: "u1", Device: "7", Class: "app", Tenant: 1, ID: "j", IssuedAt: now.Unix(), Expires: now.Add(15 * time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := old.Verify(tok, now)
	if err != nil || c.Subject != "u1" || c.Device != "7" || c.Issuer != "veyrogram" {
		t.Fatalf("verify: %+v %v", c, err)
	}
	// rotation: new key signs, old key still verifies tokens already issued
	rot, _ := ParseRing("k2:" + seed(50) + ",k1:" + seed(1))
	if _, err := rot.Verify(tok, now); err != nil {
		t.Fatalf("old token must verify after rotation: %v", err)
	}
	nt, _ := rot.Sign(Claims{Subject: "u1", Expires: now.Add(time.Minute).Unix()})
	if !strings.Contains(decodeHeader(t, nt), `"kid":"k2"`) {
		t.Fatal("new tokens must use the first key")
	}
	if _, err := old.Verify(nt, now); err == nil {
		t.Fatal("a ring without k2 must reject tokens signed by k2")
	}
	// retiring k1 invalidates its tokens
	retired, _ := ParseRing("k2:" + seed(50))
	if _, err := retired.Verify(tok, now); err == nil {
		t.Fatal("retired key must stop verifying")
	}
	if _, err := old.Verify(tok, now.Add(16*time.Minute)); err != ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func decodeHeader(t *testing.T, tok string) string {
	b, err := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVerifyRejectsForgeries(t *testing.T) {
	r, _ := ParseRing("k1:" + seed(1))
	now := time.Unix(1_700_000_000, 0)
	good, _ := r.Sign(Claims{Subject: "u1", Expires: now.Add(time.Minute).Unix()})
	parts := strings.Split(good, ".")
	enc := base64.RawURLEncoding
	none := enc.EncodeToString([]byte(`{"alg":"none","typ":"JWT","kid":"k1"}`)) + "." + parts[1] + "."
	hs := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT","kid":"k1"}`)) + "." + parts[1] + "." + parts[2]
	tampered := parts[0] + "." + enc.EncodeToString([]byte(`{"sub":"admin","exp":9999999999,"iss":"veyrogram"}`)) + "." + parts[2]
	other, _ := ParseRing("k1:" + seed(9)) // same kid, different key
	foreign, _ := other.Sign(Claims{Subject: "u1", Expires: now.Add(time.Minute).Unix()})
	for name, tok := range map[string]string{"alg none": none, "alg swap": hs, "tampered payload": tampered, "other key same kid": foreign, "garbage": "a.b", "empty": ""} {
		if _, err := r.Verify(tok, now); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
	_ = ed25519.SeedSize
}

func TestParseRingRejectsBadInput(t *testing.T) {
	for _, s := range []string{"", "k1", "k1:short", "k1:" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "k1:" + seed(1) + ",k1:" + seed(2), "a.b:" + seed(1)} {
		if _, err := ParseRing(s); err == nil {
			t.Errorf("%q must be rejected", s)
		}
	}
}
