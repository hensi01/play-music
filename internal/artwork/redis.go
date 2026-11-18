package artwork

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func newRedisCache(ctx context.Context, url string) (*redisCache, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opts.MaxRetries = 0
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &redisCache{client: client, ttl: artTTL}, nil
}

// redisKeyPrefix is the Redis namespace for cover bytes; the rest of the key is
// cacheKey. It exists so Get/Set/Invalidate cannot disagree about it.
const redisKeyPrefix = "pm:art:"

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, err := c.client.Get(ctx, redisKeyPrefix+key).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *redisCache) Set(ctx context.Context, key string, data []byte) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.client.Set(ctx, redisKeyPrefix+key, data, c.ttl).Err(); err != nil {
		slog.Debug("redis art cache set failed", "err", err)
	}
}

// Invalidate drops every cached size of an entity. KEYS is acceptable here:
// the key space is pm:art:<id>-<size> and invalidation is rare.
func (c *redisCache) Invalidate(entityID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	keys, err := c.client.Keys(ctx, redisKeyPrefix+cacheKeyPrefix(entityID)+"*").Result()
	if err == nil && len(keys) > 0 {
		c.client.Del(ctx, keys...)
	}
}
