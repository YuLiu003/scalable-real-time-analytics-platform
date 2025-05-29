package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"visualization-go/models"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns a health status for the service
func HealthCheck(c *gin.Context) {
	// Create response
	status := models.ServiceStatus{
		Status:    "ok",
		Service:   "visualization",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate health response",
		})
		return
	}

	// Set content type and send response
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", jsonBytes)
}
