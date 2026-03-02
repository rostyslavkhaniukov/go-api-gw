package proxy

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"api-gw/internal/config"
	"api-gw/internal/middleware"
)

// New creates a reverse proxy handler for the given backend URL.
func New(backendURL string, cfg config.ProxyConfig) (http.Handler, error) {
	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, fmt.Errorf("parse backend URL %q: %w", backendURL, err)
	}

	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: cfg.DialTimeout}).DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		DisableCompression:    true, // Let client negotiate compression directly with backend
		ForceAttemptHTTP2:     true,
	}

	proxy := &httputil.ReverseProxy{
		Transport:     transport,
		FlushInterval: -1, // Stream responses immediately instead of buffering
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Authorization")
			// SetXForwarded sets X-Forwarded-{For,Host,Proto} from the
			// inbound request, replacing any client-supplied values.
			pr.SetXForwarded()
		},
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if r.Context().Err() != nil {
			return // client disconnected, nothing to log or write
		}

		// MaxBytesReader error means the client sent a body exceeding the limit.
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(`{"error":{"code":413,"message":"request body too large"}}`))
			return
		}

		slog.Error("proxy error",
			"error", err,
			"path", r.URL.Path,
			"request_id", middleware.GetRequestID(r.Context()),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":502,"message":"bad gateway"}}`))
	}

	return proxy, nil
}
