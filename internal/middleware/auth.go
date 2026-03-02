package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"api-gw/internal/token"
)

// Auth validates the Bearer token from the Authorization header.
func Auth(store token.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSON(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			apiKey := parts[1]
			if apiKey == "" || apiKey != strings.TrimSpace(apiKey) {
				writeJSON(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			tok, err := store.Get(r.Context(), apiKey)
			if err != nil {
				slog.Error("token lookup failed", "error", err, "request_id", GetRequestID(r.Context()))
				writeJSON(w, http.StatusInternalServerError, "internal server error")
				return
			}
			if tok == nil || tok.IsExpired() {
				writeJSON(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := token.NewContext(r.Context(), tok)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
