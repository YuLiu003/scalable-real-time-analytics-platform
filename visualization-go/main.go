package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"visualization-go/config"
	"visualization-go/handlers"
	"visualization-go/middleware"
	"visualization-go/websocket"
)

func main() {
	// Initialize configuration
	cfg := &config.Config{
		DataServiceURL: getEnv("DATA_SERVICE_URL", "http://storage-layer-go-service:5002"),
		MaxDataPoints:  100,
	}

	// Convert MaxDataPoints from string env var if provided
	maxDataPointsStr := os.Getenv("MAX_DATA_POINTS")
	if maxDataPointsStr != "" {
		var maxPoints int
		if _, err := fmt.Sscanf(maxDataPointsStr, "%d", &maxPoints); err == nil && maxPoints > 0 {
			cfg.MaxDataPoints = maxPoints
		}
	}

	// Initialize websocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize API key validation middleware
	middleware.InitAPIKeys()

	// Initialize handlers
	handlers.InitializeAPI(cfg, hub)

	// Set up Gin router
	router := gin.Default()

	// Load templates
	router.LoadHTMLGlob("templates/*.html")

	// Serve static files if they exist
	router.Static("/static", "./static")

	// Routes
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Real-Time Analytics Platform",
		})
	})

	router.GET("/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Real-Time Analytics Platform - Dashboard",
		})
	})

	router.GET("/dashboard/:tenant_id", func(c *gin.Context) {
		tenantID := c.Param("tenant_id")
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title":    "Tenant Dashboard",
			"tenantID": tenantID,
		})
	})

	// Health check endpoint
	router.GET("/health", handlers.HealthCheck)

	// API endpoints
	router.GET("/api/status", handlers.SystemStatus)
	router.GET("/api/data", handlers.GetRecentData)
	router.GET("/api/data/:tenant_id", handlers.GetTenantData)
	router.POST("/api/data", middleware.RequireAPIKey(), handlers.ProxyDataIngestion)
	router.GET("/ws", handlers.WebSocket)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "5003"
	}

	// Start server
	log.Printf("[UNIQUE_STARTUP] Visualization-go build: minikube-test-20240522")
	log.Printf("Visualization service listening on port %s", port)
	log.Fatal(router.Run(":" + port))
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
