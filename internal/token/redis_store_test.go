package token

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

type RedisStoreSuite struct {
	suite.Suite
	client *redis.Client
	mr     *miniredis.Miniredis
	store  *RedisStore
	ctx    context.Context
}

func (s *RedisStoreSuite) SetupTest() {
	s.mr = miniredis.RunT(s.T())
	s.client = redis.NewClient(&redis.Options{Addr: s.mr.Addr()})
	s.store = NewRedisStore(s.client)
	s.ctx = context.Background()
}

func (s *RedisStoreSuite) TearDownTest() {
	s.client.Close()
}

func (s *RedisStoreSuite) TestSetAndGet() {
	tok := &Token{
		APIKey:        "test-key-1",
		RateLimit:     100,
		ExpiresAt:     time.Now().Add(time.Hour),
		AllowedRoutes: []string{"/api/v1/users/*"},
	}

	s.Require().NoError(s.store.Set(s.ctx, tok))

	got, err := s.store.Get(s.ctx, "test-key-1")
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(tok.APIKey, got.APIKey)
	s.Equal(tok.RateLimit, got.RateLimit)
	s.Equal([]string{"/api/v1/users/*"}, got.AllowedRoutes)
}

func (s *RedisStoreSuite) TestGetNotFound() {
	got, err := s.store.Get(s.ctx, "nonexistent")
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *RedisStoreSuite) TestOverwriteToken() {
	tok := &Token{
		APIKey:        "overwrite-key",
		RateLimit:     100,
		ExpiresAt:     time.Now().Add(time.Hour),
		AllowedRoutes: []string{"/api/v1/users/*"},
	}
	s.Require().NoError(s.store.Set(s.ctx, tok))

	tok.RateLimit = 200
	tok.AllowedRoutes = []string{"/api/v1/users/*", "/api/v1/products/*"}
	s.Require().NoError(s.store.Set(s.ctx, tok))

	got, err := s.store.Get(s.ctx, "overwrite-key")
	s.Require().NoError(err)
	s.Equal(200, got.RateLimit)
	s.Len(got.AllowedRoutes, 2)
}

func (s *RedisStoreSuite) TestSetWithTTL() {
	tok := &Token{
		APIKey:    "ttl-key",
		RateLimit: 50,
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}
	s.Require().NoError(s.store.Set(s.ctx, tok))

	ttl := s.mr.TTL("token:ttl-key")
	s.Positive(ttl)
	s.LessOrEqual(ttl, 2*time.Hour)
}

func (s *RedisStoreSuite) TestSetPastExpiry() {
	tok := &Token{
		APIKey:    "past-key",
		RateLimit: 50,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	s.Require().NoError(s.store.Set(s.ctx, tok))

	ttl := s.mr.TTL("token:past-key")
	s.Positive(ttl)
	s.LessOrEqual(ttl, 2*time.Second)
}

func (s *RedisStoreSuite) TestMultipleTokens() {
	tokens := []*Token{
		{APIKey: "key-1", RateLimit: 10, ExpiresAt: time.Now().Add(time.Hour)},
		{APIKey: "key-2", RateLimit: 20, ExpiresAt: time.Now().Add(time.Hour)},
		{APIKey: "key-3", RateLimit: 30, ExpiresAt: time.Now().Add(time.Hour)},
	}

	for _, tok := range tokens {
		s.Require().NoError(s.store.Set(s.ctx, tok))
	}

	for _, tok := range tokens {
		got, err := s.store.Get(s.ctx, tok.APIKey)
		s.Require().NoError(err)
		s.Require().NotNil(got)
		s.Equal(tok.RateLimit, got.RateLimit)
	}
}

func (s *RedisStoreSuite) TestRedisFailure() {
	s.mr.Close()

	_, err := s.store.Get(s.ctx, "any-key")
	s.Require().Error(err)

	tok := &Token{
		APIKey:    "fail-key",
		RateLimit: 50,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	s.Error(s.store.Set(s.ctx, tok))
}

func (s *RedisStoreSuite) TestCorruptedJSON() {
	s.mr.Set("token:corrupt-key", "not valid json{{{")

	_, err := s.store.Get(s.ctx, "corrupt-key")
	s.Error(err)
}

func TestRedisStoreSuite(t *testing.T) {
	suite.Run(t, new(RedisStoreSuite))
}
