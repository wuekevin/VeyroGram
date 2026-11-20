package outbox

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestInTxRetriesOnlyDeadlockAndLockWait(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		calls int
		fails bool
	}{
		{"deadlock then ok", &mysql.MySQLError{Number: 1213}, 3, false},
		{"lock wait then ok", fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1205}), 2, false},
		{"other mysql error not retried", &mysql.MySQLError{Number: 1062}, 1, true},
		{"plain error not retried", errors.New("boom"), 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := 0
			_, err := inTx(context.Background(), func() (int, error) {
				n++
				if n < tc.calls || tc.fails {
					return 0, tc.err
				}
				return 1, nil
			})
			if (err != nil) != tc.fails || n != tc.calls {
				t.Fatalf("err=%v calls=%d, want fails=%v calls=%d", err, n, tc.fails, tc.calls)
			}
		})
	}
}

func TestInTxGivesUpAfterBoundedAttempts(t *testing.T) {
	n := 0
	_, err := inTx(context.Background(), func() (int, error) { n++; return 0, &mysql.MySQLError{Number: 1213} })
	if err == nil || n != 5 {
		t.Fatalf("err=%v calls=%d, want an error after exactly 5 attempts", err, n)
	}
}
