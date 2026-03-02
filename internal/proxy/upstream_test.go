package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"api-gw/internal/config"
)

var testCBCfg = config.CircuitBreakerConfig{MaxFailures: 5}

func TestBuildUpstreams_Empty(t *testing.T) {
	entries, err := BuildUpstreams(nil, testProxyCfg, testCBCfg)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestBuildUpstreams_InvalidURL(t *testing.T) {
	_, err := BuildUpstreams([]config.UpstreamConfig{
		{Prefix: "/api", URL: "://invalid"},
	}, testProxyCfg, testCBCfg)
	assert.Error(t, err)
}

func TestBuildUpstreams_StripPrefixFlag(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	entries, err := BuildUpstreams([]config.UpstreamConfig{
		{Prefix: "/api/v1/users", URL: s.URL, StripPrefix: true},
		{Prefix: "/api/v1/products", URL: s.URL, StripPrefix: false},
	}, testProxyCfg, testCBCfg)
	require.NoError(t, err)

	stripByPrefix := map[string]bool{}
	for _, e := range entries {
		stripByPrefix[e.Prefix] = e.StripPrefix
	}

	assert.True(t, stripByPrefix["/api/v1/users"])
	assert.False(t, stripByPrefix["/api/v1/products"])
}

func TestBuildUpstreams_SortOrder(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s2.Close()

	entries, err := BuildUpstreams([]config.UpstreamConfig{
		{Prefix: "/api", URL: s1.URL},
		{Prefix: "/api/v1/users", URL: s2.URL},
	}, testProxyCfg, testCBCfg)
	require.NoError(t, err)

	require.Len(t, entries, 2)
	// Longest prefix should come first.
	assert.Equal(t, "/api/v1/users", entries[0].Prefix)
	assert.Equal(t, "/api", entries[1].Prefix)
}

func TestDispatcher_RoutesToCorrectUpstream(t *testing.T) {
	entries := []UpstreamEntry{
		{
			Prefix: "/api/v1/users",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("users"))
			}),
		},
		{
			Prefix: "/api/v1/products",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("products"))
			}),
		},
	}

	dispatcher := NewDispatcher(entries)

	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/users/1", "users"},
		{"/api/v1/users", "users"},
		{"/api/v1/products/42", "products"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			dispatcher.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			body, _ := io.ReadAll(rec.Body)
			assert.Equal(t, tt.want, string(body))
		})
	}
}

func TestDispatcher_NoMatch(t *testing.T) {
	entries := []UpstreamEntry{
		{
			Prefix: "/api/v1/users",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		},
	}

	dispatcher := NewDispatcher(entries)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()
	dispatcher.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestDispatcher_NoPartialSegmentMatch(t *testing.T) {
	entries := []UpstreamEntry{
		{
			Prefix: "/api/v1/user",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("user"))
			}),
		},
	}

	dispatcher := NewDispatcher(entries)

	// "/api/v1/userprofiles" must NOT match prefix "/api/v1/user"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/userprofiles", nil)
	rec := httptest.NewRecorder()
	dispatcher.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestDispatcher_LongestPrefixWins(t *testing.T) {
	entries := []UpstreamEntry{
		{
			Prefix: "/api/v1/users/admin",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("admin"))
			}),
		},
		{
			Prefix: "/api/v1/users",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("users"))
			}),
		},
	}

	dispatcher := NewDispatcher(entries)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/admin/1", nil)
	rec := httptest.NewRecorder()
	dispatcher.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	assert.Equal(t, "admin", string(body))
}

func TestDispatcher_StripPrefix(t *testing.T) {
	var receivedPath string
	entries := []UpstreamEntry{
		{
			Prefix:      "/api/v1/users",
			StripPrefix: true,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("stripped"))
			}),
		},
	}

	dispatcher := NewDispatcher(entries)

	tests := []struct {
		name     string
		path     string
		wantPath string
	}{
		{"with_suffix", "/api/v1/users/42", "/42"},
		{"exact_match", "/api/v1/users", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPath = ""
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			dispatcher.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantPath, receivedPath)
			body, _ := io.ReadAll(rec.Body)
			assert.Equal(t, "stripped", string(body))
		})
	}
}

func TestDispatcher_StripPrefixPreservesRawPath(t *testing.T) {
	var receivedPath, receivedRawPath string
	entries := []UpstreamEntry{
		{
			Prefix:      "/api/v1/users",
			StripPrefix: true,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				receivedRawPath = r.URL.RawPath
				w.WriteHeader(http.StatusOK)
			}),
		},
	}

	dispatcher := NewDispatcher(entries)

	// Create a request with percent-encoded characters in the path
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/hello%20world", nil)
	// Manually set RawPath to simulate percent-encoded path
	req.URL = &url.URL{
		Path:    "/api/v1/users/hello world",
		RawPath: "/api/v1/users/hello%20world",
	}
	rec := httptest.NewRecorder()
	dispatcher.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/hello world", receivedPath)
	assert.Equal(t, "/hello%20world", receivedRawPath)
}
