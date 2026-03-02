package token

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore is a test double for Store.
type mockStore struct {
	getCalls atomic.Int64
	getFunc  func(ctx context.Context, apiKey string) (*Token, error)
	setFunc  func(ctx context.Context, tok *Token) error
}

func (m *mockStore) Get(ctx context.Context, apiKey string) (*Token, error) {
	m.getCalls.Add(1)
	return m.getFunc(ctx, apiKey)
}

func (m *mockStore) Set(ctx context.Context, tok *Token) error {
	return m.setFunc(ctx, tok)
}

func TestCachedStore_HitReturnsCached(t *testing.T) {
	tok := &Token{APIKey: "key1", RateLimit: 100, ExpiresAt: time.Now().Add(time.Hour)}
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) { return tok, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := NewCachedStore(ctx, mock, 10*time.Second)

	got1, err := cs.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, tok, got1)

	got2, err := cs.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, tok, got2)

	assert.Equal(t, int64(1), mock.getCalls.Load())
}

func TestCachedStore_MissFetchesFromStore(t *testing.T) {
	tok := &Token{APIKey: "key1", RateLimit: 50, ExpiresAt: time.Now().Add(time.Hour)}
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) { return tok, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := NewCachedStore(ctx, mock, 10*time.Second)

	got, err := cs.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, tok, got)
	assert.Equal(t, int64(1), mock.getCalls.Load())
}

func TestCachedStore_ExpiredEntryRefetches(t *testing.T) {
	tok := &Token{APIKey: "key1", RateLimit: 100, ExpiresAt: time.Now().Add(time.Hour)}
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) { return tok, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := NewCachedStore(ctx, mock, 1*time.Millisecond)

	_, err := cs.Get(ctx, "key1")
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, err = cs.Get(ctx, "key1")
	require.NoError(t, err)

	assert.Equal(t, int64(2), mock.getCalls.Load())
}

func TestCachedStore_SetInvalidatesCache(t *testing.T) {
	tok := &Token{APIKey: "key1", RateLimit: 100, ExpiresAt: time.Now().Add(time.Hour)}
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) { return tok, nil },
		setFunc: func(_ context.Context, _ *Token) error { return nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := NewCachedStore(ctx, mock, 10*time.Second)

	// Populate cache.
	_, _ = cs.Get(ctx, "key1")
	require.Equal(t, int64(1), mock.getCalls.Load())

	// Set should invalidate.
	require.NoError(t, cs.Set(ctx, tok))

	// Next Get should go to underlying store.
	_, _ = cs.Get(ctx, "key1")
	assert.Equal(t, int64(2), mock.getCalls.Load())
}

func TestCachedStore_NilTokenNotCached(t *testing.T) {
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) { return nil, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := NewCachedStore(ctx, mock, 10*time.Second)

	got1, err := cs.Get(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, got1)

	got2, err := cs.Get(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, got2)

	// Each lookup must hit the underlying store — nil results are not cached.
	assert.Equal(t, int64(2), mock.getCalls.Load())
}

func TestCachedStore_ConcurrentAccess(t *testing.T) {
	tok := &Token{APIKey: "key1", RateLimit: 100, ExpiresAt: time.Now().Add(time.Hour)}
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) {
			time.Sleep(10 * time.Millisecond) // simulate Redis latency
			return tok, nil
		},
		setFunc: func(_ context.Context, _ *Token) error { return nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := NewCachedStore(ctx, mock, 10*time.Second)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := cs.Get(ctx, "key1")
			assert.NoError(t, err)
			assert.Equal(t, tok, got)
		}()
	}
	wg.Wait()

	// singleflight must collapse all concurrent misses into a single store call.
	assert.Equal(t, int64(1), mock.getCalls.Load())
}

func TestCachedStore_EvictsExpiredEntries(t *testing.T) {
	tok := &Token{APIKey: "key1", RateLimit: 100, ExpiresAt: time.Now().Add(time.Hour)}
	mock := &mockStore{
		getFunc: func(_ context.Context, _ string) (*Token, error) { return tok, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ttl := 5 * time.Millisecond
	cs := NewCachedStore(ctx, mock, ttl)

	// Populate cache.
	_, err := cs.Get(ctx, "key1")
	require.NoError(t, err)

	// Entry exists in cache.
	_, ok := cs.cache.Load("key1")
	require.True(t, ok)

	// Wait for the entry to expire and the eviction sweep to run.
	// Sweep runs every TTL, so wait 3x to be safe.
	time.Sleep(ttl * 3)

	// Entry must have been evicted.
	_, ok = cs.cache.Load("key1")
	assert.False(t, ok)
}
