package token

import (
	"context"
	"time"
)

// Token represents an API access token stored in Redis.
type Token struct {
	APIKey        string    `json:"api_key"`
	RateLimit     int       `json:"rate_limit"`
	ExpiresAt     time.Time `json:"expires_at"`
	AllowedRoutes []string  `json:"allowed_routes"`
}

// IsExpired returns true if the token has passed its expiration time.
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// clone returns a shallow copy with its own AllowedRoutes slice
// so that callers cannot mutate cached state.
func (t *Token) clone() *Token {
	dup := *t
	dup.AllowedRoutes = append([]string(nil), t.AllowedRoutes...)
	return &dup
}

type contextKey struct{}

// NewContext returns a new context carrying the token.
func NewContext(ctx context.Context, tok *Token) context.Context {
	return context.WithValue(ctx, contextKey{}, tok)
}

// FromContext extracts the token from context, if present.
func FromContext(ctx context.Context) (*Token, bool) {
	tok, ok := ctx.Value(contextKey{}).(*Token)
	return tok, ok
}
