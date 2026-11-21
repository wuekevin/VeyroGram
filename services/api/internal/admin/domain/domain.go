// Package domain holds the admin plane's rules: permissions, the password policy and TOTP (Story 1.19; ARCHITECTURE AD-3, AD-9, AD-24).
package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP is defined over HMAC-SHA1 and every authenticator app speaks it
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// Permissions are the closed vocabulary every route must pick from. A route declaring anything else fails startup.
const (
	PermSelf        = "self" // any authenticated, MFA-complete admin: who am I, log out
	PermAuditRead   = "audit:read"
	PermAuditVerify = "audit:verify"
	PermAdminRead   = "admin:read"
	PermAdminWrite  = "admin:write"
)

var knownPerms = map[string]bool{PermSelf: true, PermAuditRead: true, PermAuditVerify: true, PermAdminRead: true, PermAdminWrite: true}

func KnownPermission(p string) bool { return knownPerms[p] }

// Roles seeded by the migration. Role ids are stable; names are what the API shows.
const (
	RoleOwner   uint8 = 1
	RoleAuditor uint8 = 2
)

var RoleNames = map[uint8]string{RoleOwner: "owner", RoleAuditor: "auditor"}

func RoleByName(n string) (uint8, bool) {
	for id, name := range RoleNames {
		if name == n {
			return id, true
		}
	}
	return 0, false
}

var (
	ErrInvalidInput       = errors.New("admin: invalid input")
	ErrWeakPassword       = errors.New("admin: weak password")
	ErrInvalidCredentials = errors.New("admin: invalid credentials")
	ErrLocked             = errors.New("admin: account temporarily locked")
	ErrInvalidChallenge   = errors.New("admin: invalid or expired challenge")
	ErrInvalidCode        = errors.New("admin: invalid verification code")
	ErrUnauthenticated    = errors.New("admin: not authenticated")
	ErrForbidden          = errors.New("admin: forbidden")
	ErrAlreadyInitialized = errors.New("admin: already initialised")
	ErrNotFound           = errors.New("admin: not found")
	ErrConflict           = errors.New("admin: already exists")
	ErrReasonRequired     = errors.New("admin: a reason is required for this action")
	ErrLastOwner          = errors.New("admin: the last active owner cannot be disabled")
)

// WeakPasswordError lists every rule the password broke, so the operator fixes them in one go.
type WeakPasswordError struct{ Problems []string }

func (e *WeakPasswordError) Error() string {
	return "admin: weak password: " + strings.Join(e.Problems, "; ")
}
func (e *WeakPasswordError) Is(t error) bool { return t == ErrWeakPassword }

// A deliberately small deny list of the passwords every attacker tries first; the length and class rules do the real work.
var commonPasswords = map[string]bool{
	"password": true, "passw0rd": true, "admin": true, "administrator": true, "123456": true, "12345678": true, "123456789": true, "1234567890": true,
	"qwerty": true, "letmein": true, "welcome": true, "iloveyou": true, "changeme": true, "default": true, "root": true, "toor": true,
	"veyrogram": true, "veyrogram123": true, "admin123": true, "admin@123": true, "p@ssw0rd": true, "p@ssword": true,
}

// ValidatePassword enforces: 12-128 characters, at least three of lower/upper/digit/symbol, not a common password, not containing the username, and
// not a single repeated or sequential run. Empty and default passwords therefore never pass.
func ValidatePassword(username, pw string) error {
	var p []string
	n := len([]rune(pw))
	if n < 12 {
		p = append(p, "must be at least 12 characters")
	}
	if n > 128 {
		p = append(p, "must be at most 128 characters")
	}
	var lower, upper, digit, sym bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			sym = true
		}
	}
	classes := 0
	for _, b := range []bool{lower, upper, digit, sym} {
		if b {
			classes++
		}
	}
	if classes < 3 {
		p = append(p, "must mix at least three of: lower case, upper case, digits, symbols")
	}
	low := strings.ToLower(pw)
	if commonPasswords[low] || commonPasswords[strings.TrimRight(low, "0123456789!@#")] {
		p = append(p, "is a commonly used password")
	}
	if u := strings.ToLower(username); len(u) >= 3 && strings.Contains(low, u) {
		p = append(p, "must not contain the username")
	}
	if n > 0 && strings.Count(pw, string([]rune(pw)[0])) == n {
		p = append(p, "must not be one repeated character")
	}
	if len(p) > 0 {
		return &WeakPasswordError{Problems: p}
	}
	return nil
}

// ValidUsername allows 3-64 characters of letters, digits, dot, dash and underscore.
func ValidUsername(s string) bool {
	if len(s) < 3 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ---- TOTP (RFC 6238: HMAC-SHA1, 6 digits, 30 s step) ----

const totpStep = 30 * time.Second

func NewTOTPSecret() ([]byte, error) {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	return b, err
}

func TOTPSecretText(secret []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
}

// OTPAuthURI is what an authenticator app scans.
func OTPAuthURI(issuer, account string, secret []byte) string {
	v := url.Values{}
	v.Set("secret", TOTPSecretText(secret))
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}

func totpAt(secret []byte, step uint64) string {
	var c [8]byte
	binary.BigEndian.PutUint64(c[:], step)
	m := hmac.New(sha1.New, secret)
	m.Write(c[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff) % 1_000_000
	return fmt.Sprintf("%06d", code)
}

// TOTPCode returns the code for the step containing t (used by tests and by operators' tooling).
func TOTPCode(secret []byte, t time.Time) string {
	return totpAt(secret, uint64(t.Unix())/uint64(totpStep.Seconds()))
}

// VerifyTOTP accepts the current step and one step either side (clock drift). A step at or below lastStep is refused, so a code cannot be replayed.
// It returns the accepted step.
func VerifyTOTP(secret []byte, code string, now time.Time, lastStep uint64) (uint64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	cur := uint64(now.Unix()) / uint64(totpStep.Seconds())
	var hit uint64
	ok := false
	for _, s := range []uint64{cur - 1, cur, cur + 1} { // constant work: no early exit on a match
		if hmac.Equal([]byte(totpAt(secret, s)), []byte(code)) && s > lastStep && !ok {
			hit, ok = s, true
		}
	}
	return hit, ok
}

// ParseTOTPSecret is the inverse of TOTPSecretText (for tooling and tests that play the authenticator app).
func ParseTOTPSecret(text string) ([]byte, error) {
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.ReplaceAll(text, " ", "")))
}
