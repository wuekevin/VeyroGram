// Package denylist is the Redis deny-list for access tokens of revoked devices (ARCHITECTURE AD-9). An entry lives for slightly longer than the
// access-token TTL, so it vanishes exactly when every token it could block has expired anyway. One lookup per authenticated request.
package denylist

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// List is the deny-list port used by the authentication middleware and the identity module.
type List interface {
	Deny(ctx context.Context, tenant, deviceID uint64, ttl time.Duration) error
	Denied(ctx context.Context, tenant, deviceID uint64) (bool, error)
}

// Redis implements List. Timeouts are short on purpose: a slow Redis must degrade (see authn) rather than slow every request.
type Redis struct{ C *redis.Client }

var _ List = (*Redis)(nil)

// Open connects to addr (host:port) and pings it. A failing ping is returned, but the client is still usable once Redis comes back.
func Open(ctx context.Context, addr, password string) (*Redis, error) {
	c := redis.NewClient(&redis.Options{
		Addr: addr, Password: password,
		DialTimeout: 300 * time.Millisecond, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond,
		MaxRetries: 1, PoolSize: 20,
	})
	r := &Redis{C: c}
	return r, r.Ping(ctx)
}

func (r *Redis) Ping(ctx context.Context) error { return r.C.Ping(ctx).Err() }
func (r *Redis) Close() error                   { return r.C.Close() }

func key(tenant, device uint64) string {
	return "vg:deny:dev:" + strconv.FormatUint(tenant, 10) + ":" + strconv.FormatUint(device, 10)
}

func (r *Redis) Deny(ctx context.Context, tenant, deviceID uint64, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("denylist: ttl must be positive")
	}
	return r.C.Set(ctx, key(tenant, deviceID), "1", ttl).Err()
}

func (r *Redis) Denied(ctx context.Context, tenant, deviceID uint64) (bool, error) {
	n, err := r.C.Exists(ctx, key(tenant, deviceID)).Result()
	return n > 0, err
}
