// Package domain holds the conversation module's rules: sync cursors, limits and what a draft or setting may contain (Story 1.16).
package domain

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	ChannelPerson uint8 = 1
	// SystemUID is the account-notice sender; its conversation is never listed to clients. Must equal identity's SystemUID (pinned by a test in cmd/api).
	SystemUID = "vg_system"

	DefaultConvLimit = 50
	MaxConvLimit     = 100
	DefaultRecents   = 1
	MaxRecents       = 20
	DefaultMsgLimit  = 50
	MaxMsgLimit      = 200
	DefaultStateLim  = 200
	MaxStateLimit    = 500
	MaxDraftRunes    = 4000
)

var (
	ErrInvalidInput  = errors.New("conversation: invalid input")
	ErrNotFound      = errors.New("conversation: not found")
	ErrIMUnavailable = errors.New("conversation: im unavailable")
)

// ValidUID mirrors the uid alphabet the identity module issues.
func ValidUID(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// A conversation-list cursor is opaque to clients and versioned, so the paging scheme can change without breaking old clients: they send back what
// they were given and an unknown version is a clean 400 telling them to restart from the beginning.
const cursorPrefix = "c1."

func EncodeConvCursor(page int) string { return cursorPrefix + strconv.Itoa(page) }

// DecodeConvCursor returns the one-based page; an empty cursor means the first page.
func DecodeConvCursor(c string) (int, error) {
	if c == "" {
		return 1, nil
	}
	rest, ok := strings.CutPrefix(c, cursorPrefix)
	if !ok {
		return 0, ErrInvalidInput
	}
	p, err := strconv.Atoi(rest)
	if err != nil || p < 1 || p > 1000 {
		return 0, ErrInvalidInput
	}
	return p, nil
}

// ValidDraft checks a draft body. Empty content means "no draft" (a deletion), which is always valid.
func ValidDraft(content string) bool {
	return utf8.ValidString(content) && utf8.RuneCountInString(content) <= MaxDraftRunes
}

// Clamp returns def when v <= 0 and max when v exceeds it.
func Clamp(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}
