package handlers

import (
	"net/http"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/models"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns a health status for the service
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.ServiceStatus{
		Status:    "ok",
		Service:   "visualization",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
