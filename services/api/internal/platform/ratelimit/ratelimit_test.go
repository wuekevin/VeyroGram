package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFixedWindowAgainstRealRedis(t *testing.T) {
	addr := os.Getenv("VG_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	r := Open(addr, "")
	defer r.Close()
	ctx := context.Background()
	subject := "test:" + time.Now().Format("150405.000000")
	for i := 1; i <= 5; i++ {
		ok, _, err := r.Allow(ctx, subject, 3, time.Minute)
		if err != nil || ok != (i <= 3) {
			t.Fatalf("call %d: ok=%v err=%v", i, ok, err)
		}
	}
	if ok, ra, _ := r.Allow(ctx, subject+"-other", 3, time.Minute); !ok || ra <= 0 || ra > time.Minute {
		t.Fatalf("another subject has its own counter and a sane retry-after: %v %v", ok, ra)
	}
	// unreachable Redis: an error (the gate turns it into fail-open), never a hang
	dead := Open("127.0.0.1:1", "")
	defer dead.Close()
	start := time.Now()
	if _, _, err := dead.Allow(ctx, "x", 1, time.Second); err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("a dead Redis must fail fast: %v after %v", err, time.Since(start))
	}
}
