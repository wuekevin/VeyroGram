package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"veyrogram/api/internal/message/domain"
	"veyrogram/api/internal/message/ports"
)

type accounts struct {
	active map[string]bool
	err    error
	delay  time.Duration
}

func (a accounts) Active(ctx context.Context, uids []string) (map[string]bool, error) {
	if a.delay > 0 {
		time.Sleep(a.delay) // deliberately ignores ctx: the gate must still answer within its budget
	}
	return a.active, a.err
}

func msg(from, to, payload string) domain.Send {
	return domain.Send{From: from, ChannelID: to, ChannelType: 1, ClientMsgNo: "x", Payload: []byte(payload)}
}

const okText = `{"type":1,"content":"hi"}`

func TestStandardRulesDecisions(t *testing.T) {
	g := &Gate{Rules: StandardRules(accounts{active: map[string]bool{"alice": true, "bob": true}})}
	for _, c := range []struct {
		name string
		s    domain.Send
		want uint8 // 0 = allow
	}{
		{"ok", msg("alice", "bob@alice", okText), 0},
		{"unknown recipient", msg("alice", "carol", okText), domain.ReasonRecipientUnavailable},
		{"inactive sender", msg("carol", "bob", okText), domain.ReasonSenderUnavailable},
		{"system is not a peer", msg("alice", "vg_system", okText), domain.ReasonRecipientUnavailable},
		{"group not supported yet", domain.Send{From: "alice", ChannelID: "g1", ChannelType: 2, Payload: []byte(okText)}, domain.ReasonUnsupportedChannel},
		{"foreign conversation", msg("alice", "bob@carol", okText), domain.ReasonUnsupportedChannel},
		{"bad payload", msg("alice", "bob", `{"type":1}`), domain.ReasonInvalidPayload},
		{"system sender bypasses the rules", msg(domain.SystemUID, "bob", `{"type":"account_notice"}`), 0},
	} {
		v := g.Evaluate(context.Background(), c.s)
		if c.want == 0 && !v.Allow || c.want != 0 && (v.Allow || v.Reason != c.want) {
			t.Errorf("%s: %+v, want reason %d", c.name, v, c.want)
		}
	}
}

func TestRestrictedRuleFailsClosedOnErrorAndTimeout(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	obs := func(rule, res string) { mu.Lock(); seen[rule+"/"+res] = true; mu.Unlock() }

	g := &Gate{Rules: StandardRules(accounts{err: errors.New("mysql down")}), Observe: obs}
	if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); v.Allow || v.Reason != domain.ReasonUnavailable {
		t.Fatalf("a failing restricted rule must deny with 200: %+v", v)
	}
	g = &Gate{Rules: StandardRules(accounts{active: map[string]bool{"alice": true, "bob": true}, delay: 500 * time.Millisecond}), Budget: 50 * time.Millisecond, Observe: obs}
	start := time.Now()
	v := g.Evaluate(context.Background(), msg("alice", "bob", okText))
	if v.Allow || v.Reason != domain.ReasonUnavailable {
		t.Fatalf("an overrunning restricted rule must deny with 200: %+v", v)
	}
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("the budget must bound the answer even when a rule ignores its context: took %v", d)
	}
	mu.Lock()
	defer mu.Unlock()
	if !seen["accounts/error"] || !seen["accounts/timeout"] {
		t.Fatalf("failures must be observable: %v", seen)
	}
}

func TestUnrestrictedRuleFailureIsSkippedNotFatal(t *testing.T) {
	var skipped bool
	g := &Gate{Observe: func(rule, res string) {
		if rule == "ratelimit" && res == "skipped" {
			skipped = true
		}
	}, Rules: []Rule{
		{Name: "ratelimit", Restricted: false, Check: func(context.Context, domain.Send) (domain.Verdict, error) {
			return domain.Verdict{}, errors.New("redis down")
		}},
		{Name: "block", Restricted: true, Check: func(context.Context, domain.Send) (domain.Verdict, error) {
			return domain.Verdict{Reason: 150}, nil
		}},
	}}
	if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); v.Allow || v.Reason != 150 {
		t.Fatalf("a flaky optional rule must not hide a later denial: %+v", v)
	}
	if !skipped {
		t.Fatal("the skip must be counted")
	}
	g.Rules = g.Rules[:1]
	if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); !v.Allow {
		t.Fatalf("with only a failing optional rule the send is allowed: %+v", v)
	}
}

type fakeLimiter struct {
	n   int
	err error
}

func (f *fakeLimiter) Allow(_ context.Context, subject string, limit int, _ time.Duration) (bool, time.Duration, error) {
	f.n++
	return f.n <= limit, time.Second, f.err
}

