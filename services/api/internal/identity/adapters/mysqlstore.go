// Package adapters holds the identity module's MySQL store and SMS gateways.
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
	"veyrogram/api/internal/platform/outbox"

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
	var revokedAt sql.NullTime
	err = tx.QueryRowContext(ctx, "SELECT id, revoked_at FROM device WHERE user_id=? AND device_key=? FOR UPDATE", sess.UserID, in.Device.Key).Scan(&sess.DeviceID, &revokedAt)
	if err == nil && revokedAt.Valid {
		// A revoked device id stays revoked forever (stale access tokens carry it); logging in again issues a new id.
		if _, err := tx.ExecContext(ctx, "UPDATE device SET device_key=CONCAT('revoked:', id) WHERE id=?", sess.DeviceID); err != nil {
			return sess, err
		}
		err = sql.ErrNoRows
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, ierr := tx.ExecContext(ctx, "INSERT INTO device (tenant_id, user_id, device_key, class, name, created_at, last_seen_at) VALUES (?,?,?,?,?,?,?)",
			in.Tenant, sess.UserID, in.Device.Key, string(in.Device.Class), in.Device.Name, in.Now, in.Now)
		if ierr != nil {
			return sess, ierr
		}
		id, _ := res.LastInsertId()
		sess.DeviceID = uint64(id)
		// A new device signing in to an account that already has live devices: tell them (the new device sees it too and can match device_id).
		var others int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device WHERE user_id=? AND id<>? AND revoked_at IS NULL", sess.UserID, sess.DeviceID).Scan(&others); err != nil {
			return sess, err
		}
		if others > 0 {
			if err := queueNotice(ctx, tx, in.Tenant, sess.UID, map[string]any{"kind": "new_login", "device_id": strconv.FormatUint(sess.DeviceID, 10), "class": string(in.Device.Class), "name": in.Device.Name}, in.Now); err != nil {
				return sess, err
			}
		}
	case err != nil:
		return sess, err
	default:
		if _, err := tx.ExecContext(ctx, "UPDATE device SET class=?, name=?, last_seen_at=? WHERE id=?", string(in.Device.Class), in.Device.Name, in.Now, sess.DeviceID); err != nil {
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
		var class string
		if err := tx.QueryRowContext(ctx, "SELECT d.class, u.uid FROM device d JOIN `user` u ON u.id=d.user_id WHERE d.id=?", sess.DeviceID).Scan(&class, &sess.UID); err != nil {
			return sess, ports.RotateInvalid, err
		}
		sess.Class = domain.DeviceClass(class)
		if err := bumpAndQueue(ctx, tx, tenant, sess.UserID, sess.UID, sess.Class, now); err != nil {
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

// bumpAndQueue advances the IM epoch of (user, class) and queues the IM sync in the caller's transaction.
func bumpAndQueue(ctx context.Context, tx *sql.Tx, tenant, userID uint64, uid string, class domain.DeviceClass, now time.Time) error {
	if _, err := tx.ExecContext(ctx, "INSERT INTO im_epoch (tenant_id, user_id, class, epoch, updated_at) VALUES (?,?,?,1,?) ON DUPLICATE KEY UPDATE epoch=epoch+1, updated_at=VALUES(updated_at)",
		tenant, userID, string(class), now); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"uid": uid, "class": string(class)})
	_, err := outbox.Enqueue(ctx, tx, now, tenant, domain.TopicIMSync, "im:"+uid, payload)
	return err
}

// queueNotice records, in the same transaction, the notice for the user's other devices. It has its own ordering key on purpose: a cosmetic
// notice that fails or is slow must never hold back the security-relevant IM sync (kick + token rotation) queued for the same user.
func queueNotice(ctx context.Context, tx *sql.Tx, tenant uint64, uid string, n map[string]any, now time.Time) error {
	n["uid"] = uid
	payload, _ := json.Marshal(n)
	_, err := outbox.Enqueue(ctx, tx, now, tenant, domain.TopicDeviceNotice, "notice:"+uid, payload)
	return err
}

