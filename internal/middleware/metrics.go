package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/cassiomorais/payments/internal/infrastructure/observability"
	"github.com/go-chi/chi/v5"
)

// Metrics is middleware that records HTTP request metrics.
func Metrics(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			m.HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, strconv.Itoa(ww.statusCode)).Inc()
			m.HTTPRequestDuration.WithLabelValues(r.Method, routePattern).Observe(duration)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
