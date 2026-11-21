// Package ports defines what the conversation module needs from the outside world.
package ports

import (
	"context"
	"time"
)

// Message is one message as read back from the IM.
type Message struct {
	MessageID   int64
	MessageSeq  uint64
	ClientMsgNo string
	FromUID     string
	Timestamp   int32
	Payload     []byte
}

// Conversation is one row of a user's conversation list. Unread and ReadToSeq are the IM's per-user truth.
type Conversation struct {
	Peer        string
	ChannelType uint8
	Unread      uint64
	Timestamp   int64
	LastSeq     uint64
	ReadToSeq   uint64
	Recents     []Message // newest first, as the IM returns them
}

// IMReader is the read side of the IM gateway. Every call is scoped to a uid taken from the verified token, never from the request.
type IMReader interface {
	Conversations(ctx context.Context, uid string, msgCount, page, pageSize int) ([]Conversation, error)
	// Messages returns up to limit messages of the uid<->peer conversation: ascending from startSeq (inclusive) when up, else older ones at or before it.
	Messages(ctx context.Context, uid, peer string, startSeq uint64, limit int, up bool) ([]Message, error)
	ClearUnread(ctx context.Context, uid, peer string, channelType uint8) error
}

type Draft struct {
	Peer        string
	ChannelType uint8
	Content     string
	Deleted     bool
	Version     uint64
}

type Setting struct {
	Peer        string
	ChannelType uint8
	Pinned      bool
	Muted       bool
	Version     uint64
}

// StateItem is one changed piece of synced state; exactly one of Draft/Setting is set.
type StateItem struct {
	Version uint64
	Draft   *Draft
	Setting *Setting
}

type Store interface {
	// PutDraft stores the draft (empty content = tombstone) under a fresh per-user version and queues the change notice, in one transaction.
	// baseVersion is optional (nil = last writer wins). When given and the stored draft is newer than it, nothing is written and *ConflictError carries the
	// current draft so the client can merge.
	PutDraft(ctx context.Context, tenant uint64, uid, peer string, channelType uint8, content string, baseVersion *uint64, now time.Time) (version uint64, err error)
	PutSetting(ctx context.Context, tenant uint64, uid, peer string, channelType uint8, pinned, muted bool, now time.Time) (version uint64, err error)
	// State lists drafts and settings changed after `since`, ordered by version, at most limit items.
	State(ctx context.Context, tenant uint64, uid string, since uint64, limit int) ([]StateItem, error)
}

// ConflictError is returned by PutDraft when baseVersion is older than the stored draft.
type ConflictError struct{ Current Draft }

func (e *ConflictError) Error() string { return "conversation: draft changed elsewhere" }
