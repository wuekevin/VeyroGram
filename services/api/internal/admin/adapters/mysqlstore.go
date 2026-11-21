package adapters

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"veyrogram/api/internal/admin/domain"
	"veyrogram/api/internal/admin/ports"
)

type MySQLStore struct{ DB *sql.DB }

func t1(t uint64) uint64 {
	if t == 0 {
		return 1
	}
	return t
}

func (s *MySQLStore) Bootstrap(ctx context.Context, tenant uint64, username, hash string, secretEnc []byte, now time.Time) (uint64, error) {
	tenant = t1(tenant)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// the marker row is the single source of truth for "already initialised"; its primary key makes a concurrent second bootstrap fail
	if _, err := tx.ExecContext(ctx, "INSERT INTO system_state (k, created_at) VALUES ('initialized', ?)", now); err != nil {
		var n int
		if qerr := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM system_state WHERE k='initialized'").Scan(&n); qerr == nil && n > 0 {
			return 0, domain.ErrAlreadyInitialized
		}
		return 0, err
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO admin_user (tenant_id, username, password_hash, role_id, totp_secret_enc, created_at) VALUES (?,?,?,?,?,?)",
		tenant, username, hash, domain.RoleOwner, secretEnc, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), tx.Commit()
}

func (s *MySQLStore) Initialized(ctx context.Context, tenant uint64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM system_state WHERE k='initialized'").Scan(&n)
	return n > 0, err
}

const adminCols = "id, username, password_hash, role_id, status, totp_secret_enc, totp_enrolled_at, totp_last_step, failed_attempts, locked_until, created_at"

func scanAdmin(row interface{ Scan(...any) error }) (ports.Admin, error) {
	var a ports.Admin
	var status string
	var enrolled, locked sql.NullTime
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.RoleID, &status, &a.TOTPSecretEnc, &enrolled, &a.TOTPLastStep, &a.FailedAttempts, &locked, &a.CreatedAt)
	a.Active, a.Enrolled = status == "active", enrolled.Valid
	if locked.Valid {
		a.LockedUntil = locked.Time
	}
	return a, err
}

func (s *MySQLStore) AdminByUsername(ctx context.Context, tenant uint64, username string) (ports.Admin, bool, error) {
	a, err := scanAdmin(s.DB.QueryRowContext(ctx, "SELECT "+adminCols+" FROM admin_user WHERE tenant_id=? AND username=?", t1(tenant), username))
	if errors.Is(err, sql.ErrNoRows) {
		return a, false, nil
	}
	return a, err == nil, err
}

func (s *MySQLStore) AdminByID(ctx context.Context, tenant uint64, id uint64) (ports.Admin, bool, error) {
	a, err := scanAdmin(s.DB.QueryRowContext(ctx, "SELECT "+adminCols+" FROM admin_user WHERE tenant_id=? AND id=?", t1(tenant), id))
	if errors.Is(err, sql.ErrNoRows) {
		return a, false, nil
	}
	return a, err == nil, err
}

func (s *MySQLStore) ListAdmins(ctx context.Context, tenant uint64) ([]ports.Admin, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT "+adminCols+" FROM admin_user WHERE tenant_id=? ORDER BY id", t1(tenant))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.Admin
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateAdmin(ctx context.Context, tenant uint64, username, hash string, roleID uint8, secretEnc []byte, createdBy uint64, now time.Time) (uint64, error) {
	res, err := s.DB.ExecContext(ctx, "INSERT INTO admin_user (tenant_id, username, password_hash, role_id, totp_secret_enc, created_by, created_at) VALUES (?,?,?,?,?,?,?)",
		t1(tenant), username, hash, roleID, secretEnc, createdBy, now)
	if err != nil {
		var n int
		if qerr := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_user WHERE tenant_id=? AND username=?", t1(tenant), username).Scan(&n); qerr == nil && n > 0 {
			return 0, domain.ErrConflict
		}
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (s *MySQLStore) DisableAdmin(ctx context.Context, tenant uint64, id uint64, now time.Time) error {
	tenant = t1(tenant)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Every disable first locks ALL active owners in id order, then the target. A fixed order is what keeps two owners disabling each other at the same time
	// from deadlocking (each holding its own row and waiting for the other's); the second transaction simply waits, then sees the first one's result.
	rows, err := tx.QueryContext(ctx, "SELECT id FROM admin_user WHERE tenant_id=? AND role_id=? AND status='active' ORDER BY id FOR UPDATE", tenant, domain.RoleOwner)
	if err != nil {
		return err
	}
	owners, targetIsActiveOwner := 0, false
	for rows.Next() {
		var oid uint64
		if err := rows.Scan(&oid); err != nil {
			rows.Close()
			return err
		}
		owners++
		targetIsActiveOwner = targetIsActiveOwner || oid == id
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM admin_user WHERE tenant_id=? AND id=? FOR UPDATE", tenant, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if targetIsActiveOwner && owners <= 1 {
		return domain.ErrLastOwner
	}
	if _, err := tx.ExecContext(ctx, "UPDATE admin_user SET status='disabled' WHERE id=?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE admin_session SET revoked_at=? WHERE admin_id=? AND revoked_at IS NULL", now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) RecordFailure(ctx context.Context, id uint64, maxAttempts int, lockFor time.Duration, now time.Time) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT failed_attempts FROM admin_user WHERE id=? FOR UPDATE", id).Scan(&n); err != nil {
		return false, err
	}
	n++
	locked := n >= maxAttempts
	if locked {
		if _, err := tx.ExecContext(ctx, "UPDATE admin_user SET failed_attempts=0, locked_until=? WHERE id=?", now.Add(lockFor), id); err != nil {
			return false, err
		}
	} else if _, err := tx.ExecContext(ctx, "UPDATE admin_user SET failed_attempts=? WHERE id=?", n, id); err != nil {
		return false, err
	}
	return locked, tx.Commit()
}

func (s *MySQLStore) ResetFailures(ctx context.Context, id uint64) error {
	_, err := s.DB.ExecContext(ctx, "UPDATE admin_user SET failed_attempts=0, locked_until=NULL WHERE id=?", id)
	return err
}

func (s *MySQLStore) ConfirmStep(ctx context.Context, id uint64, step uint64, enroll bool, now time.Time) error {
	if enroll {
		_, err := s.DB.ExecContext(ctx, "UPDATE admin_user SET totp_last_step=?, totp_enrolled_at=COALESCE(totp_enrolled_at, ?) WHERE id=?", step, now, id)
		return err
	}
	_, err := s.DB.ExecContext(ctx, "UPDATE admin_user SET totp_last_step=? WHERE id=?", step, id)
	return err
}

func (s *MySQLStore) NewChallenge(ctx context.Context, adminID uint64, kind, hash string, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx, "INSERT INTO admin_challenge (admin_id, kind, token_hash, expires_at) VALUES (?,?,?,?)", adminID, kind, hash, expires)
	return err
}

