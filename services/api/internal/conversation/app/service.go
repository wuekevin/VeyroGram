// Package app implements the sync use cases (Story 1.16): paged conversation and message catch-up, read marks, and cross-device drafts/settings.
package app

import (
	"context"
	"sort"
	"time"

	"veyrogram/api/internal/conversation/domain"
	"veyrogram/api/internal/conversation/ports"
)

type Service struct {
	IM     ports.IMReader
	Store  ports.Store
	Tenant uint64
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

type ConversationsPage struct {
	Items      []ports.Conversation // recents sorted oldest first
	NextCursor string               // empty when there is nothing more
}

// Conversations pages the caller's conversation list, newest activity first. The uid comes from the token; the IM only ever returns that user's own
// rows, and the system account's service-notice conversation is never listed. Paging is by page number (the IM's native scheme): a conversation that
// becomes active while a client pages can shift one position, so clients upsert by (peer, type) and the next full sync repairs ordering.
func (s *Service) Conversations(ctx context.Context, uid, cursor string, limit, msgCount int) (ConversationsPage, error) {
	page, err := domain.DecodeConvCursor(cursor)
	if err != nil {
		return ConversationsPage{}, err
	}
	limit = domain.Clamp(limit, domain.DefaultConvLimit, domain.MaxConvLimit)
	msgCount = domain.Clamp(msgCount, domain.DefaultRecents, domain.MaxRecents) // the IM returns no rows at all for 0, so there is always at least 1
	rows, err := s.IM.Conversations(ctx, uid, msgCount, page, limit)
	if err != nil {
		return ConversationsPage{}, domain.ErrIMUnavailable
	}
	out := ConversationsPage{Items: make([]ports.Conversation, 0, len(rows))}
	for _, r := range rows {
		if r.ChannelType != domain.ChannelPerson || r.Peer == domain.SystemUID {
			continue // groups get listed by their own story; service notices are not conversations
		}
		sort.Slice(r.Recents, func(i, j int) bool { return r.Recents[i].MessageSeq < r.Recents[j].MessageSeq })
		out.Items = append(out.Items, r)
	}
	if len(rows) >= limit { // a full page means there may be more
		out.NextCursor = domain.EncodeConvCursor(page + 1)
	}
	return out, nil
}

type MessagesPage struct {
	Messages []ports.Message // ascending by seq, no duplicates
	HasMore  bool
}

// Messages returns the next page of the caller's conversation with peer after afterSeq (exclusive), oldest first, so a client that was offline simply
// repeats the call with the last seq it holds until HasMore is false: every message exactly once, in order.
func (s *Service) Messages(ctx context.Context, uid, peer string, afterSeq uint64, limit int) (MessagesPage, error) {
	if !domain.ValidUID(peer) || peer == domain.SystemUID {
		return MessagesPage{}, domain.ErrInvalidInput
	}
	limit = domain.Clamp(limit, domain.DefaultMsgLimit, domain.MaxMsgLimit)
	msgs, err := s.IM.Messages(ctx, uid, peer, afterSeq+1, limit+1, true) // one extra row tells us whether more follow
	if err != nil {
		return MessagesPage{}, domain.ErrIMUnavailable
	}
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].MessageSeq < msgs[j].MessageSeq })
	page := MessagesPage{Messages: make([]ports.Message, 0, len(msgs))}
	var last uint64 = afterSeq
	for _, m := range msgs {
		if m.MessageSeq <= last { // defensive: never hand out a duplicate or an out-of-range row
			continue
		}
		if len(page.Messages) == limit {
			page.HasMore = true
			break
		}
		page.Messages = append(page.Messages, m)
		last = m.MessageSeq
	}
	return page, nil
}

// MarkRead clears the caller's unread count for the conversation with peer (the IM keeps it per user, so every device sees the change).
func (s *Service) MarkRead(ctx context.Context, uid, peer string) error {
	if !domain.ValidUID(peer) || peer == domain.SystemUID {
		return domain.ErrInvalidInput
	}
	if err := s.IM.ClearUnread(ctx, uid, peer, domain.ChannelPerson); err != nil {
		return domain.ErrIMUnavailable
	}
	return nil
}

// PutDraft stores or (with empty content) clears the caller's draft for the conversation with peer. Last writer wins; the returned version orders it.
func (s *Service) PutDraft(ctx context.Context, uid, peer, content string) (uint64, error) {
	if !domain.ValidUID(peer) || peer == domain.SystemUID || !domain.ValidDraft(content) {
		return 0, domain.ErrInvalidInput
	}
	return s.Store.PutDraft(ctx, s.Tenant, uid, peer, domain.ChannelPerson, content, s.now())
}

func (s *Service) PutSetting(ctx context.Context, uid, peer string, pinned, muted bool) (uint64, error) {
	if !domain.ValidUID(peer) || peer == domain.SystemUID {
		return 0, domain.ErrInvalidInput
	}
	return s.Store.PutSetting(ctx, s.Tenant, uid, peer, domain.ChannelPerson, pinned, muted, s.now())
}

type StatePage struct {
	Items     []ports.StateItem
	NextSince uint64
	HasMore   bool
}

// State returns drafts and settings changed after since, in version order. NextSince is the version to send next time.
func (s *Service) State(ctx context.Context, uid string, since uint64, limit int) (StatePage, error) {
	limit = domain.Clamp(limit, domain.DefaultStateLim, domain.MaxStateLimit)
	items, err := s.Store.State(ctx, s.Tenant, uid, since, limit+1)
	if err != nil {
		return StatePage{}, err
	}
	p := StatePage{NextSince: since}
	if len(items) > limit {
		items, p.HasMore = items[:limit], true
	}
	p.Items = items
	if n := len(items); n > 0 {
		p.NextSince = items[n-1].Version
	}
	return p, nil
}
