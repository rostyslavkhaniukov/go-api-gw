package middleware

import (
	"log/slog"
	"net/http"
	"strconv"

	"api-gw/internal/ratelimiter"
	"api-gw/internal/token"
)

// RateLimit enforces per-token rate limiting.
func RateLimit(limiter ratelimiter.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := token.FromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusInternalServerError, "no token in context")
				return
			}

			result, err := limiter.Allow(r.Context(), tok.APIKey, tok.RateLimit)
			if err != nil {
				slog.Error("rate limit check failed", "error", err, "request_id", GetRequestID(r.Context()))
				writeJSON(w, http.StatusInternalServerError, "internal server error")
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt, 10))

			if !result.Allowed {
				writeJSON(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
