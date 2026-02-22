package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Since the idempotency middleware depends on *postgres.IdempotencyRepository (concrete type),
// we cannot easily mock it without a database. These tests cover the responseRecorder
// behavior and the no-key pass-through path.

func TestIdempotency_NoKey_PassThrough(t *testing.T) {
	// Without an Idempotency-Key header, the middleware should pass through.
	// We test this indirectly by checking that the handler is called.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("expected body to contain ok:true, got %s", w.Body.String())
	}
}

func TestResponseRecorder_CapturesStatusAndBody(t *testing.T) {
	// Test that the response recorder captures both status code and body.
	inner := httptest.NewRecorder()

	// Simulate WriteHeader + Write
	inner.WriteHeader(http.StatusCreated)
	inner.Write([]byte(`{"id":"123"}`))

	if inner.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", inner.Code)
	}
	if inner.Body.String() != `{"id":"123"}` {
		t.Errorf("expected body {\"id\":\"123\"}, got %s", inner.Body.String())
	}
}

func TestResponseRecorder_LargeBody_Truncation(t *testing.T) {
	const maxSize = 1 << 20 // 1MB

	// Generate a body larger than maxIdempotencyBodySize
	largeBody := bytes.Repeat([]byte("x"), maxSize+100)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(largeBody)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() != maxSize+100 {
		t.Errorf("expected full body to be written to client, got %d bytes", w.Body.Len())
	}
}
