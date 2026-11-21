// Package adapters implements the conversation module's ports.
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"veyrogram/api/internal/conversation/ports"
	"veyrogram/api/internal/platform/outbox"
)

// TopicStateChanged is the outbox topic that tells a user's devices that synced state changed; the worker turns it into an IM notice.
const TopicStateChanged = "conversation.state_changed"

type MySQLStore struct{ DB *sql.DB }

func tenantOr1(t uint64) uint64 {
	if t == 0 {
		return 1
	}
	return t
}

// nextVersion hands out the user's next version. The sync_clock row is locked for the rest of the transaction, so concurrent writers of one user are
// serialised and versions are unique and gap-free per user: a "changed since N" cursor can never skip a change.
func nextVersion(ctx context.Context, tx *sql.Tx, tenant uint64, uid string) (uint64, error) {
	if _, err := tx.ExecContext(ctx, "INSERT INTO sync_clock (tenant_id, user_uid, version) VALUES (?,?,LAST_INSERT_ID(1)) ON DUPLICATE KEY UPDATE version=LAST_INSERT_ID(version+1)", tenant, uid); err != nil {
		return 0, err
	}
	var v uint64
	err := tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&v)
	return v, err
}

func notify(ctx context.Context, tx *sql.Tx, now time.Time, tenant uint64, uid, kind, peer string, version uint64) error {
	payload, _ := json.Marshal(map[string]any{"uid": uid, "kind": kind, "peer": peer, "version": version}) // pointers only, never draft text
	_, err := outbox.Enqueue(ctx, tx, now, tenant, TopicStateChanged, "state:"+uid, payload)
	return err
}

func (s *MySQLStore) PutDraft(ctx context.Context, tenant uint64, uid, peer string, channelType uint8, content string, now time.Time) (uint64, error) {
	tenant = tenantOr1(tenant)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	v, err := nextVersion(ctx, tx, tenant, uid)
	if err != nil {
		return 0, err
	}
	deleted := content == ""
	if _, err := tx.ExecContext(ctx, `INSERT INTO draft (tenant_id, user_uid, channel_id, channel_type, content, deleted, version, updated_at) VALUES (?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE content=VALUES(content), deleted=VALUES(deleted), version=VALUES(version), updated_at=VALUES(updated_at)`,
		tenant, uid, peer, channelType, content, deleted, v, now); err != nil {
		return 0, err
	}
	if err := notify(ctx, tx, now, tenant, uid, "draft", peer, v); err != nil {
		return 0, err
	}
	return v, tx.Commit()
}

func (s *MySQLStore) PutSetting(ctx context.Context, tenant uint64, uid, peer string, channelType uint8, pinned, muted bool, now time.Time) (uint64, error) {
	tenant = tenantOr1(tenant)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	v, err := nextVersion(ctx, tx, tenant, uid)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_setting (tenant_id, user_uid, channel_id, channel_type, pinned, muted, version, updated_at) VALUES (?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE pinned=VALUES(pinned), muted=VALUES(muted), version=VALUES(version), updated_at=VALUES(updated_at)`,
		tenant, uid, peer, channelType, pinned, muted, v, now); err != nil {
		return 0, err
	}
	if err := notify(ctx, tx, now, tenant, uid, "setting", peer, v); err != nil {
		return 0, err
	}
	return v, tx.Commit()
}

func (s *MySQLStore) State(ctx context.Context, tenant uint64, uid string, since uint64, limit int) ([]ports.StateItem, error) {
	tenant = tenantOr1(tenant)
	rows, err := s.DB.QueryContext(ctx, `
		(SELECT version, 'draft' AS kind, channel_id, channel_type, content, deleted, 0 AS pinned, 0 AS muted FROM draft WHERE tenant_id=? AND user_uid=? AND version>?)
		UNION ALL
		(SELECT version, 'setting', channel_id, channel_type, '', 0, pinned, muted FROM conversation_setting WHERE tenant_id=? AND user_uid=? AND version>?)
		ORDER BY version LIMIT ?`, tenant, uid, since, tenant, uid, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.StateItem
	for rows.Next() {
		var it ports.StateItem
		var kind, peer, content string
		var ct uint8
		var deleted, pinned, muted bool
		if err := rows.Scan(&it.Version, &kind, &peer, &ct, &content, &deleted, &pinned, &muted); err != nil {
			return nil, err
		}
		if kind == "draft" {
			it.Draft = &ports.Draft{Peer: peer, ChannelType: ct, Content: content, Deleted: deleted, Version: it.Version}
		} else {
			it.Setting = &ports.Setting{Peer: peer, ChannelType: ct, Pinned: pinned, Muted: muted, Version: it.Version}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
