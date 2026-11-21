package adapters

import (
	"strings"
	"testing"
)

func TestArgon2idRoundTripAndTampering(t *testing.T) {
	a := Argon2id{Memory: 8 * 1024, Time: 1, Threads: 1} // cheap parameters for the test; the hash records them
	h, err := a.Hash("correct-Horse-battery-7")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("hash: %v %q", err, h)
	}
	if h2, _ := a.Hash("correct-Horse-battery-7"); h2 == h {
		t.Fatal("salts must differ")
	}
	if !a.Verify("correct-Horse-battery-7", h) || a.Verify("wrong", h) || a.Verify("", h) {
		t.Fatal("verify")
	}
	// a hash made with other (stronger) parameters still verifies: parameters live in the hash
	if !DefaultArgon2id().Verify("x-Y-z-123-ABC-def", mustHash(t, DefaultArgon2id(), "x-Y-z-123-ABC-def")) {
		t.Fatal("default parameters")
	}
	for _, bad := range []string{"", "plain", "$argon2id$v=19$m=8,t=1,p=1$AA$AA", "$bcrypt$x$y$z$w$v", "$argon2id$v=19$m=99999999,t=1,p=1$AAAA$AAAA", strings.Replace(h, "$m=", "$m=9", 1)} {
		if a.Verify("correct-Horse-battery-7", bad) {
			t.Errorf("malformed or tampered hash %q must not verify", bad)
		}
	}
}

func mustHash(t *testing.T, a Argon2id, pw string) string {
	h, err := a.Hash(pw)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSecretBoxSealsAndRejectsTampering(t *testing.T) {
	b, err := NewSecretBox(strings.Repeat("k", 40))
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := b.Seal([]byte("totp-secret"))
	if got, err := b.Open(enc); err != nil || string(got) != "totp-secret" {
		t.Fatalf("open: %v %q", err, got)
	}
	if enc2, _ := b.Seal([]byte("totp-secret")); string(enc2) == string(enc) {
		t.Fatal("nonces must differ")
	}
	enc[len(enc)-1] ^= 1
	if _, err := b.Open(enc); err == nil {
		t.Fatal("tampering must be detected")
	}
	if _, err := b.Open([]byte("short")); err == nil {
		t.Fatal("short input")
	}
	other, _ := NewSecretBox(strings.Repeat("z", 40))
	good, _ := b.Seal([]byte("x"))
	if _, err := other.Open(good); err == nil {
		t.Fatal("a different key must not open it")
	}
	if _, err := NewSecretBox("short"); err == nil {
		t.Fatal("a short key must be refused")
	}
}
