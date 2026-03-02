package proxy

import (
	"errors"
	"net/http"

	"github.com/sony/gobreaker/v2"

	"api-gw/internal/httputil"
)

// WithCircuitBreaker wraps an http.Handler with a circuit breaker.
// When the circuit is open, requests get a 503 Service Unavailable.
func WithCircuitBreaker(handler http.Handler, settings gobreaker.Settings) http.Handler {
	cb := gobreaker.NewCircuitBreaker[struct{}](settings)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := cb.Execute(func() (struct{}, error) {
			rw, ok := w.(*httputil.ResponseWriter)
			if !ok {
				rw = httputil.NewResponseWriter(w)
			}
			handler.ServeHTTP(rw, r)

			// Treat 5xx responses as errors for the circuit breaker
			if rw.StatusCode >= http.StatusInternalServerError {
				return struct{}{}, &backendError{code: rw.StatusCode}
			}
			return struct{}{}, nil
		})

		if err != nil {
			// If the error is from an open circuit breaker, return 503
			var be *backendError
			if !errors.As(err, &be) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":{"code":503,"message":"service unavailable (circuit breaker open)"}}`))
			}
		}
	})
}

type backendError struct {
	code int
}

func (e *backendError) Error() string {
	return http.StatusText(e.code)
}
