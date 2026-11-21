// Package domain holds the identity module's pure rules: phone/device validation, policy thresholds and the error vocabulary.
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Phone is an E.164 number ("+" and 8-15 digits, first digit non-zero). Clients must send E.164; no region is guessed server-side.
type Phone string

var phoneRE = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// ParsePhone removes spaces and dashes and validates E.164.
func ParsePhone(s string) (Phone, error) {
	s = strings.NewReplacer(" ", "", "-", "").Replace(s)
	if !phoneRE.MatchString(s) {
		return "", ErrInvalidInput
	}
	return Phone(s), nil
}

// Last4 is the only part of the number kept in readable form.
func (p Phone) Last4() string { return string(p)[len(p)-4:] }

type DeviceClass string

const (
	ClassApp DeviceClass = "app"
	ClassWeb DeviceClass = "web"
	ClassPC  DeviceClass = "pc"
)

// Device is what a client presents at login: a stable random key it generated, a class and a display name.
type Device struct {
	Key   string
	Class DeviceClass
	Name  string
}

var deviceKeyRE = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// ParseDevice validates the client-supplied device descriptor and strips control characters from the name.
func ParseDevice(key, class, name string) (Device, error) {
	c := DeviceClass(class)
	if !deviceKeyRE.MatchString(key) || (c != ClassApp && c != ClassWeb && c != ClassPC) {
		return Device{}, ErrInvalidInput
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
	}
	return Device{Key: key, Class: c, Name: name}, nil
}

// ValidDeviceKey reports whether s is acceptable as a device key (used where only the key is sent, e.g. code requests).
func ValidDeviceKey(s string) bool { return deviceKeyRE.MatchString(s) }

// Policy holds every tunable threshold (story 1.12 "限流阈值"). Counters are per rolling window and live in login_event.
type Policy struct {
	CodeTTL         time.Duration
	MaxCodeAttempts int // wrong guesses against one code before it is dead
	SendCooldown    time.Duration
	SendWindow      time.Duration
	SendPerPhone    int
	SendPerIP       int
	SendPerDevice   int
	SendGlobal      int // circuit breaker against SMS pumping across many numbers
	FailWindow      time.Duration
	FailPerPhone    int
	FailPerIP       int
	FailPerDevice   int
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	MessageTemplate string // %s = code, %d = minutes
}

func DefaultPolicy() Policy {
	return Policy{
		CodeTTL: 5 * time.Minute, MaxCodeAttempts: 5,
		SendCooldown: 60 * time.Second, SendWindow: time.Hour,
		SendPerPhone: 5, SendPerIP: 20, SendPerDevice: 10, SendGlobal: 2000,
		FailWindow: 15 * time.Minute, FailPerPhone: 5, FailPerIP: 30, FailPerDevice: 10,
		AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour,
		MessageTemplate: "【VeyroGram】验证码 %s，%d 分钟内有效。请勿告诉他人。",
	}
}

var (
	ErrInvalidInput   = errors.New("identity: invalid input")
	ErrInvalidCode    = errors.New("identity: invalid or expired code")
	ErrInvalidRefresh = errors.New("identity: invalid refresh token")
	ErrDisabled       = errors.New("identity: account disabled")
	ErrSMSUnavailable = errors.New("identity: sms delivery unavailable")
	ErrCodeConsumed   = errors.New("identity: code already used")
)

// LimitError is returned when a rate limit or lockout applies; RetryAfter is an upper bound.
type LimitError struct {
	Locked     bool // true = failed-attempt lockout, false = send limit
	RetryAfter time.Duration
}

func (e *LimitError) Error() string { return "identity: rate limited" }
