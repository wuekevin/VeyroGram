// Package adapters implements the message module's ports.
package adapters

import (
	"context"
	"database/sql"

	"veyrogram/api/internal/message/ports"
)

type MySQLStore struct{ DB *sql.DB }

func (s *MySQLStore) Record(ctx context.Context, r ports.Ref) error {
	tenant := r.Tenant
	if tenant == 0 {
		tenant = 1
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO message_ref (tenant_id, message_id, message_seq, from_uid, channel_id, channel_type, client_msg_no, created_at) VALUES (?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id",
		tenant, r.MessageID, r.MessageSeq, r.From, r.ChannelID, r.ChannelType, r.ClientMsgNo, r.At)
	return err
}
