// Package adapters holds the identity module's MySQL store and SMS gateways.
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/identity/ports"
)

// MySQLStore implements ports.Store. All statements are tenant-scoped (AD-10).
type MySQLStore struct{ DB *sql.DB }

var _ ports.Store = (*MySQLStore)(nil)

// dimension -> fixed column; never built from input.
var dimCol = map[ports.Dim]string{ports.DimPhone: "phone_hash", ports.DimIP: "ip", ports.DimDevice: "device_key"}

func (s *MySQLStore) CountEvents(ctx context.Context, kind, result string, d ports.Dim, val string, since time.Time) (int, error) {
	q := "SELECT COUNT(*) FROM login_event WHERE kind=? AND result=? AND created_at>=?"
	args := []any{kind, result, since}
	if d != ports.DimAll {
		col, ok := dimCol[d]
		if !ok {
			return 0, errors.New("identity: unknown dimension")
		}
		q += " AND " + col + "=?"
		args = append(args, val)
	}
	var n int
	return n, s.DB.QueryRowContext(ctx, q, args...).Scan(&n)
}

func (s *MySQLStore) LastEvent(ctx context.Context, kind, result string, d ports.Dim, val string) (time.Time, bool, error) {
	col, ok := dimCol[d]
	if !ok {
		return time.Time{}, false, errors.New("identity: unknown dimension")
	}
	var t sql.NullTime
	err := s.DB.QueryRowContext(ctx, "SELECT MAX(created_at) FROM login_event WHERE kind=? AND result=? AND "+col+"=?", kind, result, val).Scan(&t)
	return t.Time, t.Valid, err
}

func (s *MySQLStore) AddEvent(ctx context.Context, e ports.Event) error {
	var uid any
	if e.UserID != 0 {
		uid = e.UserID
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO login_event (tenant_id, kind, result, phone_hash, ip, device_key, user_id, trace_id, created_at) VALUES (?,?,?,?,?,?,?,?,?)",
		e.Tenant, e.Kind, e.Result, e.PhoneHash, e.IP, e.DeviceKey, uid, e.TraceID, e.At)
	return err
}

func (s *MySQLStore) NewCode(ctx context.Context, tenant uint64, phoneHash, codeHash string, expires, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE sms_code SET consumed_at=? WHERE tenant_id=? AND phone_hash=? AND consumed_at IS NULL", now, tenant, phoneHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO sms_code (tenant_id, phone_hash, code_hash, expires_at, created_at) VALUES (?,?,?,?,?)", tenant, phoneHash, codeHash, expires, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) LatestCode(ctx context.Context, tenant uint64, phoneHash string, now time.Time) (ports.Code, bool, error) {
	var c ports.Code
	err := s.DB.QueryRowContext(ctx, "SELECT id, code_hash, attempts FROM sms_code WHERE tenant_id=? AND phone_hash=? AND consumed_at IS NULL AND expires_at>? ORDER BY id DESC LIMIT 1",
		tenant, phoneHash, now).Scan(&c.ID, &c.Hash, &c.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	return c, err == nil, err
}

func (s *MySQLStore) BumpAttempt(ctx context.Context, id uint64) error {
	_, err := s.DB.ExecContext(ctx, "UPDATE sms_code SET attempts=attempts+1 WHERE id=?", id)
	return err
}

func (s *MySQLStore) Login(ctx context.Context, in ports.LoginTx) (ports.Session, error) {
	var sess ports.Session
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return sess, err
	}
	defer tx.Rollback()
	// 1. single-use: only one concurrent request can flip consumed_at
	r, err := tx.ExecContext(ctx, "UPDATE sms_code SET consumed_at=? WHERE id=? AND tenant_id=? AND consumed_at IS NULL", in.Now, in.CodeID, in.Tenant)
	if err != nil {
		return sess, err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return sess, domain.ErrCodeConsumed
	}
	// 2. user: load or register
	var status string
	err = tx.QueryRowContext(ctx, "SELECT id, uid, status FROM `user` WHERE tenant_id=? AND phone_hash=? FOR UPDATE", in.Tenant, in.PhoneHash).Scan(&sess.UserID, &sess.UID, &status)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, ierr := tx.ExecContext(ctx, "INSERT INTO `user` (tenant_id, uid, phone_hash, phone_last4, created_at) VALUES (?,?,?,?,?)", in.Tenant, in.NewUID, in.PhoneHash, in.Last4, in.Now)
		if ierr != nil {
			return sess, ierr // a duplicate here means a concurrent first login; the caller's code was single-use so this request simply fails
		}
		id, _ := res.LastInsertId()
		sess.UserID, sess.UID, sess.IsNew, status = uint64(id), in.NewUID, true, "active"
	case err != nil:
		return sess, err
	}
	if status != "active" {
		return sess, domain.ErrDisabled
	}
	// 3. device: one row per (user, device_key); logging in again re-activates a revoked device
	err = tx.QueryRowContext(ctx, "SELECT id FROM device WHERE user_id=? AND device_key=? FOR UPDATE", sess.UserID, in.Device.Key).Scan(&sess.DeviceID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, ierr := tx.ExecContext(ctx, "INSERT INTO device (tenant_id, user_id, device_key, class, name, created_at, last_seen_at) VALUES (?,?,?,?,?,?,?)",
			in.Tenant, sess.UserID, in.Device.Key, string(in.Device.Class), in.Device.Name, in.Now, in.Now)
		if ierr != nil {
			return sess, ierr
		}
		id, _ := res.LastInsertId()
		sess.DeviceID = uint64(id)
	case err != nil:
		return sess, err
	default:
		if _, err := tx.ExecContext(ctx, "UPDATE device SET class=?, name=?, last_seen_at=?, revoked_at=NULL WHERE id=?", string(in.Device.Class), in.Device.Name, in.Now, sess.DeviceID); err != nil {
			return sess, err
		}
	}
	sess.Class = in.Device.Class
	// 4. one live refresh chain per device
	if _, err := tx.ExecContext(ctx, "UPDATE refresh_token SET status='revoked' WHERE device_id=? AND status='active'", sess.DeviceID); err != nil {
		return sess, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO refresh_token (tenant_id, user_id, device_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?,?)",
		in.Tenant, sess.UserID, sess.DeviceID, in.RefreshHash, in.RefreshExpires, in.Now); err != nil {
		return sess, err
	}
	return sess, tx.Commit()
}

