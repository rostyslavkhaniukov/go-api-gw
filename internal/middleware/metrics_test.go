package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getCounterValue(t *testing.T, counter *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	m := &dto.Metric{}
	c, err := counter.GetMetricWithLabelValues(labels...)
	require.NoError(t, err)
	require.NoError(t, c.Write(m))
	return m.GetCounter().GetValue()
}

func getHistogramCount(t *testing.T, histogram *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	m := &dto.Metric{}
	obs, err := histogram.GetMetricWithLabelValues(labels...)
	require.NoError(t, err)
	metric, ok := obs.(prometheus.Metric)
	require.True(t, ok, "histogram observer does not implement prometheus.Metric")
	require.NoError(t, metric.Write(m))
	return m.GetHistogram().GetSampleCount()
}

func TestMetrics_IncrementsCounter(t *testing.T) {
	// Without chi context, route pattern is "unknown"
	before := getCounterValue(t, httpRequestsTotal, "GET", "unknown", "200")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics-test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := getCounterValue(t, httpRequestsTotal, "GET", "unknown", "200")
	assert.InDelta(t, 1, after-before, 0)
}

func TestMetrics_RecordsDuration(t *testing.T) {
	before := getHistogramCount(t, httpRequestDuration, "POST", "unknown")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/duration-test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := getHistogramCount(t, httpRequestDuration, "POST", "unknown")
	assert.Equal(t, uint64(1), after-before)
}

func TestMetrics_CapturesStatusCode(t *testing.T) {
	before := getCounterValue(t, httpRequestsTotal, "DELETE", "unknown", "404")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/status-test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := getCounterValue(t, httpRequestsTotal, "DELETE", "unknown", "404")
	assert.InDelta(t, 1, after-before, 0)
}

func TestMetrics_PassesThroughResponse(t *testing.T) {
	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/passthrough-test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
}
