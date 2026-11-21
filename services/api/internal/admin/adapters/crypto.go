// Package adapters implements the admin module's ports.
package adapters

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id with the RFC 9106 "second recommended" profile (64 MiB, 3 passes) scaled for an interactive admin login. Hashes are self-describing, so the cost can
// be raised later and old hashes still verify.
type Argon2id struct {
	Memory, Time uint32
	Threads      uint8
}

func DefaultArgon2id() Argon2id { return Argon2id{Memory: 64 * 1024, Time: 3, Threads: 2} }

func (a Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, a.Time, a.Memory, a.Threads, 32)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, a.Memory, a.Time, a.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func (a Argon2id) Verify(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var v int
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m > 1<<20 || t > 20 || p == 0 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// DummyVerify burns the same time as a real verification, so an unknown username costs the same as a wrong password (no user enumeration by timing).
func (a Argon2id) DummyVerify(password string) {
	salt := make([]byte, 16)
	_ = argon2.IDKey([]byte(password), salt, a.Time, a.Memory, a.Threads, 32)
}

// SecretBox is AES-256-GCM keyed from a configured secret (SHA-256 of it), used for the TOTP secrets at rest.
type SecretBox struct{ aead cipher.AEAD }

func NewSecretBox(secret string) (*SecretBox, error) {
	if len(secret) < 32 {
		return nil, errors.New("admin: the TOTP encryption key must be at least 32 characters")
	}
	k := sha256.Sum256([]byte("veyrogram admin totp v1\x00" + secret))
	b, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(b)
	return &SecretBox{aead: g}, err
}

func (s *SecretBox) Seal(plain []byte) ([]byte, error) {
	n := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(n); err != nil {
		return nil, err
	}
	return s.aead.Seal(n, n, plain, nil), nil
}

func (s *SecretBox) Open(sealed []byte) ([]byte, error) {
	if len(sealed) < s.aead.NonceSize() {
		return nil, errors.New("admin: sealed value too short")
	}
	return s.aead.Open(nil, sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():], nil)
}