func TestRateRuleDeniesOverLimitAndFailsOpenWhenCounterIsDown(t *testing.T) {
	rl := &fakeLimiter{}
	g := &Gate{Rules: []Rule{RateRule(rl, 2, time.Second)}}
	for i, want := range []bool{true, true, false, false} {
		if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); v.Allow != want || (!want && v.Reason != domain.ReasonRateLimited) {
			t.Fatalf("send %d: %+v", i+1, v)
		}
	}
	// system notices are never rate limited
	if v := g.Evaluate(context.Background(), msg(domain.SystemUID, "bob", okText)); !v.Allow {
		t.Fatal("system sender must bypass")
	}
	// counter store down: the optional rule is skipped (and counted), messaging continues
	var skipped bool
	g = &Gate{Rules: []Rule{RateRule(&fakeLimiter{err: errors.New("redis down")}, 2, time.Second)}, Observe: func(r, res string) { skipped = skipped || res == "skipped" }}
	if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); !v.Allow || !skipped {
		t.Fatalf("fail-open expected: %+v skipped=%v", v, skipped)
	}
	// disabled
	g = &Gate{Rules: []Rule{RateRule(rl, 0, time.Second)}}
	if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); !v.Allow {
		t.Fatal("limit 0 disables the rule")
	}
}

type recRefs struct{ got []ports.Ref }

func (r *recRefs) Record(_ context.Context, ref ports.Ref) error {
	r.got = append(r.got, ref)
	return nil
}

func TestRecordNotifiedCanonicalisesAndSkipsNonBusinessMessages(t *testing.T) {
	refs := &recRefs{}
	s := &Service{Refs: refs, Tenant: 1}
	err := s.RecordNotified(context.Background(), []Notified{
		{MessageID: 1, MessageSeq: 1, ClientMsgNo: "a", FromUID: "alice", ChannelID: "bob@alice", ChannelType: 1, Timestamp: 100},
		{MessageID: 2, MessageSeq: 2, ClientMsgNo: "b", FromUID: "bob", ChannelID: "alice@bob", ChannelType: 1, Timestamp: 101},
		{MessageID: 3, MessageSeq: 3, ClientMsgNo: "n", FromUID: domain.SystemUID, ChannelID: "alice@vg_system", ChannelType: 1}, // system notice
		{MessageID: 4, MessageSeq: 4, ClientMsgNo: "g", FromUID: "alice", ChannelID: "g1", ChannelType: 2},                       // group
		{MessageID: 5, MessageSeq: 5, ClientMsgNo: "x", FromUID: "mallory", ChannelID: "alice@bob", ChannelType: 1},              // not a participant
		{MessageID: 0, MessageSeq: 6, ClientMsgNo: "z", FromUID: "alice", ChannelID: "bob@alice", ChannelType: 1},                // no id
	})
	if err != nil || len(refs.got) != 2 {
		t.Fatalf("only the two person messages are recorded: %v %+v", err, refs.got)
	}
	if refs.got[0].ChannelID != "alice@bob" || refs.got[1].ChannelID != "alice@bob" {
		t.Fatalf("both directions of one conversation share one canonical id: %+v", refs.got)
	}
}

type fakeBlocks struct {
	blocked map[string]bool // "blocker>sender"
	err     error
}

func (f fakeBlocks) BlockedBy(_ context.Context, blocker, sender string) (bool, error) {
	return f.blocked[blocker+">"+sender], f.err
}

func TestBlockRule(t *testing.T) {
	g := &Gate{Rules: []Rule{BlockRule(fakeBlocks{blocked: map[string]bool{"bob>alice": true}})}}
	for _, c := range []struct {
		name string
		s    domain.Send
		want uint8
	}{
		{"blocked sender is refused like a missing recipient", msg("alice", "bob@alice", okText), domain.ReasonRecipientUnavailable},
		{"the blocker can still write", msg("bob", "alice", okText), 0},
		{"a third party is not affected", msg("carol", "bob", okText), 0},
		{"writing to yourself is never a block", msg("alice", "alice", okText), 0},
		{"system notices pass", msg(domain.SystemUID, "bob", okText), 0},
	} {
		v := g.Evaluate(context.Background(), c.s)
		if c.want == 0 && !v.Allow || c.want != 0 && (v.Allow || v.Reason != c.want) {
			t.Errorf("%s: %+v", c.name, v)
		}
	}
	// the block list cannot be read: deny, never allow
	g = &Gate{Rules: []Rule{BlockRule(fakeBlocks{err: errors.New("mysql down")})}}
	if v := g.Evaluate(context.Background(), msg("alice", "bob", okText)); v.Allow || v.Reason != domain.ReasonUnavailable {
		t.Fatalf("fail-closed expected: %+v", v)
	}
}
