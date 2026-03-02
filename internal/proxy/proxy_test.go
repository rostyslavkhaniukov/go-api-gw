package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"api-gw/internal/config"
)

var testProxyCfg = config.ProxyConfig{
	DialTimeout:           5 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
	MaxIdleConns:          128,
}

func TestProxy_ForwardsRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1", nil)
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	assert.JSONEq(t, `{"status":"ok"}`, string(body))
}

func TestProxy_ForwardsHeaders(t *testing.T) {
	var gotHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, "custom-value", gotHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "application/json", gotHeaders.Get("Accept"))
}

func TestProxy_ForwardsQueryParams(t *testing.T) {
	var gotQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=qux", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, "foo=bar&baz=qux", gotQuery)
}

func TestProxy_ForwardsPath(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, "/api/v1/users/42", gotPath)
}

func TestProxy_DifferentMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var gotMethod string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			proxy, err := New(backend.URL, testProxyCfg)
			require.NoError(t, err)

			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req)

			assert.Equal(t, method, gotMethod)
		})
	}
}

func TestProxy_ForwardsBody(t *testing.T) {
	var gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, body, gotBody)
}

func TestProxy_BackendError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("backend error"))
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestProxy_SetsHostHeader(t *testing.T) {
	var gotHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	// Host should be set to the backend's host, not the original request host
	assert.Contains(t, gotHost, "127.0.0.1")
}

func TestNew_InvalidURL(t *testing.T) {
	// url.Parse is very lenient, but we can test that New doesn't panic
	_, err := New("://invalid", testProxyCfg)
	assert.Error(t, err)
}

func TestProxy_AuthorizationHeaderNotForwarded(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Empty(t, gotAuth)
}

func TestProxy_XForwardedForNotSpoofable(t *testing.T) {
	var gotXFF string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	// The spoofed IP must be stripped; only the real client IP should remain.
	assert.NotContains(t, gotXFF, "6.6.6.6")
}

func TestProxy_MaxBytesError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy, err := New(backend.URL, testProxyCfg)
	require.NoError(t, err)

	// Create request with a body whose Content-Length (100) exceeds the MaxBytesReader limit (5).
	// The Transport advertises Content-Length: 100 to the backend but the body read
	// fails after 5 bytes, causing the proxy to invoke ErrorHandler with MaxBytesError.
	largeBody := strings.Repeat("x", 100)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 5)
	// Preserve the original Content-Length so the Transport sends it in the request
	// header. The backend will expect 100 bytes but only receive 5, causing a
	// transport-level error that triggers the proxy ErrorHandler.
	req.ContentLength = int64(len(largeBody))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Contains(t, rec.Body.String(), "request body too large")
}

func TestProxy_BackendUnreachable(t *testing.T) {
	// Use a URL that will fail to connect
	proxy, err := New("http://127.0.0.1:1", testProxyCfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "bad gateway")
}
