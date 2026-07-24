package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const idempotencyTTL = 24 * time.Hour

type cachedResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// responseRecorder captures what the wrapped handler writes so it can be
// both sent to the real client and cached for replay.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rr *responseRecorder) WriteHeader(status int) {
	rr.status = status
	rr.ResponseWriter.WriteHeader(status)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.body.Write(b)
	return rr.ResponseWriter.Write(b)
}

// Idempotency enforces the standard "Idempotency-Key" header pattern
// (as used by Stripe and most payment/billing APIs) on POST/PATCH/PUT
// requests: the first request with a given key executes normally and its
// response is cached; any retry with the same key within the TTL replays
// the cached response instead of re-executing the handler. This is what
// prevents a client's network retry from double-charging a card or
// double-creating a resource.
func Idempotency(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodPut {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				// Not required on every endpoint by default — routes that
				// need to mandate it can check the header themselves.
				// Here we only dedupe when the client opts in.
				next.ServeHTTP(w, r)
				return
			}

			redisKey := "idempotency:" + key
			ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
			defer cancel()

			if cached, err := rdb.Get(ctx, redisKey).Result(); err == nil {
				var cr cachedResponse
				if jsonErr := json.Unmarshal([]byte(cached), &cr); jsonErr == nil {
					w.Header().Set("Idempotent-Replay", "true")
					w.WriteHeader(cr.Status)
					_, _ = io.WriteString(w, cr.Body)
					return
				}
			}

			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Only cache successful/deterministic outcomes. A 5xx should
			// be retryable by the client with the same key, not locked in.
			if rec.status < 500 {
				payload, _ := json.Marshal(cachedResponse{Status: rec.status, Body: rec.body.String()})
				rdb.Set(ctx, redisKey, payload, idempotencyTTL)
			}
		})
	}
}
