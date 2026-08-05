package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/core/redis"
	"github.com/hensi01/play-music/log"
)

// sessionKey derives a stable Redis key from a JWT string. The full token is
// never stored, only a truncated hash, so a leaked Redis dump does not expose
// usable tokens.
func sessionKey(tokenStr string) string {
	h := sha256.Sum256([]byte(tokenStr))
	return fmt.Sprintf("playmusic:session:%x", h[:16])
}

// RecordSession registers an active session in Redis (when enabled). Auth
// remains stateless (signed JWT); this purely additive store enables
// server-side session visibility and revocation.
func RecordSession(ctx context.Context, tokenStr string, userID string) {
	if !redis.Enabled() {
		return
	}
	// Give the write a short timeout so a slow/unreachable Redis never blocks login.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	key := sessionKey(tokenStr)
	redis.Set(ctx, key, userID, conf.Server.SessionTimeout)
	redis.SAdd(ctx, redis.KeySessions, key)
	log.Trace(ctx, "Recorded session in Redis", "sessionKey", key)
}

// DeleteSession removes a session from Redis. Used for server-side revocation.
func DeleteSession(ctx context.Context, tokenStr string) {
	if !redis.Enabled() {
		return
	}
	key := sessionKey(tokenStr)
	redis.SRem(ctx, redis.KeySessions, key)
	redis.Del(ctx, key)
}
