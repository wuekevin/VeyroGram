// Package idempotency implements Idempotency-Key semantics for unsafe HTTP methods (ARCHITECTURE AD-8):
// a repeated identical request within the TTL replays the first response; the same key with a different request is rejected (422);
// a request still in flight answers 409; 5xx responses are never cached so the client can retry.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/go-sql-driver/mysql"

	"veyrogram/api/internal/platform/httpx"
)

// Outcome is the result of Begin.
type Outcome int

const (
	New        Outcome = iota // caller must run the handler and then Complete (or Abort)
	Replay                    // a completed response exists for this exact request
	InProgress                // another request with this key is still running
	Mismatch                  // the key was used with a different request
)

// Result is a stored response.
type Result struct {
	Status      int
	ContentType string
	Body        []byte
}

// Store persists keys in MySQL (table idempotency_key).
type Store struct {
	DB          *sql.DB
	TTL         time.Duration    // default 24h
	StaleAfter  time.Duration    // an in_progress row older than this is considered abandoned (default 2m)
	Now         func() time.Time // default time.Now
	MaxBodySize int              // stored response limit (default 1 MiB)
}

func (s *Store) ttl() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return 24 * time.Hour
}
func (s *Store) stale() time.Duration {
	if s.StaleAfter > 0 {
		return s.StaleAfter
	}
	return 2 * time.Minute
}
func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Store) maxBody() int {
	if s.MaxBodySize > 0 {
		return s.MaxBodySize
	}
	return 1 << 20
}

// Begin claims the key or reports what already exists.
func (s *Store) Begin(ctx context.Context, tenant uint64, scope string, key, req [32]byte) (Outcome, *Result, error) {
	now := s.now()
	// drop an expired or abandoned row for exactly this key, then try to claim it
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM idempotency_key WHERE tenant_id=? AND scope=? AND key_hash=?
		AND (expires_at <= ? OR (state='in_progress' AND created_at <= ?))`, tenant, scope, key[:], now, now.Add(-s.stale())); err != nil {
		return 0, nil, err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO idempotency_key (tenant_id, scope, key_hash, request_hash, state, created_at, expires_at)
		VALUES (?,?,?,?, 'in_progress', ?, ?)`, tenant, scope, key[:], req[:], now, now.Add(s.ttl()))
	if err == nil {
		return New, nil, nil
	}
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != 1062 {
		return 0, nil, err
	}
	var (
		reqHash []byte
		state   string
		status  sql.NullInt64
		ctype   sql.NullString
		body    []byte
	)
	if err := s.DB.QueryRowContext(ctx, `SELECT request_hash, state, response_status, response_ctype, response_body FROM idempotency_key
		WHERE tenant_id=? AND scope=? AND key_hash=?`, tenant, scope, key[:]).Scan(&reqHash, &state, &status, &ctype, &body); err != nil {
		if errors.Is(err, sql.ErrNoRows) { // deleted between insert and select: let the caller retry as a new request
			return s.Begin(ctx, tenant, scope, key, req)
		}
		return 0, nil, err
	}
	if !bytes.Equal(reqHash, req[:]) {
		return Mismatch, nil, nil
	}
	if state != "completed" {
		return InProgress, nil, nil
	}
	return Replay, &Result{Status: int(status.Int64), ContentType: ctype.String, Body: body}, nil
}

// Complete stores the response for replay.
func (s *Store) Complete(ctx context.Context, tenant uint64, scope string, key [32]byte, r Result) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE idempotency_key SET state='completed', response_status=?, response_ctype=?, response_body=?
		WHERE tenant_id=? AND scope=? AND key_hash=? AND state='in_progress'`, r.Status, r.ContentType, r.Body, tenant, scope, key[:])
	return err
}

// Abort releases an in-progress claim so the client may retry (used for 5xx, panics and oversized responses).
func (s *Store) Abort(ctx context.Context, tenant uint64, scope string, key [32]byte) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM idempotency_key WHERE tenant_id=? AND scope=? AND key_hash=? AND state='in_progress'`, tenant, scope, key[:])
	return err
}

