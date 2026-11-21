// Package ratelimit is a small Redis fixed-window counter used by the send-policy gate (ARCHITECTURE AD-21). It is deliberately simple and fail-open at
// the caller: a Redis outage must not stop messaging, so the gate treats an error here as "skip this rule" and counts it.
package ratelimit

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct{ C *redis.Client }

// Open connects to addr. Timeouts are short: the check runs inside the IM's synchronous before_send hook.
func Open(addr, password string) *Redis {
	return &Redis{C: redis.NewClient(&redis.Options{
		Addr: addr, Password: password,
		DialTimeout: 200 * time.Millisecond, ReadTimeout: 100 * time.Millisecond, WriteTimeout: 100 * time.Millisecond,
		MaxRetries: 0, PoolSize: 20,
	})}
}

func (r *Redis) Close() error { return r.C.Close() }

// Allow counts one event for subject in the current window and reports whether it is within limit. retryAfter is the time until the window rolls over.
func (r *Redis) Allow(ctx context.Context, subject string, limit int, window time.Duration) (ok bool, retryAfter time.Duration, err error) {
	now := time.Now()
	slot := now.UnixNano() / int64(window)
	key := "vg:rl:" + subject + ":" + strconv.FormatInt(slot, 10)
	pipe := r.C.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window+time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, 0, err
	}
	next := time.Unix(0, (slot+1)*int64(window))
	return incr.Val() <= int64(limit), next.Sub(now), nil
}
