package imgw_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imwebhook"
)

func pct(d []time.Duration, p float64) time.Duration {
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(float64(len(s)-1)*p)]
}

// Latency budget (ARCHITECTURE AD-21: before_send P95 <= 50 ms [ASSUMPTION]). Measured on loopback with a trivial in-memory policy, so it bounds
// the callback hop only; a Redis-backed policy adds its own cost and is NOT covered here. Numbers are for this host only.
func TestContractBeforeSendLatencyBudget(t *testing.T) {
	const n = 200
	ctx := context.Background()
	run := func(withHook bool) []time.Duration {
		h := &hookRig{policy: func(imwebhook.BeforeSend) (imwebhook.Decision, error) { return imwebhook.Decision{Allow: true}, nil }}
		url := ""
		if withHook {
			url = h.start(t, hookSecret)
		}
		c := newClient(t, startIM(t, url))
		var out []time.Duration
		for i := 0; i < n+20; i++ {
			st := time.Now()
			if _, err := c.Send(ctx, "alice", "bob", 1, fmt.Sprintf("lat-%v-%d", withHook, i), []byte("hello")); err != nil {
				t.Fatalf("send %d: %v", i, err)
			}
			if i >= 20 { // warm-up excluded
				out = append(out, time.Since(st))
			}
		}
		return out
	}
	base, hooked := run(false), run(true)
	overhead := pct(hooked, .95) - pct(base, .95)
	t.Logf("send latency over %d sends (loopback, this host): without hook p50=%v p95=%v | with before_send p50=%v p95=%v | p95 overhead=%v",
		n, pct(base, .5), pct(base, .95), pct(hooked, .5), pct(hooked, .95), overhead)
	if pct(hooked, .95) > 50*time.Millisecond+pct(base, .95) {
		t.Fatalf("before_send adds more than the 50ms P95 budget: %v", overhead)
	}
}
