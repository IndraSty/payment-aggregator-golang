package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

type HealthResponse struct {
	Status     string            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Version    string            `json:"version"`
	Components map[string]string `json:"components"`
}

// Health godoc
// @Summary Health check — checks all dependencies
// @Tags system
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Health(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()

	components := make(map[string]string)
	overallHealthy := true

	// Check PostgreSQL
	if err := h.db.Ping(ctx); err != nil {
		components["postgres"] = "unhealthy: " + err.Error()
		overallHealthy = false
	} else {
		components["postgres"] = "healthy"
	}

	// Check Redis
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		components["redis"] = "unhealthy: " + err.Error()
		overallHealthy = false
	} else {
		components["redis"] = "healthy"
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !overallHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	return c.JSON(httpStatus, HealthResponse{
		Status:     status,
		Timestamp:  time.Now(),
		Version:    "1.0.0",
		Components: components,
	})
}
