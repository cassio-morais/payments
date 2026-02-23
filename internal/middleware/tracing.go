package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"github.com/go-chi/chi/v5"
)

func Tracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap the next handler to extract chi's route pattern after chi routing completes
			wrappedNext := http.HandlerFunc(func(w2 http.ResponseWriter, r2 *http.Request) {
				// Extract chi's matched route pattern
				// This is set by chi's routing logic before the handler is called
				rctx := chi.RouteContext(r2.Context())
				var operation string

				if rctx != nil && rctx.RoutePattern() != "" {
					// Use chi's matched pattern for cardinality-friendly per-endpoint metrics
					// e.g., "GET /api/v1/accounts/{id}" instead of "GET /api/v1/accounts/abc-123-def-456"
					operation = fmt.Sprintf("%s %s", r2.Method, rctx.RoutePattern())
				} else {
					// Fallback to method + path if chi hasn't matched (shouldn't happen in normal flow)
					operation = fmt.Sprintf("%s %s", r2.Method, r2.URL.Path)
				}

				// Instrument with the extracted operation name
				instrumentedHandler := otelhttp.NewHandler(next, operation)
				instrumentedHandler.ServeHTTP(w2, r2)
			})

			wrappedNext.ServeHTTP(w, r)
		})
	}
}
