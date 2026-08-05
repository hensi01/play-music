// Package redis provides a thin, safely-disablable wrapper around go-redis.
//
// Redis is used as an optional shared state store: now-playing, sessions,
// rate-limit counters, scanner locks, etc. Every operation degrades gracefully
// to a no-op when Redis is disabled or unreachable, so Play Music keeps working
// with purely in-memory state exactly as before.
package redis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/log"
	goredis "github.com/redis/go-redis/v9"
)

// ErrDisabled is returned by operations when Redis is not enabled/unreachable.
var ErrDisabled = errors.New("redis is not enabled")

const (
	// KeyNowPlaying is a Redis hash: userID -> now-playing JSON.
	KeyNowPlaying = "playmusic:nowplaying"
	// KeySessions is a Redis set: token ID -> userID (revocation support).
	KeySessions = "playmusic:sessions"
	// KeyScannerLock is a Redis key used as a distributed lock for scanning.
	KeyScannerLock = "playmusic:scanner:lock"
	// ChannelEvents is the pub/sub channel for server events.
	ChannelEvents = "playmusic:events"
)

var (
	client  *goredis.Client
	enabled bool
	once    sync.Once
)

// Init connects to Redis using conf.Server.Redis. It is idempotent and safe to
// call from multiple goroutines. When disabled or unreachable, the package
// stays disabled and all operations become no-ops.
func Init() {
	once.Do(func() {
		if !conf.Server.Redis.Enabled || conf.Server.Redis.URL == "" {
			log.Debug("Redis not enabled. Using in-memory state")
			return
		}
		opts, err := goredis.ParseURL(conf.Server.Redis.URL)
		if err != nil {
			log.Warn("Invalid Redis URL. Redis will be disabled", "url", conf.Server.Redis.URL, "err", err)
			return
		}
		if conf.Server.Redis.Password != "" {
			opts.Password = conf.Server.Redis.Password
		}
		c := goredis.NewClient(opts)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.Ping(ctx).Err(); err != nil {
			log.Warn("Redis not reachable. Using in-memory state", "err", err)
			_ = c.Close()
			return
		}
		client = c
		enabled = true
		log.Info("Redis connected", "addr", opts.Addr)
	})
}

// Enabled reports whether Redis is configured and reachable.
func Enabled() bool {
	return enabled && client != nil
}

// Get returns the string value for key. ok is false when the key is missing or
// Redis is unavailable.
func Get(ctx context.Context, key string) (string, bool) {
	if !Enabled() {
		return "", false
	}
	v, err := client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

// Set stores key with value and TTL (0 = no expiry). No-op when disabled.
func Set(ctx context.Context, key, value string, ttl time.Duration) {
	if !Enabled() {
		return
	}
	if err := client.Set(ctx, key, value, ttl).Err(); err != nil {
		log.Warn(ctx, "Redis Set failed", "key", key, "err", err)
	}
}

// SetNX atomically sets key only if it does not exist. Returns true when the
// value was set (i.e. the lock was acquired). Returns false when disabled.
func SetNX(ctx context.Context, key, value string, ttl time.Duration) bool {
	if !Enabled() {
		return false
	}
	ok, err := client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		log.Warn(ctx, "Redis SetNX failed", "key", key, "err", err)
		return false
	}
	return ok
}

// Del removes the given keys.
func Del(ctx context.Context, keys ...string) {
	if !Enabled() {
		return
	}
	_ = client.Del(ctx, keys...)
}

// HSet sets a field in a hash.
func HSet(ctx context.Context, hash, field, value string) {
	if !Enabled() {
		return
	}
	_ = client.HSet(ctx, hash, field, value)
}

// HGet returns a field from a hash.
func HGet(ctx context.Context, hash, field string) (string, bool) {
	if !Enabled() {
		return "", false
	}
	v, err := client.HGet(ctx, hash, field).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

// HDel removes fields from a hash.
func HDel(ctx context.Context, hash string, fields ...string) {
	if !Enabled() {
		return
	}
	_ = client.HDel(ctx, hash, fields...)
}

// SAdd adds members to a set.
func SAdd(ctx context.Context, key string, members ...string) {
	if !Enabled() {
		return
	}
	_ = client.SAdd(ctx, key, members)
}

// SRem removes members from a set.
func SRem(ctx context.Context, key string, members ...string) {
	if !Enabled() {
		return
	}
	_ = client.SRem(ctx, key, members)
}

// SMembers returns all members of a set.
func SMembers(ctx context.Context, key string) []string {
	if !Enabled() {
		return nil
	}
	members, err := client.SMembers(ctx, key).Result()
	if err != nil {
		return nil
	}
	return members
}

// Incr increments a counter, applying TTL on first increment. Used for
// distributed rate limiting. Returns ErrDisabled when Redis is unavailable.
func Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if !Enabled() {
		return 0, ErrDisabled
	}
	n, err := client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 && ttl > 0 {
		_ = client.Expire(ctx, key, ttl)
	}
	return n, nil
}

// Publish sends a message to a pub/sub channel.
func Publish(ctx context.Context, channel, message string) {
	if !Enabled() {
		return
	}
	_ = client.Publish(ctx, channel, message)
}

// Subscribe returns a channel receiving payloads published to the given
// channel. It returns nil when Redis is disabled. The channel closes when ctx
// is done or the subscription drops.
func Subscribe(ctx context.Context, channel string) <-chan string {
	if !Enabled() {
		return nil
	}
	sub := client.Subscribe(ctx, channel)
	ch := make(chan string, 8)
	go func() {
		defer close(ch)
		defer func() { _ = sub.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub.Channel():
				if !ok {
					return
				}
				ch <- msg.Payload
			}
		}
	}()
	return ch
}
