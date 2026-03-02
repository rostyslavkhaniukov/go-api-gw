package token

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type cachedEntry struct {
	token     *Token
	expiresAt time.Time
}

var _ Store = (*CachedStore)(nil)

// CachedStore wraps a Store with an in-memory cache using sync.Map.
// Concurrent fetches for the same key are deduplicated via singleflight.
// A background goroutine periodically evicts expired entries.
// Only non-nil tokens are cached to prevent memory exhaustion from
// lookups of nonexistent keys.
type CachedStore struct {
	store Store
	ttl   time.Duration
	cache sync.Map
	group singleflight.Group
}

// NewCachedStore creates a CachedStore decorator around the given store.
// It spawns a background eviction goroutine that runs until ctx is cancelled;
// the caller must ensure the context is eventually cancelled to avoid a goroutine leak.
func NewCachedStore(ctx context.Context, store Store, ttl time.Duration) *CachedStore {
	cs := &CachedStore{store: store, ttl: ttl}
	go cs.evictLoop(ctx)
	return cs
}

// evictLoop periodically removes expired entries from the cache.
func (c *CachedStore) evictLoop(ctx context.Context) {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			c.cache.Range(func(key, value any) bool {
				if entry, ok := value.(cachedEntry); ok && now.After(entry.expiresAt) {
					c.cache.Delete(key)
				}
				return true
			})
		}
	}
}

// Get returns a cached token if available and not expired, otherwise
// fetches from the underlying store and caches the result.
// Concurrent requests for the same expired/missing key are deduplicated
// so that only one goroutine hits the underlying store.
// Nil (not-found) results are never cached.
func (c *CachedStore) Get(ctx context.Context, apiKey string) (*Token, error) {
	if v, ok := c.cache.Load(apiKey); ok {
		if entry, ok := v.(cachedEntry); ok && time.Now().Before(entry.expiresAt) {
			return entry.token.clone(), nil
		}
	}

	// Use a detached context for the singleflight callback so that one
	// caller's cancellation does not fail all waiting goroutines.
	// The TTL acts as a hard upper bound to prevent leaked work.
	sfCtx, sfCancel := context.WithTimeout(context.WithoutCancel(ctx), c.ttl)
	defer sfCancel()

	v, err, _ := c.group.Do(apiKey, func() (any, error) {
		// Double-check: another goroutine may have populated the cache
		// while we were waiting for the singleflight lock.
		if v, ok := c.cache.Load(apiKey); ok {
			if entry, ok := v.(cachedEntry); ok && time.Now().Before(entry.expiresAt) {
				return entry.token.clone(), nil
			}
		}

		tok, err := c.store.Get(sfCtx, apiKey)
		if err != nil {
			return nil, err
		}

		if tok != nil {
			c.cache.Store(apiKey, cachedEntry{
				token:     tok,
				expiresAt: time.Now().Add(c.ttl),
			})
		}
		return tok, nil
	})
	if err != nil {
		return nil, err
	}

	tok, _ := v.(*Token)
	if tok != nil {
		return tok.clone(), nil
	}
	return nil, nil
}

// Set delegates to the underlying store and invalidates the cache entry.
func (c *CachedStore) Set(ctx context.Context, tok *Token) error {
	err := c.store.Set(ctx, tok)
	if err != nil {
		return err
	}
	c.cache.Delete(tok.APIKey)
	return nil
}
