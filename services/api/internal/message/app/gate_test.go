package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"veyrogram/api/internal/message/domain"
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
