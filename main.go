package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"real-time-analytics-platform/config"
	"real-time-analytics-platform/handlers"
	"real-time-analytics-platform/middleware"
	"real-time-analytics-platform/websocket"
)

func main() {
	// Initialize configuration
	cfg := config.Load()

	// Setup Gin
	if !cfg.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize API
	handlers.InitializeAPI(cfg, hub)

	// Add middlewares
	r.Use(middleware.PrometheusMetrics())

	// Configure paths for static files
	r.Static("/static", "./static")

	// Serve HTML templates
	r.LoadHTMLGlob("templates/*")

	// Define routes
	// Home page
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// WebSocket endpoint
	r.GET("/ws", handlers.WebSocket)

	// API routes
	api := r.Group("/api")
	{
		api.GET("/status", handlers.SystemStatus)
		api.POST("/data", handlers.ProxyDataIngestion)
		api.GET("/data/recent", handlers.GetRecentData)
	}

	// Health check
	r.GET("/health", handlers.HealthCheck)

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start the server
	port := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting visualization service on port %d...", cfg.Port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
