package server

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/hensi01/play-music/core/redis"
)

// redisRateLimiter is an IP-based fixed-window rate limiter backed by Redis, so
// login attempt limits are shared across Play Music instances. It is used in
// place of httprate when Redis is enabled; when Redis is unavailable the
// request is allowed through (the caller falls back to the in-memory limiter).
func redisRateLimiter(limit int, window time.Duration) func(http.Handler) http.Handler {
	prefix := "playmusic:ratelimit:"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := prefix + clientIP(r)
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			n, err := redis.Incr(ctx, key, window)
			if err != nil {
				// Redis disabled or unreachable: allow the request. The caller
				// only uses this middleware when Redis is enabled, but a Redis
				// outage should not lock out login.
				next.ServeHTTP(w, r)
				return
			}
			if n > int64(limit) {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the client IP from RemoteAddr (already rewritten to the
// real IP by the RealIP middleware).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host
}
