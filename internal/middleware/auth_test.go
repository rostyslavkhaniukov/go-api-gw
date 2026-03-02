package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"api-gw/internal/token"
)

// mockStore implements token.Store for testing.
type mockStore struct {
	tokens map[string]*token.Token
	err    error
}

func (m *mockStore) Get(_ context.Context, apiKey string) (*token.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens[apiKey], nil
}

func (m *mockStore) Set(_ context.Context, tok *token.Token) error {
	m.tokens[tok.APIKey] = tok
	return nil
}

func TestAuth(t *testing.T) {
	validToken := &token.Token{
		APIKey:        "valid-key",
		RateLimit:     100,
		ExpiresAt:     time.Now().Add(time.Hour),
		AllowedRoutes: []string{"/api/v1/*"},
	}
	expiredToken := &token.Token{
		APIKey:    "expired-key",
		RateLimit: 100,
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	tests := []struct {
		name      string
		store     *mockStore
		auth      string
		wantCode  int
		wantToken bool
	}{
		{
			name:     "missing header",
			store:    &mockStore{},
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "invalid format",
			store:    &mockStore{},
			auth:     "Basic abc123",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "empty API key",
			store:    &mockStore{err: errors.New("should not be called")},
			auth:     "Bearer ",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "unknown token",
			store:    &mockStore{tokens: map[string]*token.Token{}},
			auth:     "Bearer nonexistent",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "expired token",
			store:    &mockStore{tokens: map[string]*token.Token{"expired-key": expiredToken}},
			auth:     "Bearer expired-key",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:      "valid token",
			store:     &mockStore{tokens: map[string]*token.Token{"valid-key": validToken}},
			auth:      "Bearer valid-key",
			wantCode:  http.StatusOK,
			wantToken: true,
		},
		{
			name:     "case insensitive bearer",
			store:    &mockStore{tokens: map[string]*token.Token{"valid-key": validToken}},
			auth:     "BEARER valid-key",
			wantCode: http.StatusOK,
		},
		{
			name:     "store error",
			store:    &mockStore{err: errors.New("redis connection refused")},
			auth:     "Bearer some-key",
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotToken *token.Token
			handler := Auth(tt.store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotToken, _ = token.FromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
			if tt.wantToken {
				require.NotNil(t, gotToken)
				assert.Equal(t, "valid-key", gotToken.APIKey)
			}
		})
	}
}
