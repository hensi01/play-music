package artwork

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache is a best-effort cache in front of the disk cache; failures
// fall back silently.
type redisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func newRedisCache(ctx context.Context, url string, log *slog.Logger) (*redisCache, error) {
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

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, err := c.client.Get(ctx, "pm:art:"+key).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *redisCache) Set(ctx context.Context, key string, data []byte) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.client.Set(ctx, "pm:art:"+key, data, c.ttl).Err(); err != nil {
		slog.Debug("redis art cache set failed", "err", err)
	}
}
