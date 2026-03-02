package ratelimiter

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Sliding window rate limiter Lua script.
// Uses a sorted set where members are unique request IDs (timestamps with counter)
// and scores are timestamps. Removes expired entries, counts remaining, and adds new entry if allowed.
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

-- Remove expired entries outside the window
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

-- Count current entries in the window
local count = redis.call('ZCARD', key)

if count < limit then
    -- Add new entry
    redis.call('ZADD', key, now, member)
    redis.call('EXPIRE', key, window)
    return {1, limit - count - 1}
else
    return {0, 0}
end
`)

var _ Limiter = (*RedisLimiter)(nil)

// RedisLimiter implements Limiter using a Redis sliding window.
type RedisLimiter struct {
	client *redis.Client
	window time.Duration
}

// NewRedisLimiter creates a new sliding window rate limiter.
func NewRedisLimiter(client *redis.Client, window time.Duration) *RedisLimiter {
	return &RedisLimiter{
		client: client,
		window: window,
	}
}

// Allow checks if a request is within the rate limit for the given key.
func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int) (Result, error) {
	now := time.Now()
	windowSecs := int64(l.window / time.Second)
	resetAt := now.Add(l.window).Unix()

	// Use nanosecond timestamp + random suffix as unique member
	member := strconv.FormatInt(now.UnixNano(), 36) + strconv.FormatUint(rand.Uint64(), 36)

	res, err := slidingWindowScript.Run(ctx, l.client,
		[]string{"ratelimit:" + key},
		now.Unix(), windowSecs, limit, member,
	).Int64Slice()
	if err != nil {
		return Result{}, fmt.Errorf("sliding window script: %w", err)
	}

	allowed := res[0] == 1
	remaining := int(res[1])

	return Result{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}
