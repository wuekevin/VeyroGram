package app

import (
	"context"
	"errors"
	"testing"

	"veyrogram/api/internal/conversation/domain"
	"veyrogram/api/internal/conversation/ports"
)

type fakeIM struct {
	convs []ports.Conversation
	msgs  []ports.Message
	err   error
	got   struct {
		start uint64
		limit int
		up    bool
	}
	pageSize int
}

func (f *fakeIM) Conversations(_ context.Context, _ string, _, _, pageSize int) ([]ports.Conversation, error) {
	f.pageSize = pageSize
	return f.convs, f.err
}
func (f *fakeIM) Messages(_ context.Context, _, _ string, start uint64, limit int, up bool) ([]ports.Message, error) {
	f.got.start, f.got.limit, f.got.up = start, limit, up
	return f.msgs, f.err
}
func (f *fakeIM) ClearUnread(context.Context, string, string, uint8) error { return f.err }

func seqs(n ...uint64) []ports.Message {
	var out []ports.Message
	for _, s := range n {
		out = append(out, ports.Message{MessageSeq: s})
	}
	return out
}

func TestMessagesPagingIsOrderedDedupedAndSignalsMore(t *testing.T) {
	im := &fakeIM{msgs: seqs(7, 5, 6, 6, 8)} // unordered, with a duplicate, one more than the limit
	s := &Service{IM: im}
	p, err := s.Messages(context.Background(), "u1", "u2", 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if im.got.start != 5 || im.got.limit != 4 || !im.got.up {
		t.Fatalf("must ask the IM for seq>=after+1, ascending, one extra row: %+v", im.got)
	}
	if len(p.Messages) != 3 || p.Messages[0].MessageSeq != 5 || p.Messages[1].MessageSeq != 6 || p.Messages[2].MessageSeq != 7 || !p.HasMore {
		t.Fatalf("got %+v", p)
	}
	// rows at or below the cursor are never handed out again
	im.msgs = seqs(3, 4, 5)
	p, _ = s.Messages(context.Background(), "u1", "u2", 4, 50)
	if len(p.Messages) != 1 || p.Messages[0].MessageSeq != 5 || p.HasMore {
		t.Fatalf("stale rows must be dropped: %+v", p)
	}
}

func TestMessagesRejectsBadPeers(t *testing.T) {
	s := &Service{IM: &fakeIM{}}
	for _, peer := range []string{"", "a@b", "vg_system", "a b", "x/../y"} {
		if _, err := s.Messages(context.Background(), "u1", peer, 0, 10); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("peer %q must be refused: %v", peer, err)
		}
	}
}

func TestConversationsFiltersSystemAndNonPersonAndPages(t *testing.T) {
	im := &fakeIM{convs: []ports.Conversation{
		{Peer: "bob", ChannelType: 1, Recents: seqs(9, 7, 8)},
		{Peer: domain.SystemUID, ChannelType: 1},
		{Peer: "g1", ChannelType: 2},
	}}
	s := &Service{IM: im}
	p, err := s.Conversations(context.Background(), "u1", "", 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 1 || p.Items[0].Peer != "bob" {
		t.Fatalf("only the person conversation is listed: %+v", p.Items)
	}
	if r := p.Items[0].Recents; r[0].MessageSeq != 7 || r[2].MessageSeq != 9 {
		t.Fatalf("recents oldest first: %+v", r)
	}
	if p.NextCursor != "c1.2" { // the IM returned a full page (3 rows) before filtering
		t.Fatalf("cursor %q", p.NextCursor)
	}
	im.convs = im.convs[:1]
	if p, _ = s.Conversations(context.Background(), "u1", "c1.2", 3, 3); p.NextCursor != "" {
		t.Fatalf("a short page ends the listing: %q", p.NextCursor)
	}
	for _, bad := range []string{"x", "c2.1", "c1.0", "c1.-3", "c1.abc", "c1.99999"} {
		if _, err := s.Conversations(context.Background(), "u1", bad, 3, 3); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("cursor %q must be refused", bad)
		}
	}
	if im.pageSize != 3 {
		t.Fatalf("page size forwarded: %d", im.pageSize)
	}
	if _, err := s.Conversations(context.Background(), "u1", "", 1000, 1000); err != nil || im.pageSize != domain.MaxConvLimit {
		t.Fatalf("limits are clamped: %d %v", im.pageSize, err)
	}
}

func TestIMFailureIsClassified(t *testing.T) {
	s := &Service{IM: &fakeIM{err: errors.New("boom")}}
	if _, err := s.Messages(context.Background(), "u1", "u2", 0, 5); !errors.Is(err, domain.ErrIMUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.Conversations(context.Background(), "u1", "", 5, 1); !errors.Is(err, domain.ErrIMUnavailable) {
		t.Fatal(err)
	}
	if err := s.MarkRead(context.Background(), "u1", "u2"); !errors.Is(err, domain.ErrIMUnavailable) {
		t.Fatal(err)
	}
}

func TestDraftValidation(t *testing.T) {
	s := &Service{}
	for _, c := range []struct{ peer, content string }{{"a@b", "x"}, {"vg_system", "x"}, {"bob", string(make([]rune, 4001))}, {"bob", "\xff\xfe"}} {
		if _, err := s.PutDraft(context.Background(), "u1", c.peer, c.content, nil); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%+v must be refused", c)
		}
	}
}

func TestRecentsAreClampedSoThePageStaysUnderTheIMPayloadBudget(t *testing.T) {
	var gotCount int
	im := &countIM{count: &gotCount}
	s := &Service{IM: im}
	if _, err := s.Conversations(context.Background(), "u1", "", 100, 20); err != nil || gotCount != 10 {
		t.Fatalf("100 rows x 20 recents must be clamped to 100 x 10: %d %v", gotCount, err)
	}
	if _, err := s.Conversations(context.Background(), "u1", "", 20, 20); err != nil || gotCount != 20 {
		t.Fatalf("20 x 20 is within bounds: %d", gotCount)
	}
	if _, err := s.Conversations(context.Background(), "u1", "", 100, 1); err != nil || gotCount != 1 {
		t.Fatalf("100 x 1: %d", gotCount)
	}
}

type countIM struct {
	fakeIM
	count *int
}

func (c *countIM) Conversations(ctx context.Context, uid string, msgCount, page, pageSize int) ([]ports.Conversation, error) {
	*c.count = msgCount
	return nil, nil
}
