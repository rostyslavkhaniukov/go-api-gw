package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"api-gw/internal/config"
	"api-gw/internal/proxy"
	"api-gw/internal/ratelimiter"
	"api-gw/internal/token"
)

// setupFunctional creates a full gateway stack from a YAML config string.
// It returns the router, the token store (to seed tokens), and a cleanup function.
func setupFunctional(t *testing.T, yamlConfig string, upstreamServers map[string]*httptest.Server) http.Handler {
	t.Helper()

	// Write temp config
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlConfig), 0o644))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	// Replace upstream URLs with test server URLs
	for i := range cfg.Upstreams {
		if srv, ok := upstreamServers[cfg.Upstreams[i].Prefix]; ok {
			cfg.Upstreams[i].URL = srv.URL
		}
	}

	// Redis (miniredis)
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	// Domain
	tokenStore := token.NewRedisStore(client)
	limiter := ratelimiter.NewRedisLimiter(client, cfg.RateLimit.Window)

	// Seed a test token
	testToken := &token.Token{
		APIKey:        "test-key",
		RateLimit:     100,
		ExpiresAt:     time.Now().Add(time.Hour),
		AllowedRoutes: []string{"/api/v1/users/*", "/api/v1/products/*"},
	}
	require.NoError(t, tokenStore.Set(context.Background(), testToken))

	restrictedToken := &token.Token{
		APIKey:        "users-only-key",
		RateLimit:     100,
		ExpiresAt:     time.Now().Add(time.Hour),
		AllowedRoutes: []string{"/api/v1/users/*"},
	}
	require.NoError(t, tokenStore.Set(context.Background(), restrictedToken))

	lowRateToken := &token.Token{
		APIKey:        "low-rate-key",
		RateLimit:     2,
		ExpiresAt:     time.Now().Add(time.Hour),
		AllowedRoutes: []string{"/api/v1/users/*", "/api/v1/products/*"},
	}
	require.NoError(t, tokenStore.Set(context.Background(), lowRateToken))

	// Build upstreams
	entries, err := proxy.BuildUpstreams(cfg.Upstreams, cfg.Proxy, cfg.CircuitBreaker)
	require.NoError(t, err)
	handler := proxy.NewDispatcher(entries)

	return NewRouter(tokenStore, limiter, handler, 10<<20)
}

func TestFunctional_RoutingToCorrectUpstream(t *testing.T) {
	usersServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"users","path":"` + r.URL.Path + `"}`))
	}))
	defer usersServer.Close()

	productsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"products","path":"` + r.URL.Path + `"}`))
	}))
	defer productsServer.Close()

	yamlConfig := `
server:
  port: 8080
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
    strip_prefix: false
  - prefix: /api/v1/products
    url: http://placeholder
    strip_prefix: false
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users":    usersServer,
		"/api/v1/products": productsServer,
	})

	tests := []struct {
		name       string
		path       string
		wantSvc    string
		wantStatus int
	}{
		{"users_detail", "/api/v1/users/42", "users", http.StatusOK},
		{"users_subpath", "/api/v1/users/1", "users", http.StatusOK},
		{"products_detail", "/api/v1/products/7", "products", http.StatusOK},
		{"products_subpath", "/api/v1/products/99", "products", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Authorization", "Bearer test-key")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var resp map[string]string
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			assert.Equal(t, tt.wantSvc, resp["service"])
		})
	}
}

func TestFunctional_StripPrefix(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"stripped":true}`))
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
    strip_prefix: true
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/42", receivedPath)
}

func TestFunctional_StripPrefixSubpath(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
    strip_prefix: true
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/99", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/99", receivedPath)
}

func TestFunctional_NoStripPrefix(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
    strip_prefix: false
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/api/v1/users/42", receivedPath)
}

func TestFunctional_AuthRequired(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	// No auth header
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestFunctional_InvalidToken(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.Header.Set("Authorization", "Bearer nonexistent-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestFunctional_RouteForbidden(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
  - prefix: /api/v1/products
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users":    backend,
		"/api/v1/products": backend,
	})

	// users-only-key trying to access products
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/1", nil)
	req.Header.Set("Authorization", "Bearer users-only-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestFunctional_RateLimiting(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	// low-rate-key has limit of 2
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
		req.Header.Set("Authorization", "Bearer low-rate-key")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "request %d", i+1)
		assert.Equal(t, "2", rec.Header().Get("X-RateLimit-Limit"), "request %d", i+1)
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.Header.Set("Authorization", "Bearer low-rate-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestFunctional_RequestIDPropagation(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	// Without existing request ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))

	// With existing request ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("X-Request-ID", "custom-id-123")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, "custom-id-123", rec.Header().Get("X-Request-ID"))
}

func TestFunctional_MultipleHTTPMethods(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/users/1", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestFunctional_NoUpstreamMatch(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	yamlConfig := `
upstreams:
  - prefix: /api/v1/users
    url: http://placeholder
rate_limit:
  window: 60s
`
	router := setupFunctional(t, yamlConfig, map[string]*httptest.Server{
		"/api/v1/users": backend,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}
