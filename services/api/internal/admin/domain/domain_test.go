package domain

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordPolicyRejectsDefaultEmptyAndWeak(t *testing.T) {
	bad := map[string]string{
		"empty":                  "",
		"default admin":          "admin",
		"admin123":               "admin123",
		"password":               "password",
		"too short":              "Ab1!xyz",
		"11 chars":               "Abcdef1!ghi",
		"only lower":             "abcdefghijklmnop",
		"only digits":            "123456789012345",
		"two classes only":       "abcdefghij12345",
		"common with suffix":     "Password123!",
		"contains username":      "xx-superowner-9!A",
		"one repeated character": strings.Repeat("a", 20),
		"too long":               "Aa1!" + strings.Repeat("x", 130),
		"product name":           "veyrogram123",
	}
	for name, pw := range bad {
		if err := ValidatePassword("superowner", pw); err == nil {
			t.Errorf("%s: %q must be rejected", name, pw)
		}
	}
	good := []string{"correct-Horse-battery-7", "Zq9!mW3#pL8@vR2$", "long passphrase with Spaces 42"}
	for _, pw := range good {
		if err := ValidatePassword("superowner", pw); err != nil {
			t.Errorf("%q must pass: %v", pw, err)
		}
	}
	// every broken rule is reported together
	err := ValidatePassword("admin", "admin")
	we, ok := err.(*WeakPasswordError)
	if !ok || len(we.Problems) < 3 {
		t.Fatalf("expected several problems: %v", err)
	}
}

// RFC 6238 Appendix B test vectors (SHA-1 secret "12345678901234567890"; the RFC prints 8 digits, the last 6 are what a 6-digit authenticator shows).
func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1111111111: "050471", 1234567890: "005924", 2000000000: "279037"} {
		if got := TOTPCode(secret, time.Unix(ts, 0)); got != want {
			t.Errorf("t=%d: %s, want %s", ts, got, want)
		}
	}
}

func TestVerifyTOTPWindowAndReplay(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111109, 0)
	cur := uint64(now.Unix()) / 30
	if step, ok := VerifyTOTP(secret, TOTPCode(secret, now), now, 0); !ok || step != cur {
		t.Fatalf("current code: %d %v", step, ok)
	}
	for _, d := range []time.Duration{-30 * time.Second, 30 * time.Second} {
		if _, ok := VerifyTOTP(secret, TOTPCode(secret, now.Add(d)), now, 0); !ok {
			t.Errorf("one step of clock drift (%v) must be accepted", d)
		}
	}
	if _, ok := VerifyTOTP(secret, TOTPCode(secret, now.Add(90*time.Second)), now, 0); ok {
		t.Error("three steps away must be refused")
	}
	if _, ok := VerifyTOTP(secret, TOTPCode(secret, now), now, cur); ok {
		t.Error("a code whose step was already used must be refused (replay)")
	}
	for _, c := range []string{"", "12345", "1234567", "abcdef", "000000"} {
		if _, ok := VerifyTOTP(secret, c, now, 0); ok && c != TOTPCode(secret, now) {
			// 000000 could collide by chance with one of three windows; only fail on shape errors
			if len(c) != 6 || c != "000000" {
				t.Errorf("%q must be refused", c)
			}
		}
	}
}

func TestOTPAuthURIAndSecretText(t *testing.T) {
	u := OTPAuthURI("VeyroGram", "owner@x", []byte("12345678901234567890"))
	for _, want := range []string{"otpauth://totp/VeyroGram:owner@x?", "secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "issuer=VeyroGram", "digits=6", "period=30"} {
		if !strings.Contains(u, want) && !strings.Contains(u, strings.ReplaceAll(want, "@", "%40")) {
			t.Errorf("missing %q in %s", want, u)
		}
	}
}

func TestUsernameAndPermissions(t *testing.T) {
	for s, ok := range map[string]bool{"owner": true, "a.b-c_d": true, "ab": false, "a b": false, "a@b": false, strings.Repeat("a", 65): false} {
		if ValidUsername(s) != ok {
			t.Errorf("ValidUsername(%q) != %v", s, ok)
		}
	}
	if !KnownPermission(PermAuditRead) || KnownPermission("audit:delete") || KnownPermission("") {
		t.Error("permission vocabulary is closed")
	}
}
