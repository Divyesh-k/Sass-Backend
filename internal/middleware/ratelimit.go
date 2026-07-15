package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
	"github.com/divyeshkakadiya/saas-backend/internal/response"
)

// RateLimit implements a fixed-window counter per identity (authenticated
// user ID if present, otherwise client IP) using Redis INCR + EXPIRE.
// This is deliberately simpler than a sliding-window/token-bucket
// algorithm — it's O(1) per request, easy to reason about, and good
// enough for API abuse protection. Swap the algorithm here if you need
// smoother burst handling later; nothing above this layer changes.
func RateLimit(rdb *redis.Client, requestsPerWindow int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := clientIdentity(r)
			key := fmt.Sprintf("ratelimit:%s:%d", identity, time.Now().Unix()/int64(window.Seconds()))

			ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()

			count, err := rdb.Incr(ctx, key).Result()
			if err != nil {
				// Redis being unavailable should degrade to "allow" rather
				// than take the whole API down — rate limiting is a
				// protection layer, not a hard dependency for correctness.
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				rdb.Expire(ctx, key, window)
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerWindow))
			remaining := requestsPerWindow - int(count)
			if remaining < 0 {
				remaining = 0
			}
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			if int(count) > requestsPerWindow {
				response.Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIdentity(r *http.Request) string {
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		return "user:" + userID
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
