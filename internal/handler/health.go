package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
// These are essential for container orchestration (Kubernetes, Docker).
type HealthHandler struct {
	// We'll add database and Redis checkers here later
	// dbChecker    func() error
	// redisChecker func() error
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status  string            `json:"status"`
	Version string            `json:"version,omitempty"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// Health returns a simple health check.
// Used for basic "is the service running" checks.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
	})
}

// Ready checks if the service is ready to accept traffic.
// This checks all dependencies (database, Redis, etc.).
func (h *HealthHandler) Ready(c *gin.Context) {
	checks := make(map[string]string)
	allHealthy := true

	// TODO: Add database health check
	// if err := h.dbChecker(); err != nil {
	//     checks["database"] = "unhealthy: " + err.Error()
	//     allHealthy = false
	// } else {
	//     checks["database"] = "healthy"
	// }

	// TODO: Add Redis health check
	// if err := h.redisChecker(); err != nil {
	//     checks["redis"] = "unhealthy: " + err.Error()
	//     allHealthy = false
	// } else {
	//     checks["redis"] = "healthy"
	// }

	// For now, just mark as healthy
	checks["database"] = "not configured"
	checks["redis"] = "not configured"

	status := http.StatusOK
	statusText := "ready"
	if !allHealthy {
		status = http.StatusServiceUnavailable
		statusText = "not ready"
	}

	c.JSON(status, HealthResponse{
		Status: statusText,
		Checks: checks,
	})
}

// Live checks if the service is alive.
// This is a simple check - if the server responds, it's alive.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "alive",
	})
}

// RegisterRoutes registers health check routes on a Gin router group.
func (h *HealthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	health := rg.Group("/health")
	health.GET("", h.Health)
	health.GET("/ready", h.Ready)
	health.GET("/live", h.Live)
}
