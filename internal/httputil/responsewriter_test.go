package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponseWriter_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	assert.Equal(t, http.StatusOK, rw.StatusCode)
}

func TestWriteHeader_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, rw.StatusCode)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWriteHeader_IgnoresSecondCall(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusInternalServerError) // should be ignored

	assert.Equal(t, http.StatusCreated, rw.StatusCode)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestWrite_ImplicitWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	n, err := rw.Write([]byte("hello"))

	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, http.StatusOK, rw.StatusCode)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestWrite_AfterExplicitWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusAccepted)
	n, err := rw.Write([]byte("body"))

	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, http.StatusAccepted, rw.StatusCode)
	assert.Equal(t, "body", rec.Body.String())
}

func TestWrite_MultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	_, _ = rw.Write([]byte("hello "))
	_, _ = rw.Write([]byte("world"))

	assert.Equal(t, http.StatusOK, rw.StatusCode)
	assert.Equal(t, "hello world", rec.Body.String())
}

func TestFlush_DelegatesToUnderlying(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	// httptest.ResponseRecorder implements http.Flusher
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("streamed"))
	rw.Flush()

	assert.True(t, rec.Flushed)
}

func TestFlush_NoopWhenNotFlusher(t *testing.T) {
	// Use a minimal ResponseWriter that does not implement http.Flusher.
	rw := NewResponseWriter(&nonFlusherWriter{header: http.Header{}})

	// Should not panic.
	rw.Flush()
}

func TestUnwrap_ReturnsUnderlying(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	underlying := rw.Unwrap()

	assert.Equal(t, rec, underlying)
}

func TestWriteHeader_ThenWriteDoesNotOverride(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusBadRequest)
	_, _ = rw.Write([]byte("error"))

	// Write must not reset status to 200
	assert.Equal(t, http.StatusBadRequest, rw.StatusCode)
}

// nonFlusherWriter is a minimal http.ResponseWriter that does not implement http.Flusher.
type nonFlusherWriter struct {
	header http.Header
}

func (w *nonFlusherWriter) Header() http.Header        { return w.header }
func (w *nonFlusherWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *nonFlusherWriter) WriteHeader(int)             {}
