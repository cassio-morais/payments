package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthController struct {
	pool   *pgxpool.Pool
	redis  *redis.Client
}

func NewHealthController(pool *pgxpool.Pool, redis *redis.Client) *HealthController {
	return &HealthController{pool: pool, redis: redis}
}

func (h *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthController) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *HealthController) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "database unavailable",
		})
		return
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "redis unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
