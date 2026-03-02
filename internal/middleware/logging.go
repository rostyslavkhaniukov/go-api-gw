package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"api-gw/internal/httputil"
)

// Logging logs each request with method, path, status, and duration using slog.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw, ok := w.(*httputil.ResponseWriter)
		if !ok {
			rw = httputil.NewResponseWriter(w)
		}

		next.ServeHTTP(rw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", GetRequestID(r.Context()),
		)
	})
}
