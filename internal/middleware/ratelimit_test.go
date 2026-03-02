package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"api-gw/internal/ratelimiter"
	"api-gw/internal/token"
)

type mockLimiter struct {
	result ratelimiter.Result
	err    error
}

func (m *mockLimiter) Allow(_ context.Context, _ string, _ int) (ratelimiter.Result, error) {
	return m.result, m.err
}

func TestRateLimit(t *testing.T) {
	tests := []struct {
		name        string
		limiter     *mockLimiter
		token       *token.Token
		wantCode    int
		wantHeaders map[string]string
	}{
		{
			name:     "allowed",
			limiter:  &mockLimiter{result: ratelimiter.Result{Allowed: true, Limit: 50, Remaining: 42, ResetAt: time.Now().Add(time.Minute).Unix()}},
			token:    &token.Token{APIKey: "key", RateLimit: 50, ExpiresAt: time.Now().Add(time.Hour)},
			wantCode: http.StatusOK,
			wantHeaders: map[string]string{
				"X-RateLimit-Limit":     "50",
				"X-RateLimit-Remaining": "42",
			},
		},
		{
			name:     "exceeded",
			limiter:  &mockLimiter{result: ratelimiter.Result{Allowed: false, Limit: 10, Remaining: 0, ResetAt: time.Now().Add(time.Minute).Unix()}},
			token:    &token.Token{APIKey: "key", RateLimit: 10, ExpiresAt: time.Now().Add(time.Hour)},
			wantCode: http.StatusTooManyRequests,
			wantHeaders: map[string]string{
				"X-RateLimit-Limit":     "10",
				"X-RateLimit-Remaining": "0",
			},
		},
		{
			name:     "no token in context",
			limiter:  &mockLimiter{result: ratelimiter.Result{Allowed: true}},
			wantCode: http.StatusInternalServerError,
		},
		{
			name:     "limiter error",
			limiter:  &mockLimiter{err: errors.New("redis connection failed")},
			token:    &token.Token{APIKey: "key", RateLimit: 100, ExpiresAt: time.Now().Add(time.Hour)},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RateLimit(tt.limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
			if tt.token != nil {
				req = req.WithContext(token.NewContext(req.Context(), tt.token))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
			for header, want := range tt.wantHeaders {
				assert.Equal(t, want, rec.Header().Get(header), "header %s", header)
			}
			if len(tt.wantHeaders) > 0 {
				assert.NotEmpty(t, rec.Header().Get("X-RateLimit-Reset"))
			}
		})
	}
}
