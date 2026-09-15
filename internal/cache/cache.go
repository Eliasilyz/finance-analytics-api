// Package cache provides a small Redis-backed cache implementing the
// service.Cache contract. It is strictly best-effort: every method returns
// its error and callers decide whether to fail open or closed.
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a go-redis backed Cache. Keys are namespaced with the prefix so
// cached responses never collide with rate-limit counters.
type Redis struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// NewRedis builds a cache that stores values under prefix+key with ttl.
func NewRedis(client *redis.Client, ttl time.Duration) *Redis {
	return &Redis{client: client, ttl: ttl, prefix: "fda:"}
}

func (c *Redis) key(k string) string { return c.prefix + k }

func (c *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, c.key(key)).Bytes()
}

func (c *Redis) Set(ctx context.Context, key string, val []byte) error {
	return c.client.Set(ctx, c.key(key), val, c.ttl).Err()
}

func (c *Redis) Del(ctx context.Context, keys ...string) error {
	for i := range keys {
		keys[i] = c.key(keys[i])
	}
	return c.client.Del(ctx, keys...).Err()
}
