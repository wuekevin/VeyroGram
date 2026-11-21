package denylist

import (
	"context"
	"os"
	"testing"
	"time"
)

func open(t *testing.T) *Redis {
	t.Helper()
	addr := os.Getenv("VG_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set: skipping Redis integration test")
	}
	r, err := Open(context.Background(), addr, "")
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestDenyAndExpiry(t *testing.T) {
	r := open(t)
	ctx := context.Background()
	const tenant, dev = 7, 424242
	if d, err := r.Denied(ctx, tenant, dev); err != nil || d {
		t.Fatalf("fresh device must not be denied: %v %v", d, err)
	}
	if err := r.Deny(ctx, tenant, dev, 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d, _ := r.Denied(ctx, tenant, dev); !d {
		t.Fatal("device must be denied")
	}
	if d, _ := r.Denied(ctx, 8, dev); d {
		t.Fatal("deny entries are tenant-scoped")
	}
	if d, _ := r.Denied(ctx, tenant, dev+1); d {
		t.Fatal("deny entries are device-scoped")
	}
	time.Sleep(450 * time.Millisecond)
	if d, _ := r.Denied(ctx, tenant, dev); d {
		t.Fatal("entry must expire with its TTL so the list cannot grow without bound")
	}
	if err := r.Deny(ctx, tenant, dev, 0); err == nil {
		t.Fatal("a non-positive TTL (an entry that never expires) must be refused")
	}
}

func TestUnreachableRedisReportsAnError(t *testing.T) {
	r, err := Open(context.Background(), "127.0.0.1:1", "")
	if err == nil {
		t.Fatal("ping must fail")
	}
	if _, err := r.Denied(context.Background(), 1, 1); err == nil {
		t.Fatal("lookup against a dead Redis must return an error, not 'not denied'")
	}
}
