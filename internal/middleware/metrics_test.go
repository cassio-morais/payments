package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cassiomorais/payments/internal/infrastructure/observability"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_Success(t *testing.T) {
	// Create metrics with test registry
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics("test", reg)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	mw := Metrics(metrics)

	// Create test request with chi route context
	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/api/test", handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Verify metrics were recorded
	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	var foundRequestsTotal, foundDuration bool
	for _, mf := range metricFamilies {
		if *mf.Name == "test_http_requests_total" {
			foundRequestsTotal = true
			assert.Greater(t, len(mf.Metric), 0)
		}
		if *mf.Name == "test_http_request_duration_seconds" {
			foundDuration = true
			assert.Greater(t, len(mf.Metric), 0)
		}
	}

	assert.True(t, foundRequestsTotal, "http_requests_total metric should be recorded")
	assert.True(t, foundDuration, "http_request_duration metric should be recorded")
}

func TestMetrics_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			metrics := observability.NewMetrics("test", reg)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			mw := Metrics(metrics)
			r := chi.NewRouter()
			r.Use(mw)
			r.Get("/test", handler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestMetrics_DifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			metrics := observability.NewMetrics("test", reg)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			mw := Metrics(metrics)
			r := chi.NewRouter()
			r.Use(mw)
			r.MethodFunc(method, "/test", handler)

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMetrics_RoutePattern(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics("test", reg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := Metrics(metrics)
	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/accounts/{id}", handler)

	// Make request with actual ID
	req := httptest.NewRequest("GET", "/accounts/123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Metrics should use the route pattern, not the actual path
}

func TestMetrics_NoRoutePattern(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics("test", reg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := Metrics(metrics)
	wrappedHandler := mw(handler)

	// Request without chi routing (no route pattern)
	req := httptest.NewRequest("GET", "/unknown", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStatusWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

	sw.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, sw.statusCode)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestStatusWriter_DefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Write without calling WriteHeader
	sw.Write([]byte("test"))

	// Should keep default status
	assert.Equal(t, http.StatusOK, sw.statusCode)
}
