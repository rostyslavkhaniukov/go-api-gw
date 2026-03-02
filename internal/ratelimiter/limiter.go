package ratelimiter

import "context"

// Result contains the outcome of a rate limit check.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   int64 // Unix timestamp when the window resets
}

// Limiter is the interface for rate limiting.
type Limiter interface {
	// Allow checks if a request identified by key is within the rate limit.
	Allow(ctx context.Context, key string, limit int) (Result, error)
}