func (s *MySQLStore) Rotate(ctx context.Context, tenant uint64, oldHash, newHash string, newExpires, now time.Time) (ports.Session, ports.RotateResult, error) {
	var sess ports.Session
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return sess, ports.RotateInvalid, err
	}
	defer tx.Rollback()
	var id uint64
	var status string
	var expires time.Time
	err = tx.QueryRowContext(ctx, "SELECT id, user_id, device_id, status, expires_at FROM refresh_token WHERE tenant_id=? AND token_hash=? FOR UPDATE", tenant, oldHash).
		Scan(&id, &sess.UserID, &sess.DeviceID, &status, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return sess, ports.RotateInvalid, nil
	}
	if err != nil {
		return sess, ports.RotateInvalid, err
	}
	if status == "rotated" { // reuse: someone holds a stale token, so the chain is compromised. Revoke the device and commit that.
		if _, err := tx.ExecContext(ctx, "UPDATE device SET revoked_at=? WHERE id=? AND revoked_at IS NULL", now, sess.DeviceID); err != nil {
			return sess, ports.RotateInvalid, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE refresh_token SET status='revoked' WHERE device_id=? AND status='active'", sess.DeviceID); err != nil {
			return sess, ports.RotateInvalid, err
		}
		return sess, ports.RotateReuse, tx.Commit()
	}
	if status != "active" || !expires.After(now) {
		return sess, ports.RotateInvalid, nil
	}
	var class string
	var revoked sql.NullTime
	var ustatus string
	if err := tx.QueryRowContext(ctx, "SELECT d.class, d.revoked_at, u.uid, u.status FROM device d JOIN `user` u ON u.id=d.user_id WHERE d.id=?", sess.DeviceID).
		Scan(&class, &revoked, &sess.UID, &ustatus); err != nil {
		return sess, ports.RotateInvalid, err
	}
	if revoked.Valid || ustatus != "active" {
		return sess, ports.RotateInvalid, nil
	}
	sess.Class = domain.DeviceClass(class)
	if _, err := tx.ExecContext(ctx, "UPDATE refresh_token SET status='rotated' WHERE id=?", id); err != nil {
		return sess, ports.RotateInvalid, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO refresh_token (tenant_id, user_id, device_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?,?)",
		tenant, sess.UserID, sess.DeviceID, newHash, newExpires, now); err != nil {
		return sess, ports.RotateInvalid, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE device SET last_seen_at=? WHERE id=?", now, sess.DeviceID); err != nil {
		return sess, ports.RotateInvalid, err
	}
	return sess, ports.RotateOK, tx.Commit()
}
