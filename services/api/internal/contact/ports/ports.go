// Package ports defines what the contact module needs from the outside world.
package ports

import (
	"context"
	"time"
)

type Block struct {
	UID       string
	CreatedAt time.Time
}

// Accounts tells whether an account exists and is enabled; implemented by the composition root on top of the identity module (AD-1).
type Accounts interface {
	Active(ctx context.Context, uids []string) (map[string]bool, error)
}

type Store interface {
	// Add blocks blocked on behalf of user. Blocking twice is a no-op. It returns ErrTooMany semantics via the second result: full=true when the user already
	// holds max blocks and this one is new.
	Add(ctx context.Context, tenant uint64, user, blocked string, max int, now time.Time) (full bool, err error)
	Remove(ctx context.Context, tenant uint64, user, blocked string) error
	List(ctx context.Context, tenant uint64, user string, limit int) ([]Block, error)
	// BlockedBy reports whether blocker has blocked sender (the question the send gate asks).
	BlockedBy(ctx context.Context, tenant uint64, blocker, sender string) (bool, error)
}
