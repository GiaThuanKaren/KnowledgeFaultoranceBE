package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter limits incoming requests per UID (if authenticated) or client IP
// using a Redis sliding-window log.
func RedisRateLimiter(client *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client == nil || limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			identifier := extractIdentifier(r)
			key := fmt.Sprintf("ratelimit:%s", identifier)

			ctx := r.Context()
			now := time.Now().UnixNano()
			clearBefore := now - window.Nanoseconds()

			pipe := client.TxPipeline()
			pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(clearBefore, 10))
			cardCmd := pipe.ZCard(ctx, key)
			_, err := pipe.Exec(ctx)
			if err != nil && err != redis.Nil {
				// On Redis error, fail open to avoid blocking legitimate traffic
				next.ServeHTTP(w, r)
				return
			}

			if cardCmd.Val() >= int64(limit) {
				writeError(w, http.StatusTooManyRequests, "ERR_RATE_LIMITED", "Rate limit exceeded. Please try again later.")
				return
			}

			// Record current request
			randSuffix := make([]byte, 4)
			_, _ = rand.Read(randSuffix)
			member := fmt.Sprintf("%d-%s", now, hex.EncodeToString(randSuffix))

			pipe2 := client.TxPipeline()
			pipe2.ZAdd(ctx, key, redis.Z{
				Score:  float64(now),
				Member: member,
			})
			pipe2.Expire(ctx, key, window)
			_, _ = pipe2.Exec(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

func extractIdentifier(r *http.Request) string {
	if uid, ok := GetUID(r.Context()); ok && uid != "" {
		return uid
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}

	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		if ip := strings.TrimSpace(xrip); ip != "" {
			return ip
		}
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}
