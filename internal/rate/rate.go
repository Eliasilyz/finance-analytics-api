// Package rate implements a Redis-backed fixed-window rate limiter. Callers
// decide how to react to an unavailable limiter; the Allow error is surfaced
// so the API can fail open (see handler.RateLimit).
package rate

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter decides whether the request identified by key may proceed.
// A non-nil error means the limiter itself is unavailable and the caller must
// choose a fallback policy (fail-open vs fail-closed).
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// RedisLimiter counts requests per key inside a fixed window using INCR + a
// one-time EXPIRE. The counter is monotonic within the window; it resets when
// the window elapses.
type RedisLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
	prefix string
}

// NewRedis builds a limiter that allows up to limit requests per window per
// key.
func NewRedis(client *redis.Client, limit int64, window time.Duration) *RedisLimiter {
	return &RedisLimiter{client: client, limit: limit, window: window, prefix: "fda:rl:"}
}

// Allow increments the counter for key and reports whether the request stays
// within the window's limit. The counter counts EVERY check, independently of
// whether the guarded request succeeds or fails afterwards.
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	k := l.prefix + key
	n, err := l.client.Incr(ctx, k).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		// First request in this window arms the TTL. If EXPIRE fails the key
		// has no TTL and could grow unbounded; acceptable here, upgrade path:
		// one Lua script doing SET NX PX + INCR atomically.
		_ = l.client.Expire(ctx, k, l.window).Err()
	}
	return n <= l.limit, nil
}