func (s *MySQLStore) ConsumeChallenge(ctx context.Context, kind, hash string, now time.Time) (uint64, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	var id, admin uint64
	err = tx.QueryRowContext(ctx, "SELECT id, admin_id FROM admin_challenge WHERE token_hash=? AND kind=? AND used_at IS NULL AND expires_at > ? FOR UPDATE", hash, kind, now).Scan(&id, &admin)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE admin_challenge SET used_at=? WHERE id=?", now, id); err != nil {
		return 0, false, err
	}
	return admin, true, tx.Commit()
}

func (s *MySQLStore) NewSession(ctx context.Context, adminID uint64, hash, ip string, now, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx, "INSERT INTO admin_session (admin_id, token_hash, ip, created_at, last_seen_at, expires_at) VALUES (?,?,?,?,?,?)", adminID, hash, ip, now, now, expires)
	return err
}

func (s *MySQLStore) Authenticate(ctx context.Context, hash string, now time.Time, idle time.Duration) (ports.Principal, bool, error) {
	var p ports.Principal
	err := s.DB.QueryRowContext(ctx, `SELECT s.id, a.id, a.username, a.role_id FROM admin_session s JOIN admin_user a ON a.id=s.admin_id
		WHERE s.token_hash=? AND s.revoked_at IS NULL AND s.expires_at > ? AND s.last_seen_at > ? AND a.status='active' AND a.totp_enrolled_at IS NOT NULL`,
		hash, now, now.Add(-idle)).Scan(&p.SessionID, &p.AdminID, &p.Username, &p.RoleID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT permission FROM permission WHERE role_id=?", p.RoleID)
	if err != nil {
		return p, false, err
	}
	defer rows.Close()
	p.Perms = map[string]bool{}
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return p, false, err
		}
		p.Perms[perm] = true
	}
	if _, err := s.DB.ExecContext(ctx, "UPDATE admin_session SET last_seen_at=? WHERE id=?", now, p.SessionID); err != nil {
		return p, false, err
	}
	return p, true, rows.Err()
}

func (s *MySQLStore) RevokeSession(ctx context.Context, id uint64, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, "UPDATE admin_session SET revoked_at=COALESCE(revoked_at, ?) WHERE id=?", now, id)
	return err
}

func (s *MySQLStore) ListAudit(ctx context.Context, tenant uint64, f ports.AuditFilter) ([]ports.AuditRow, error) {
	q := "SELECT id, actor, action, target, COALESCE(detail,''), trace_id, created_at FROM audit_log WHERE tenant_id=?"
	args := []any{t1(tenant)}
	if f.Actor != "" {
		q += " AND actor=?"
		args = append(args, f.Actor)
	}
	if f.Action != "" {
		q += " AND action=?"
		args = append(args, f.Action)
	}
	if f.BeforeID > 0 {
		q += " AND id<?"
		args = append(args, f.BeforeID)
	}
	q += " ORDER BY id DESC LIMIT ?"
	args = append(args, f.Limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.AuditRow
	for rows.Next() {
		var r ports.AuditRow
		if err := rows.Scan(&r.ID, &r.Actor, &r.Action, &r.Target, &r.Detail, &r.TraceID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
