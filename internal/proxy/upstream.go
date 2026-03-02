package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/sony/gobreaker/v2"

	"api-gw/internal/config"
)

// UpstreamEntry pairs a path prefix with a ready-to-use HTTP handler
// (reverse proxy wrapped with a circuit breaker).
type UpstreamEntry struct {
	Prefix      string
	StripPrefix bool
	Handler     http.Handler
}

// BuildUpstreams creates an UpstreamEntry per configured upstream.
// Entries are sorted longest-prefix-first so the dispatcher matches
// the most specific prefix.
func BuildUpstreams(upstreams []config.UpstreamConfig, proxyCfg config.ProxyConfig, cbCfg config.CircuitBreakerConfig) ([]UpstreamEntry, error) {
	entries := make([]UpstreamEntry, 0, len(upstreams))
	for _, u := range upstreams {
		h, err := New(u.URL, proxyCfg)
		if err != nil {
			return nil, fmt.Errorf("upstream %q: %w", u.Prefix, err)
		}

		cb := WithCircuitBreaker(h, gobreaker.Settings{
			Name:    u.Prefix,
			Timeout: cbCfg.Timeout,
			// Trip when consecutive failures reach the threshold (e.g. 5 means
			// the 5th consecutive failure opens the circuit).
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= cbCfg.MaxFailures
			},
		})

		entries = append(entries, UpstreamEntry{
			Prefix:      u.Prefix,
			StripPrefix: u.StripPrefix,
			Handler:     cb,
		})
	}

	slices.SortFunc(entries, func(a, b UpstreamEntry) int {
		return len(b.Prefix) - len(a.Prefix)
	})

	return entries, nil
}

// NewDispatcher returns an http.Handler that routes requests to the
// correct upstream based on path prefix. Returns 502 if no upstream matches.
func NewDispatcher(entries []UpstreamEntry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, e := range entries {
			if r.URL.Path == e.Prefix || strings.HasPrefix(r.URL.Path, e.Prefix+"/") {
				if e.StripPrefix {
					// Shallow-copy the request and URL to avoid the overhead
					// of r.Clone which deep-copies the header map.
					r2 := new(http.Request)
					*r2 = *r
					u2 := new(url.URL)
					*u2 = *r.URL
					r2.URL = u2

					r2.URL.Path = strings.TrimPrefix(r.URL.Path, e.Prefix)
					if r2.URL.Path == "" {
						r2.URL.Path = "/"
					}
					if r.URL.RawPath != "" {
						r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, e.Prefix)
						if r2.URL.RawPath == "" {
							r2.URL.RawPath = "/"
						}
					}
					e.Handler.ServeHTTP(w, r2)
					return
				}
				e.Handler.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":502,"message":"no upstream matched"}}`))
	})
}