// PurgeExpired deletes rows past their expiry; call it from a worker.
func (s *Store) PurgeExpired(ctx context.Context) (int64, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM idempotency_key WHERE expires_at <= ?`, s.now())
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

var keyRE = regexp.MustCompile(`^[A-Za-z0-9._:\-]{8,128}$`)

const maxRequestBody = 1 << 20

// Subject and Tenant let the caller bind keys to the authenticated principal and tenant (anonymous until auth lands).
type Subject func(*http.Request) string
type Tenant func(*http.Request) uint64

// Middleware applies idempotency to POST/PUT/PATCH/DELETE requests that carry an Idempotency-Key header.
func Middleware(s *Store, subject Subject, tenant Tenant) func(http.Handler) http.Handler {
	if subject == nil {
		subject = func(*http.Request) string { return "anon" }
	}
	if tenant == nil {
		tenant = func(*http.Request) uint64 { return 1 }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				next.ServeHTTP(w, r)
				return
			}
			raw := r.Header.Get("Idempotency-Key")
			if raw == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !keyRE.MatchString(raw) {
				httpx.WriteError(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 8-128 characters of [A-Za-z0-9._:-]")
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
			if err != nil {
				httpx.WriteError(w, r, http.StatusBadRequest, "bad_request", "could not read request body")
				return
			}
			if len(body) > maxRequestBody {
				httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large for an idempotent request")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			h := sha256.New()
			h.Write([]byte(r.Method + "\n" + r.URL.Path + "?" + r.URL.RawQuery + "\n"))
			h.Write(body)
			var req, key [32]byte
			copy(req[:], h.Sum(nil))
			key = sha256.Sum256([]byte(raw))
			tid, scope := tenant(r), r.Method+" "+r.URL.Path+"|"+subject(r)
			out, res, err := s.Begin(r.Context(), tid, scope, key, req)
			if err != nil {
				httpx.WriteError(w, r, http.StatusServiceUnavailable, "idempotency_unavailable", "could not verify idempotency; retry later")
				return
			}
			switch out {
			case Replay:
				if res.ContentType != "" {
					w.Header().Set("Content-Type", res.ContentType)
				}
				w.Header().Set("Idempotent-Replayed", "true")
				w.WriteHeader(res.Status)
				_, _ = w.Write(res.Body)
				return
			case InProgress:
				httpx.WriteError(w, r, http.StatusConflict, "idempotency_in_progress", "a request with this Idempotency-Key is still being processed")
				return
			case Mismatch:
				httpx.WriteError(w, r, http.StatusUnprocessableEntity, "idempotency_key_reused", "this Idempotency-Key was already used with a different request")
				return
			}
			rec := &recorder{ResponseWriter: w, status: http.StatusOK, limit: s.maxBody()}
			done := false
			defer func() {
				if done {
					return
				}
				_ = s.Abort(context.WithoutCancel(r.Context()), tid, scope, key) // panic in the handler: release the key
			}()
			next.ServeHTTP(rec, r)
			done = true
			ctx := context.WithoutCancel(r.Context())
			if rec.status >= 500 || rec.overflow {
				_ = s.Abort(ctx, tid, scope, key)
				return
			}
			_ = s.Complete(ctx, tid, scope, key, Result{Status: rec.status, ContentType: w.Header().Get("Content-Type"), Body: rec.buf.Bytes()})
		})
	}
}

type recorder struct {
	http.ResponseWriter
	status   int
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (r *recorder) WriteHeader(c int) { r.status = c; r.ResponseWriter.WriteHeader(c) }
func (r *recorder) Write(p []byte) (int, error) {
	if r.buf.Len()+len(p) > r.limit {
		r.overflow = true
	} else {
		r.buf.Write(p)
	}
	return r.ResponseWriter.Write(p)
}
