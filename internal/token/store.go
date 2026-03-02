package token

import "context"

// Store is the interface for token persistence.
type Store interface {
	// Get retrieves a token by its API key. Returns nil if not found.
	Get(ctx context.Context, apiKey string) (*Token, error)
	// Set stores a token, using its ExpiresAt for TTL.
	Set(ctx context.Context, tok *Token) error
}
