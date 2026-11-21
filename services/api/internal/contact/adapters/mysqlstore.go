// Package adapters implements the contact module's ports.
package adapters

import (
	"context"
	"database/sql"
	"time"

	"veyrogram/api/internal/contact/ports"
)

type MySQLStore struct{ DB *sql.DB }

func t1(t uint64) uint64 {
	if t == 0 {
		return 1
	}
	return t
}

func (s *MySQLStore) Add(ctx context.Context, tenant uint64, user, blocked string, max int, now time.Time) (bool, error) {
	tenant = t1(tenant)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_block WHERE tenant_id=? AND user_uid=? AND blocked_uid=? FOR UPDATE", tenant, user, blocked).Scan(&exists); err != nil {
		return false, err
	}
	if exists == 0 {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_block WHERE tenant_id=? AND user_uid=? FOR UPDATE", tenant, user).Scan(&n); err != nil {
			return false, err
		}
		if n >= max {
			return true, nil
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO user_block (tenant_id, user_uid, blocked_uid, created_at) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE created_at=created_at", tenant, user, blocked, now); err != nil {
			return false, err
		}
	}
	return false, tx.Commit()
}

func (s *MySQLStore) Remove(ctx context.Context, tenant uint64, user, blocked string) error {
	_, err := s.DB.ExecContext(ctx, "DELETE FROM user_block WHERE tenant_id=? AND user_uid=? AND blocked_uid=?", t1(tenant), user, blocked)
	return err
}

func (s *MySQLStore) List(ctx context.Context, tenant uint64, user string, limit int) ([]ports.Block, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT blocked_uid, created_at FROM user_block WHERE tenant_id=? AND user_uid=? ORDER BY created_at DESC, blocked_uid LIMIT ?", t1(tenant), user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.Block
	for rows.Next() {
		var b ports.Block
		if err := rows.Scan(&b.UID, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *MySQLStore) BlockedBy(ctx context.Context, tenant uint64, blocker, sender string) (bool, error) {
	var one int
	err := s.DB.QueryRowContext(ctx, "SELECT 1 FROM user_block WHERE tenant_id=? AND user_uid=? AND blocked_uid=?", t1(tenant), blocker, sender).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
