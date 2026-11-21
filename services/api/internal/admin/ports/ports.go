// Package ports defines what the admin module needs from the outside world.
package ports

import (
	"context"
	"time"
)

type Admin struct {
	ID             uint64
	Username       string
	PasswordHash   string
	RoleID         uint8
	Active         bool
	TOTPSecretEnc  []byte
	Enrolled       bool
	TOTPLastStep   uint64
	FailedAttempts int
	LockedUntil    time.Time // zero = not locked
	CreatedAt      time.Time
}

type Principal struct {
	AdminID   uint64
	Username  string
	RoleID    uint8
	SessionID uint64
	Perms     map[string]bool
}

type AuditRow struct {
	ID        uint64
	Actor     string
	Action    string
	Target    string
	Detail    string
	TraceID   string
	CreatedAt time.Time
}

type AuditFilter struct {
	Actor, Action string
	BeforeID      uint64 // cursor: rows with id < BeforeID (0 = newest)
	Limit         int
}

// Hasher hashes and checks passwords (Argon2id).
type Hasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
	// DummyVerify costs as much as Verify, for unknown usernames (no user enumeration by timing).
	DummyVerify(password string)
}

// Box encrypts the TOTP secret at rest.
type Box interface {
	Seal(plain []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

type Store interface {
	// Bootstrap creates the first admin (role owner) and the 'initialised' marker in one transaction; ErrAlreadyInitialized (as domain error) if the marker exists.
	Bootstrap(ctx context.Context, tenant uint64, username, passwordHash string, secretEnc []byte, now time.Time) (adminID uint64, err error)
	Initialized(ctx context.Context, tenant uint64) (bool, error)
	AdminByUsername(ctx context.Context, tenant uint64, username string) (Admin, bool, error)
	AdminByID(ctx context.Context, tenant uint64, id uint64) (Admin, bool, error)
	ListAdmins(ctx context.Context, tenant uint64) ([]Admin, error)
	CreateAdmin(ctx context.Context, tenant uint64, username, passwordHash string, roleID uint8, secretEnc []byte, createdBy uint64, now time.Time) (uint64, error)
	// DisableAdmin disables the admin and revokes all their sessions. It refuses (domain.ErrLastOwner) to disable the last active owner.
	DisableAdmin(ctx context.Context, tenant uint64, id uint64, now time.Time) error

	RecordFailure(ctx context.Context, id uint64, maxAttempts int, lockFor time.Duration, now time.Time) (locked bool, err error)
	ResetFailures(ctx context.Context, id uint64) error
	// ConfirmStep records the accepted TOTP step (replay guard) and, for first-time enrolment, the enrolment time.
	ConfirmStep(ctx context.Context, id uint64, step uint64, enroll bool, now time.Time) error

	NewChallenge(ctx context.Context, adminID uint64, kind, tokenHash string, expires time.Time) error
	// ConsumeChallenge marks the challenge used and returns its admin. A second call with the same token fails.
	ConsumeChallenge(ctx context.Context, kind, tokenHash string, now time.Time) (adminID uint64, ok bool, err error)

	NewSession(ctx context.Context, adminID uint64, tokenHash, ip string, now, expires time.Time) error
	// Authenticate resolves a live session of an active admin and its permissions, and refreshes last_seen_at. Unknown, expired, revoked sessions and
	// disabled or not-yet-enrolled admins all resolve to ok=false.
	Authenticate(ctx context.Context, tokenHash string, now time.Time, idle time.Duration) (Principal, bool, error)
	RevokeSession(ctx context.Context, sessionID uint64, now time.Time) error

	ListAudit(ctx context.Context, tenant uint64, f AuditFilter) ([]AuditRow, error)
}
