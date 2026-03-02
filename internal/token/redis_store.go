package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "token:"

var _ Store = (*RedisStore)(nil)

// RedisStore implements Store backed by Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new RedisStore.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Get retrieves a token by API key from Redis.
func (s *RedisStore) Get(ctx context.Context, apiKey string) (*Token, error) {
	data, err := s.client.Get(ctx, keyPrefix+apiKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &tok, nil
}

// Set stores a token in Redis with a TTL derived from ExpiresAt.
func (s *RedisStore) Set(ctx context.Context, tok *Token) error {
	data, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	ttl := time.Until(tok.ExpiresAt)
	if ttl <= 0 {
		slog.Warn("storing already-expired token", "api_key", tok.APIKey, "expires_at", tok.ExpiresAt)
		ttl = time.Second
	} else if ttl < time.Second {
		ttl = time.Second
	}

	if err := s.client.Set(ctx, keyPrefix+tok.APIKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}
