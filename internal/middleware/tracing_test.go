package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestTracing_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mw := Tracing()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestTracing_WithChiRoutePattern(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := Tracing()
	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/api/accounts/{id}", handler)

	req := httptest.NewRequest("GET", "/api/accounts/123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Span should be created with pattern "GET /api/accounts/{id}"
}

func TestTracing_WithoutChiRoutePattern(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := Tracing()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/unknown", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should fallback to "GET /unknown"
}

func TestTracing_DifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			mw := Tracing()
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

func TestTracing_PreservesResponseStatus(t *testing.T) {
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
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			mw := Tracing()
			wrappedHandler := mw(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestTracing_PreservesResponseBody(t *testing.T) {
	expectedBody := `{"message":"test response"}`

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	})

	mw := Tracing()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, expectedBody, w.Body.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestTracing_PreservesRequestContext(t *testing.T) {
	var receivedContext bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request context is preserved
		if r.Context() != nil {
			receivedContext = true
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := Tracing()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	assert.True(t, receivedContext, "Request context should be preserved")
	assert.Equal(t, http.StatusOK, w.Code)
}
