package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesNew(t *testing.T) {
	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, respID)
	assert.NotEmpty(t, ctxID)
	assert.Equal(t, respID, ctxID)
}

func TestRequestID_PreservesExisting(t *testing.T) {
	existingID := "my-custom-request-id-123"

	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, existingID, rec.Header().Get(RequestIDHeader))
	assert.Equal(t, existingID, ctxID)
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ids := make(map[string]bool)
	for range 100 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		id := rec.Header().Get(RequestIDHeader)
		require.False(t, ids[id], "duplicate request ID generated: %q", id)
		ids[id] = true
	}
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	id := GetRequestID(req.Context())
	assert.Empty(t, id)
}

func TestRequestID_OversizedIDRejected(t *testing.T) {
	// ID longer than 64 characters should be rejected and a new one generated
	longID := "aaaaaaaaaabbbbbbbbbbccccccccccddddddddddeeeeeeeeeeffffffffff12345"
	if len(longID) <= 64 {
		longID += "extra"
	}

	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, longID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get(RequestIDHeader)
	assert.NotEqual(t, longID, respID)
	assert.NotEqual(t, longID, ctxID)
	assert.NotEmpty(t, respID)
}

func TestRequestID_SpecialCharactersRejected(t *testing.T) {
	invalidIDs := []string{
		"id with spaces",
		"id<script>",
		"id\nwith\nnewlines",
		"id;drop table",
		"../../../../etc/passwd",
	}

	for _, invalidID := range invalidIDs {
		t.Run(invalidID, func(t *testing.T) {
			var ctxID string
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctxID = GetRequestID(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set(RequestIDHeader, invalidID)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			respID := rec.Header().Get(RequestIDHeader)
			assert.NotEqual(t, invalidID, respID)
			assert.NotEqual(t, invalidID, ctxID)
			assert.NotEmpty(t, respID)
		})
	}
}
