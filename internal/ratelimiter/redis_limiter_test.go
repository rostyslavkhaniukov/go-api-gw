package ratelimiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RedisLimiterSuite struct {
	suite.Suite
	client *redis.Client
	mr     *miniredis.Miniredis
	ctx    context.Context
}

func (s *RedisLimiterSuite) SetupTest() {
	s.mr = miniredis.RunT(s.T())
	s.client = redis.NewClient(&redis.Options{Addr: s.mr.Addr()})
	s.ctx = context.Background()
}

func (s *RedisLimiterSuite) TearDownTest() {
	s.client.Close()
}

func (s *RedisLimiterSuite) TestAllow() {
	limiter := NewRedisLimiter(s.client, 60*time.Second)

	// First request should be allowed.
	res, err := limiter.Allow(s.ctx, "user-1", 3)
	s.Require().NoError(err)
	s.True(res.Allowed)
	s.Equal(2, res.Remaining)

	// Use up remaining quota.
	for i := range 2 {
		res, err = limiter.Allow(s.ctx, "user-1", 3)
		s.Require().NoError(err)
		s.True(res.Allowed, "request %d should be allowed", i+2)
	}

	// Fourth request should be denied.
	res, err = limiter.Allow(s.ctx, "user-1", 3)
	s.Require().NoError(err)
	s.False(res.Allowed)
	s.Equal(0, res.Remaining)
}

func (s *RedisLimiterSuite) TestDifferentKeys() {
	limiter := NewRedisLimiter(s.client, 60*time.Second)

	res1, err := limiter.Allow(s.ctx, "user-a", 1)
	s.Require().NoError(err)
	s.True(res1.Allowed)

	res2, err := limiter.Allow(s.ctx, "user-b", 1)
	s.Require().NoError(err)
	s.True(res2.Allowed)
}

func (s *RedisLimiterSuite) TestWindowExpiration() {
	limiter := NewRedisLimiter(s.client, 2*time.Second)

	res, err := limiter.Allow(s.ctx, "expire-user", 1)
	s.Require().NoError(err)
	s.True(res.Allowed)

	res, err = limiter.Allow(s.ctx, "expire-user", 1)
	s.Require().NoError(err)
	s.False(res.Allowed)

	// Fast-forward time in miniredis past the window.
	s.mr.FastForward(3 * time.Second)

	res, err = limiter.Allow(s.ctx, "expire-user", 1)
	s.Require().NoError(err)
	s.True(res.Allowed)
}

func (s *RedisLimiterSuite) TestResultFields() {
	limiter := NewRedisLimiter(s.client, 60*time.Second)

	res, err := limiter.Allow(s.ctx, "fields-user", 5)
	s.Require().NoError(err)
	s.Equal(5, res.Limit)
	s.Equal(4, res.Remaining)
	s.NotZero(res.ResetAt)
}

func (s *RedisLimiterSuite) TestHighLimit() {
	limiter := NewRedisLimiter(s.client, 60*time.Second)

	limit := 100
	for i := range limit {
		res, err := limiter.Allow(s.ctx, "high-limit-user", limit)
		s.Require().NoError(err)
		s.Require().True(res.Allowed, "request %d should be allowed (limit %d)", i+1, limit)
	}

	res, err := limiter.Allow(s.ctx, "high-limit-user", limit)
	s.Require().NoError(err)
	s.False(res.Allowed)
}

func (s *RedisLimiterSuite) TestConcurrentAllow() {
	limiter := NewRedisLimiter(s.client, 60*time.Second)
	t := s.T()

	const goroutines = 20
	const limit = 10

	var mu sync.Mutex
	allowedCount := 0
	deniedCount := 0
	var wg sync.WaitGroup

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := limiter.Allow(s.ctx, "concurrent-user", limit)
			if !assert.NoError(t, err) {
				return
			}
			mu.Lock()
			if res.Allowed {
				allowedCount++
			} else {
				deniedCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	s.Equal(limit, allowedCount)
	s.Equal(goroutines-limit, deniedCount)
}

func (s *RedisLimiterSuite) TestZeroLimit() {
	limiter := NewRedisLimiter(s.client, 60*time.Second)

	res, err := limiter.Allow(s.ctx, "zero-limit-user", 0)
	s.Require().NoError(err)
	s.False(res.Allowed)
}

func TestRedisLimiterSuite(t *testing.T) {
	suite.Run(t, new(RedisLimiterSuite))
}
