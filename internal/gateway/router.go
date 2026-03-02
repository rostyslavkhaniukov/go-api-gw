package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"api-gw/internal/middleware"
	"api-gw/internal/ratelimiter"
	"api-gw/internal/token"
)

// NewRouter builds the chi router with all middleware and routes.
func NewRouter(
	tokenStore token.Store,
	limiter ratelimiter.Limiter,
	proxyHandler http.Handler,
	maxBodyBytes int64,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware (applied to all routes)
	r.Use(middleware.Recovery)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging)
	r.Use(middleware.Metrics)
	r.Use(middleware.MaxBody(maxBodyBytes))

	// Protected API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(tokenStore))
		r.Use(middleware.RouteCheck)
		r.Use(middleware.RateLimit(limiter))

		r.HandleFunc("/*", proxyHandler.ServeHTTP)
	})

	return r
}
