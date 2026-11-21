// Package app implements the send path of the message module (Story 1.15): the policy gate shared by the IM's before_send hook and the
// /v1/messages API, and the API's send use case.
package app

import (
	"context"
	"time"

	"veyrogram/api/internal/message/domain"
	"veyrogram/api/internal/message/ports"
)

// Rule is one check of the send policy. A Restricted rule guards something that must never be skipped (membership, block, mute, account state):
// if it errors or overruns the budget the send is DENIED (fail-closed, AD-21). An unrestricted rule that fails is skipped and counted, so a flaky
// optional check (say, a rate-limit projection) cannot take messaging down.
type Rule struct {
	Name       string
	Restricted bool
	Check      func(context.Context, domain.Send) (domain.Verdict, error)
}

// Gate evaluates the rules in order within a fixed time budget. The first denial wins.
type Gate struct {
	Rules   []Rule
	Budget  time.Duration             // whole-evaluation budget; the IM waits synchronously, so keep it well under its own timeout
	Observe func(rule, result string) // result: allow | deny | error | timeout | skipped
	Timing  func(time.Duration)       // whole-evaluation duration (metrics); optional
}

// DefaultBudget is a starting point, to be tuned against NFR-9 (P95 <= 50 ms for the whole hook is an [ASSUMPTION] still to be load-tested).
const DefaultBudget = 400 * time.Millisecond

func (g *Gate) observe(rule, result string) {
	if g.Observe != nil {
		g.Observe(rule, result)
	}
}

// Evaluate never returns an error: every failure mode is a verdict.
func (g *Gate) Evaluate(ctx context.Context, s domain.Send) domain.Verdict {
	if g.Timing != nil {
		start := time.Now()
		defer func() { g.Timing(time.Since(start)) }()
	}
	if s.From == domain.SystemUID { // server-originated account notices; no client can authenticate as this uid
		g.observe("system", "allow")
		return domain.Allow
	}
	budget := g.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	for _, r := range g.Rules {
		type res struct {
			v   domain.Verdict
			err error
		}
		ch := make(chan res, 1)
		go func() {
			v, err := r.Check(ctx, s)
			ch <- res{v, err}
		}()
		var got res
		select {
		case got = <-ch:
		case <-ctx.Done():
			got = res{err: ctx.Err()}
		}
		switch {
		case got.err != nil && r.Restricted:
			result := "error"
			if ctx.Err() != nil {
				result = "timeout"
			}
			g.observe(r.Name, result)
			return domain.Verdict{Reason: domain.ReasonUnavailable}
		case got.err != nil:
			g.observe(r.Name, "skipped")
		case !got.v.Allow:
			g.observe(r.Name, "deny")
			return got.v
		default:
			g.observe(r.Name, "allow")
		}
	}
	return domain.Allow
}

// BlockRule refuses a message when the recipient has blocked the sender. It is a restricted rule: if the block list cannot be read the send is denied
// (fail-closed), never waved through. The sender is told "recipient unavailable", the same answer as for a missing account, so a block is not revealed.
func BlockRule(b ports.Blocks) Rule {
	return Rule{Name: "block", Restricted: true, Check: func(ctx context.Context, s domain.Send) (domain.Verdict, error) {
		peer, ok := domain.Peer(s.From, s.ChannelID)
		if !ok || peer == s.From {
			return domain.Allow, nil // structural problems are the channel rule's business; writing to yourself is never a block
		}
		blocked, err := b.BlockedBy(ctx, peer, s.From)
		if err != nil {
			return domain.Verdict{}, err
		}
		if blocked {
			return domain.Verdict{Reason: domain.ReasonRecipientUnavailable}, nil
		}
		return domain.Allow, nil
	}}
}

// RateRule limits how fast one account may send. It is an unrestricted rule: when the counter store is down the rule is skipped and counted, so a Redis
// outage never stops messaging (the restricted rules still apply). limit <= 0 disables it.
func RateRule(rl ports.RateLimiter, limit int, window time.Duration) Rule {
	return Rule{Name: "ratelimit", Restricted: false, Check: func(ctx context.Context, s domain.Send) (domain.Verdict, error) {
		if limit <= 0 || rl == nil {
			return domain.Allow, nil
		}
		ok, _, err := rl.Allow(ctx, "msg:"+s.From, limit, window)
		if err != nil {
			return domain.Verdict{}, err
		}
		if !ok {
			return domain.Verdict{Reason: domain.ReasonRateLimited}, nil
		}
		return domain.Allow, nil
	}}
}

// StandardRules is the v1 person-chat policy. Block lists, mute and rate limits join here when their stories land (they read the Redis projection
// described in AD-21); until then the IM's native send_ban/blacklist (checked by the IM before this hook) cover mute and block.
func StandardRules(acc ports.Accounts) []Rule {
	return []Rule{
		{Name: "channel", Restricted: true, Check: func(_ context.Context, s domain.Send) (domain.Verdict, error) {
			if s.ChannelType != domain.ChannelPerson {
				return domain.Verdict{Reason: domain.ReasonUnsupportedChannel}, nil // groups get their rules in their own story; until then closed
			}
			if _, ok := domain.Peer(s.From, s.ChannelID); !ok {
				return domain.Verdict{Reason: domain.ReasonUnsupportedChannel}, nil
			}
			return domain.Allow, nil
		}},
		{Name: "payload", Restricted: true, Check: func(_ context.Context, s domain.Send) (domain.Verdict, error) {
			if r := domain.CheckPayload(s.Payload); r != 0 {
				return domain.Verdict{Reason: r}, nil
			}
			return domain.Allow, nil
		}},
		{Name: "accounts", Restricted: true, Check: func(ctx context.Context, s domain.Send) (domain.Verdict, error) {
			peer, _ := domain.Peer(s.From, s.ChannelID)
			m, err := acc.Active(ctx, []string{s.From, peer})
			if err != nil {
				return domain.Verdict{}, err
			}
			if !m[s.From] {
				return domain.Verdict{Reason: domain.ReasonSenderUnavailable}, nil
			}
			if peer == domain.SystemUID || !m[peer] {
				return domain.Verdict{Reason: domain.ReasonRecipientUnavailable}, nil
			}
			return domain.Allow, nil
		}},
	}
}
