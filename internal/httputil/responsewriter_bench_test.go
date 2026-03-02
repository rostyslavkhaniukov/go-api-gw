package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// tripleWrapNew simulates the old code path: Logging, Metrics, and
// CircuitBreaker each call NewResponseWriter unconditionally.
func tripleWrapNew(w http.ResponseWriter) (*ResponseWriter, *ResponseWriter) {
	rw1 := NewResponseWriter(w)   // Logging
	_ = NewResponseWriter(rw1)    // Metrics (intermediate, not needed)
	rw3 := NewResponseWriter(rw1) // CircuitBreaker
	return rw1, rw3
}

// tripleWrapReuse simulates the new code path: Logging creates the wrapper,
// Metrics and CircuitBreaker type-assert and reuse it.
func tripleWrapReuse(w http.ResponseWriter) *ResponseWriter {
	// Logging — always creates
	rw := NewResponseWriter(w)

	// Metrics — reuses
	if existing, ok := http.ResponseWriter(rw).(*ResponseWriter); ok {
		_ = existing // reused
	}

	// CircuitBreaker — reuses
	if existing, ok := http.ResponseWriter(rw).(*ResponseWriter); ok {
		_ = existing // reused
	}

	return rw
}

// BenchmarkResponseWriter_TripleNew measures 3 heap allocations via
// NewResponseWriter per simulated request (the old Logging → Metrics →
// CircuitBreaker chain).
func BenchmarkResponseWriter_TripleNew(b *testing.B) {
	w := httptest.NewRecorder()
	body := []byte("ok")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rw1, rw3 := tripleWrapNew(w)

		rw3.WriteHeader(http.StatusOK)
		rw3.Write(body)
		_ = rw1.StatusCode

		w.Body.Reset()
	}
}

// BenchmarkResponseWriter_TypeAssertReuse measures the new code path:
// one allocation by Logging, Metrics and CircuitBreaker reuse via type-assert.
func BenchmarkResponseWriter_TypeAssertReuse(b *testing.B) {
	w := httptest.NewRecorder()
	body := []byte("ok")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rw := tripleWrapReuse(w)

		rw.WriteHeader(http.StatusOK)
		rw.Write(body)
		_ = rw.StatusCode

		w.Body.Reset()
	}
}

// --- parallel variants under concurrency ---

func BenchmarkResponseWriter_TripleNew_Parallel(b *testing.B) {
	body := []byte("ok")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			rw1, rw3 := tripleWrapNew(w)
			rw3.WriteHeader(http.StatusOK)
			rw3.Write(body)
			_ = rw1.StatusCode
			w.Body.Reset()
		}
	})
}

func BenchmarkResponseWriter_TypeAssertReuse_Parallel(b *testing.B) {
	body := []byte("ok")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			rw := tripleWrapReuse(w)
			rw.WriteHeader(http.StatusOK)
			rw.Write(body)
			_ = rw.StatusCode
			w.Body.Reset()
		}
	})
}
