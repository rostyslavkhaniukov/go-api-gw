package token

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"not expired", time.Now().Add(time.Hour), false},
		{"expired", time.Now().Add(-time.Hour), true},
		{"zero value", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := Token{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, tok.IsExpired())
		})
	}
}

func TestContext(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		tok := &Token{
			APIKey:        "test-key",
			RateLimit:     100,
			AllowedRoutes: []string{"/api/*"},
		}
		ctx := NewContext(context.Background(), tok)

		got, ok := FromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, tok.APIKey, got.APIKey)
		assert.Equal(t, tok.RateLimit, got.RateLimit)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := FromContext(context.Background())
		assert.False(t, ok)
	})
}
