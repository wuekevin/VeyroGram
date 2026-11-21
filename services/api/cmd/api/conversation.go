package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"veyrogram/api/internal/conversation/adapters"
	"veyrogram/api/internal/conversation/app"
	convhttp "veyrogram/api/internal/conversation/http"
	"veyrogram/api/internal/conversation/ports"
	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/outbox"
)

// imReader adapts the IM gateway to the conversation module's read port.
type imReader struct{ im *imgw.Client }

func toMsg(m imgw.SyncedMessage) ports.Message {
	return ports.Message{MessageID: m.MessageID, MessageSeq: m.MessageSeq, ClientMsgNo: m.ClientMsgNo, FromUID: m.FromUID, Timestamp: m.Timestamp, Payload: m.Payload}
}

func (r imReader) Conversations(ctx context.Context, uid string, msgCount, page, pageSize int) ([]ports.Conversation, error) {
	rows, err := r.im.ConversationSync(ctx, uid, msgCount, page, pageSize)
	if err != nil {
		return nil, err
	}
	out := make([]ports.Conversation, len(rows))
	for i, c := range rows {
		cv := ports.Conversation{Peer: c.ChannelID, ChannelType: c.ChannelType, Unread: c.Unread, Timestamp: c.Timestamp, LastSeq: c.LastMessageSeq, ReadToSeq: c.ReadToMessageSeq}
		for _, m := range c.Recents {
			cv.Recents = append(cv.Recents, toMsg(m))
		}
		out[i] = cv
	}
	return out, nil
}

func (r imReader) Messages(ctx context.Context, uid, peer string, startSeq uint64, limit int, up bool) ([]ports.Message, error) {
	res, err := r.im.ChannelMessageSync(ctx, uid, peer, startSeq, limit, up)
	if err != nil {
		return nil, err
	}
	out := make([]ports.Message, len(res.Messages))
	for i, m := range res.Messages {
		out[i] = toMsg(m)
	}
	return out, nil
}

func (r imReader) ClearUnread(ctx context.Context, uid, peer string, channelType uint8) error {
	return r.im.ClearUnread(ctx, uid, peer, channelType)
}

func conversationMux(pool *sql.DB, log *slog.Logger, im *imgw.Client, auth func(http.Handler) http.Handler) *http.ServeMux {
	svc := &app.Service{IM: imReader{im}, Store: &adapters.MySQLStore{DB: pool}, Tenant: 1}
	mux := http.NewServeMux()
	(&convhttp.Handler{Svc: svc, Log: log, Auth: auth}).Mount(mux)
	return mux
}

// stateNoticeHandler is the outbox handler for conversation.state_changed (worker process): it tells the user's devices, through the IM, that drafts or
// settings changed, so another device can fetch /v1/sync/state without polling. The notice carries pointers (kind, peer, version), never draft text.
func stateNoticeHandler(im *imgw.Client) outbox.Handler {
	n := imNotifier{im}
	return func(ctx context.Context, e outbox.Event) error {
		var p map[string]any
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return fmt.Errorf("conversation.state_changed: bad payload")
		}
		uid, _ := p["uid"].(string)
		if uid == "" {
			return fmt.Errorf("conversation.state_changed: bad payload")
		}
		delete(p, "uid")
		p["type"] = "sync_notice"
		msg, _ := json.Marshal(p)
		return n.Notify(ctx, uid, "syncnotice:"+e.ID, msg)
	}
}
