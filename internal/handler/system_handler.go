package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// SystemHandler provides endpoints for health and readiness checks
type SystemHandler struct {
	dbPool      *pgxpool.Pool
	redisClient *redis.Client
}

// NewSystemHandler creates a new SystemHandler
func NewSystemHandler(dbPool *pgxpool.Pool, redisClient *redis.Client) *SystemHandler {
	return &SystemHandler{
		dbPool:      dbPool,
		redisClient: redisClient,
	}
}

// Health handles liveness probe: GET /health
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// Ready handles readiness probe: GET /ready
func (h *SystemHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbStatus := "down"
	if h.dbPool != nil {
		if err := h.dbPool.Ping(ctx); err == nil {
			dbStatus = "up"
		}
	}

	redisStatus := "down"
	if h.redisClient != nil {
		if err := h.redisClient.Ping(ctx).Err(); err == nil {
			redisStatus = "up"
		}
	}

	isReady := dbStatus == "up" && redisStatus == "up"
	statusCode := http.StatusOK
	statusText := "ready"
	if !isReady {
		statusCode = http.StatusServiceUnavailable
		statusText = "unhealthy"
	}

	WriteJSON(w, statusCode, map[string]string{
		"status":   statusText,
		"database": dbStatus,
		"redis":    redisStatus,
	})
}
