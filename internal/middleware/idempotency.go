package middleware

import (
	"bytes"
	"net/http"
	"time"

	"github.com/cassiomorais/payments/internal/repository/postgres"
)

const maxIdempotencyBodySize = 1 << 20 // 1MB

// Idempotency is middleware that caches responses by Idempotency-Key header.
func Idempotency(idempotencyRepo *postgres.IdempotencyRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				// No idempotency key — pass through.
				next.ServeHTTP(w, r)
				return
			}

			entry, err := idempotencyRepo.Get(r.Context(), key)
			if err == nil && entry != nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotency-Replayed", "true")
				w.WriteHeader(entry.ResponseStatus)
				w.Write([]byte(entry.ResponseBody))
				return
			}

			// Wrap the response writer to capture the response.
			rec := &responseRecorder{ResponseWriter: w, body: &bytes.Buffer{}, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Cache the response (only for success/client-error status codes, and if body not too large).
			if rec.statusCode >= 200 && rec.statusCode < 500 && rec.body.Len() <= maxIdempotencyBodySize {
				now := time.Now()
				idempotencyRepo.Set(r.Context(), &postgres.IdempotencyEntry{
					Key:            key,
					ResponseBody:   rec.body.String(),
					ResponseStatus: rec.statusCode,
					CreatedAt:      now,
					ExpiresAt:      now.Add(24 * time.Hour),
				})
			}
		})
	}
}

// responseRecorder captures the response status code and body.
type responseRecorder struct {
	http.ResponseWriter
	statusCode    int
	body          *bytes.Buffer
	bodyTruncated bool
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.bodyTruncated {
		if r.body.Len()+len(b) > maxIdempotencyBodySize {
			r.bodyTruncated = true
		} else {
			r.body.Write(b)
		}
	}
	return r.ResponseWriter.Write(b)
}
