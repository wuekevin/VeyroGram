// Package httpx holds small HTTP helpers shared by all modules: request IDs and the standard error envelope (AD-8).
package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type ctxKey struct{}

// RequestID assigns an X-Request-Id (generated unless a well-formed one is supplied) and echoes it on the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validID(id) {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-Id", id)
		r.Header.Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

func validID(s string) bool {
	if len(s) < 8 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' || c == '-') {
			return false
		}
	}
	return true
}

// Error is the standard error envelope: {code, message, trace_id, details}.
type Error struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	TraceID string   `json:"trace_id"`
	Details []string `json:"details,omitempty"`
}

// WriteError writes the envelope with the given HTTP status.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Error{Code: code, Message: message, TraceID: r.Header.Get("X-Request-Id")})
}

// WriteErrorDetails is WriteError plus a list of human-readable details (for example every password rule that was broken).
func WriteErrorDetails(w http.ResponseWriter, r *http.Request, status int, code, message string, details []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Error{Code: code, Message: message, TraceID: r.Header.Get("X-Request-Id"), Details: details})
}

// NotFound is the catch-all handler for the public mux until modules register routes.
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusNotFound, "not_found", "no such resource")
}
