package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	return httprate.Limit(
		requestsPerMinute,
		1*time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
				"code":  "rate_limit",
			})
		}),
	)
}
