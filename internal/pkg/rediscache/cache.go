// Package rediscache is a thin typed wrapper around go-redis used by the
// analytics layer (and any other read-heavy path) for short-lived caching.
// When the Redis client is nil the cache degrades to a pass-through — callers
// get a cache-miss on every call but the app continues to work.
package rediscache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// DefaultTTL is the standard window for analytics-style caches. 15 minutes
// matches spec 09 §"Redis 15-min cache".
const DefaultTTL = 15 * time.Minute

// Cache is the minimal surface the analytics usecase depends on.
type Cache interface {
	Get(ctx context.Context, key string, dest any) (hit bool, err error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

type redisCache struct {
	client *redis.Client
	logger zerolog.Logger
	prefix string
}

// New returns a Cache backed by the given Redis client. `prefix` is prepended
// to every key so different features + workspaces don't collide. Pass nil
// client to get a no-op cache that always returns miss (handy in tests).
func New(client *redis.Client, prefix string, logger zerolog.Logger) Cache {
	return &redisCache{client: client, logger: logger, prefix: prefix}
}

func (c *redisCache) key(k string) string {
	if c.prefix == "" {
		return k
	}
	return c.prefix + ":" + k
}

func (c *redisCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	if c == nil || c.client == nil {
		return false, nil
	}
	raw, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		c.logger.Debug().Err(err).Str("key", key).Msg("rediscache: get failed — treating as miss")
		return false, nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		c.logger.Warn().Err(err).Str("key", key).Msg("rediscache: bad json — purging")
		_ = c.client.Del(ctx, c.key(key)).Err()
		return false, nil
	}
	return true, nil
}

func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.key(key), raw, ttl).Err()
}

func (c *redisCache) Del(ctx context.Context, keys ...string) error {
	if c == nil || c.client == nil || len(keys) == 0 {
		return nil
	}
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = c.key(k)
	}
	return c.client.Del(ctx, prefixed...).Err()
}
