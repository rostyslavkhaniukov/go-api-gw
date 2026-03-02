package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"api-gw/internal/token"
)

func TestRouteCheck(t *testing.T) {
	tests := []struct {
		name          string
		allowedRoutes []string
		path          string
		hasToken      bool
		wantCode      int
	}{
		{"allowed", []string{"/api/v1/users/*", "/api/v1/products/*"}, "/api/v1/users/123", true, http.StatusOK},
		{"forbidden", []string{"/api/v1/users/*"}, "/api/v1/admin/settings", true, http.StatusForbidden},
		{"no token", nil, "/api/v1/users/1", false, http.StatusForbidden},
		{"empty allowed routes", []string{}, "/api/v1/users/1", true, http.StatusForbidden},
		{"multi level path", []string{"/api/v1/users/*"}, "/api/v1/users/123/posts/456", true, http.StatusOK},
		{"exact match", []string{"/api/v1/users"}, "/api/v1/users", true, http.StatusOK},
		{"subpath without wildcard", []string{"/api/v1/users"}, "/api/v1/users/123", true, http.StatusForbidden},
		// Path traversal attempts.
		{"traversal dot-dot", []string{"/api/v1/users/*"}, "/api/v1/users/../admin/delete", true, http.StatusForbidden},
		{"traversal double dot-dot", []string{"/api/v1/users/*"}, "/api/v1/users/../../secret", true, http.StatusForbidden},
		{"traversal dot-slash", []string{"/api/v1/users/*"}, "/api/v1/users/./../../etc/passwd", true, http.StatusForbidden},
		{"traversal encoded", []string{"/api/v1/users/*"}, "/api/v1/users/%2e%2e/admin", true, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RouteCheck(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.hasToken {
				tok := &token.Token{
					APIKey:        "key",
					RateLimit:     100,
					ExpiresAt:     time.Now().Add(time.Hour),
					AllowedRoutes: tt.allowedRoutes,
				}
				req = req.WithContext(token.NewContext(req.Context(), tok))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}
