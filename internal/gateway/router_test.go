package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"api-gw/internal/ratelimiter"
	"api-gw/internal/token"
)

type mockStore struct {
	tokens map[string]*token.Token
}

func (m *mockStore) Get(_ context.Context, apiKey string) (*token.Token, error) {
	return m.tokens[apiKey], nil
}

func (m *mockStore) Set(_ context.Context, tok *token.Token) error {
	m.tokens[tok.APIKey] = tok
	return nil
}

type mockLimiter struct {
	result ratelimiter.Result
}

func (m *mockLimiter) Allow(_ context.Context, _ string, _ int) (ratelimiter.Result, error) {
	return m.result, nil
}

func setupRouter(t *testing.T) http.Handler {
	t.Helper()

	store := &mockStore{tokens: map[string]*token.Token{
		"valid-key": {
			APIKey:        "valid-key",
			RateLimit:     100,
			ExpiresAt:     time.Now().Add(time.Hour),
			AllowedRoutes: []string{"/api/v1/users/*"},
		},
	}}
	limiter := &mockLimiter{result: ratelimiter.Result{Allowed: true, Limit: 100, Remaining: 99}}

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"proxied":true}`))
	})

	return NewRouter(store, limiter, backend, 10<<20)
}

func TestRouter(t *testing.T) {
	router := setupRouter(t)

	tests := []struct {
		name     string
		method   string
		path     string
		auth     string
		wantCode int
	}{
		{"proxy with auth", http.MethodPost, "/api/v1/users/1", "Bearer valid-key", http.StatusOK},
		{"no auth", http.MethodPost, "/api/v1/users/1", "", http.StatusUnauthorized},
		{"forbidden route", http.MethodPost, "/api/v1/admin/settings", "Bearer valid-key", http.StatusForbidden},
		{"GET method", http.MethodGet, "/api/v1/users/1", "Bearer valid-key", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}
