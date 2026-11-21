// Package app implements the block list use cases.
package app

import (
	"context"
	"time"

	"veyrogram/api/internal/contact/domain"
	"veyrogram/api/internal/contact/ports"
)

type Service struct {
	Store    ports.Store
	Accounts ports.Accounts
	Tenant   uint64
	Now      func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Block adds target to the caller's block list. Blocking yourself or an unknown/disabled account is refused; blocking twice is fine.
func (s *Service) Block(ctx context.Context, user, target string) error {
	if !domain.ValidUID(target) || target == user {
		return domain.ErrInvalidInput
	}
	m, err := s.Accounts.Active(ctx, []string{target})
	if err != nil {
		return err
	}
	if !m[target] {
		return domain.ErrNotFound
	}
	full, err := s.Store.Add(ctx, s.Tenant, user, target, domain.MaxBlocks, s.now())
	if err != nil {
		return err
	}
	if full {
		return domain.ErrTooMany
	}
	return nil
}

// Unblock removes target from the caller's list; removing someone who is not blocked is fine (idempotent).
func (s *Service) Unblock(ctx context.Context, user, target string) error {
	if !domain.ValidUID(target) {
		return domain.ErrInvalidInput
	}
	return s.Store.Remove(ctx, s.Tenant, user, target)
}

func (s *Service) List(ctx context.Context, user string) ([]ports.Block, error) {
	return s.Store.List(ctx, s.Tenant, user, domain.MaxBlocks)
}
