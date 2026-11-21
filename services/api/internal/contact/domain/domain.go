// Package domain holds the contact module's rules. Today that is the block list (Story 1.15 follow-up); friends/contacts arrive with their own story.
package domain

import "errors"

const MaxBlocks = 1000

var (
	ErrInvalidInput = errors.New("contact: invalid input")
	ErrNotFound     = errors.New("contact: no such account")
	ErrTooMany      = errors.New("contact: block list is full")
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