func (s *MySQLStore) ListDevices(ctx context.Context, tenant uint64, userUID string) ([]ports.DeviceInfo, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT d.id, d.class, d.name, d.created_at, d.last_seen_at FROM device d JOIN `user` u ON u.id=d.user_id WHERE d.tenant_id=? AND u.uid=? AND d.revoked_at IS NULL ORDER BY d.last_seen_at DESC, d.id DESC",
		tenant, userUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.DeviceInfo
	for rows.Next() {
		var d ports.DeviceInfo
		var class string
		if err := rows.Scan(&d.ID, &class, &d.Name, &d.CreatedAt, &d.LastSeenAt); err != nil {
			return nil, err
		}
		d.Class = domain.DeviceClass(class)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *MySQLStore) RevokeDevice(ctx context.Context, tenant uint64, userUID string, deviceID uint64, now time.Time) (ports.RevokedDevice, bool, error) {
	rd := ports.RevokedDevice{DeviceID: deviceID, UID: userUID}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return rd, false, err
	}
	defer tx.Rollback()
	var userID uint64
	var class string
	err = tx.QueryRowContext(ctx, "SELECT d.class, u.id FROM device d JOIN `user` u ON u.id=d.user_id WHERE d.id=? AND d.tenant_id=? AND u.uid=? FOR UPDATE", deviceID, tenant, userUID).Scan(&class, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return rd, false, nil // not found, or not this user's: indistinguishable on purpose
	}
	if err != nil {
		return rd, false, err
	}
	rd.Class = domain.DeviceClass(class)
	if _, err := tx.ExecContext(ctx, "UPDATE device SET revoked_at=COALESCE(revoked_at, ?) WHERE id=?", now, deviceID); err != nil {
		return rd, false, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE refresh_token SET status='revoked' WHERE device_id=? AND status='active'", deviceID); err != nil {
		return rd, false, err
	}
	if err := bumpAndQueue(ctx, tx, tenant, userID, userUID, rd.Class, now); err != nil {
		return rd, false, err
	}
	if err := queueNotice(ctx, tx, tenant, userUID, map[string]any{"kind": "device_revoked", "device_id": strconv.FormatUint(deviceID, 10), "class": string(rd.Class)}, now); err != nil {
		return rd, false, err
	}
	return rd, true, tx.Commit()
}

func (s *MySQLStore) RevokeAllDevices(ctx context.Context, tenant uint64, userUID string, now time.Time) ([]ports.RevokedDevice, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var userID uint64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM `user` WHERE tenant_id=? AND uid=? FOR UPDATE", tenant, userUID).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id, class FROM device WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL FOR UPDATE", tenant, userID)
	if err != nil {
		return nil, err
	}
	var out []ports.RevokedDevice
	classes := map[domain.DeviceClass]bool{}
	for rows.Next() {
		rd := ports.RevokedDevice{UID: userUID}
		var class string
		if err := rows.Scan(&rd.DeviceID, &class); err != nil {
			rows.Close()
			return nil, err
		}
		rd.Class = domain.DeviceClass(class)
		out = append(out, rd)
		classes[rd.Class] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE device SET revoked_at=? WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL", now, tenant, userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE refresh_token SET status='revoked' WHERE user_id=? AND status='active'", userID); err != nil {
		return nil, err
	}
	for class := range classes {
		if err := bumpAndQueue(ctx, tx, tenant, userID, userUID, class, now); err != nil {
			return nil, err
		}
	}
	if err := queueNotice(ctx, tx, tenant, userUID, map[string]any{"kind": "logout_all", "devices": len(out)}, now); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

func (s *MySQLStore) IMCreds(ctx context.Context, tenant uint64, userUID string, deviceID uint64) (ports.IMCreds, error) {
	var c ports.IMCreds
	var class string
	var revoked sql.NullTime
	var userID uint64
	err := s.DB.QueryRowContext(ctx, "SELECT d.class, d.revoked_at, u.id FROM device d JOIN `user` u ON u.id=d.user_id WHERE d.id=? AND d.tenant_id=? AND u.uid=? AND u.status='active'", deviceID, tenant, userUID).Scan(&class, &revoked, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	c.Class, c.Active = domain.DeviceClass(class), !revoked.Valid
	c.Epoch, err = s.ClassEpoch(ctx, tenant, userUID, c.Class)
	return c, err
}

func (s *MySQLStore) ClassEpoch(ctx context.Context, tenant uint64, userUID string, class domain.DeviceClass) (uint64, error) {
	var e uint64
	err := s.DB.QueryRowContext(ctx, "SELECT e.epoch FROM im_epoch e JOIN `user` u ON u.id=e.user_id WHERE e.tenant_id=? AND u.uid=? AND e.class=?", tenant, userUID, string(class)).Scan(&e)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return e, err
}
