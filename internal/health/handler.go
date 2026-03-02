package health

import (
	"context"
	"encoding/json"
	"net/http"
)

type response struct {
	Status string `json:"status"`
}

// Checker reports whether a dependency is healthy.
type Checker interface {
	Ping(ctx context.Context) error
}

// Healthz returns a simple liveness check.
func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response{Status: "ok"})
	})
}

// Readyz returns a readiness check that pings the given dependency.
func Readyz(checker Checker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := checker.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(response{Status: "not ready"})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response{Status: "ready"})
	})
}
