package service

import (
	"context"
)

// Cache is the optional read-through cache behind the computed endpoints.
// nil on a service means caching is disabled entirely. Implementations must
// be best-effort: callers treat Get errors as a miss and ignore Set/Del
// errors, so an unavailable cache degrades to a plain DB read.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte) error
	Del(ctx context.Context, keys ...string) error
}
