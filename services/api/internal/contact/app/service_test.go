package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"veyrogram/api/internal/contact/domain"
	"veyrogram/api/internal/contact/ports"
)

type store struct {
	n    int
	full bool
	adds []string
}

func (s *store) Add(_ context.Context, _ uint64, _, blocked string, _ int, _ time.Time) (bool, error) {
	s.adds = append(s.adds, blocked)
	return s.full, nil
}
func (s *store) Remove(context.Context, uint64, string, string) error             { return nil }
func (s *store) List(context.Context, uint64, string, int) ([]ports.Block, error) { return nil, nil }
func (s *store) BlockedBy(context.Context, uint64, string, string) (bool, error)  { return false, nil }

type accts map[string]bool

func (a accts) Active(_ context.Context, uids []string) (map[string]bool, error) { return a, nil }

func TestBlockValidation(t *testing.T) {
	st := &store{}
	s := &Service{Store: st, Accounts: accts{"bob": true}}
	ctx := context.Background()
	if err := s.Block(ctx, "alice", "bob"); err != nil || len(st.adds) != 1 {
		t.Fatalf("ok: %v %v", err, st.adds)
	}
	for _, c := range []struct {
		target string
		want   error
	}{{"alice", domain.ErrInvalidInput}, {"a b", domain.ErrInvalidInput}, {"", domain.ErrInvalidInput}, {"ghost", domain.ErrNotFound}} {
		if err := s.Block(ctx, "alice", c.target); !errors.Is(err, c.want) {
			t.Errorf("%q: %v, want %v", c.target, err, c.want)
		}
	}
	st.full = true
	if err := s.Block(ctx, "alice", "bob"); !errors.Is(err, domain.ErrTooMany) {
		t.Fatalf("full list: %v", err)
	}
}
