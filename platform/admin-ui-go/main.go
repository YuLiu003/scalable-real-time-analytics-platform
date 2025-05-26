package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"admin-ui-go/handlers"
	"admin-ui-go/middleware"
)

func main() {
	// Set up Gin
	r := gin.Default()

	// Add middlewares
	r.Use(middleware.Logger())
	r.Use(middleware.PrometheusMetrics())

	// Configure paths for static files
	r.Static("/static", "./static")

	// Serve HTML templates
	r.LoadHTMLGlob("templates/*")

	// Main route for admin UI
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "Analytics Platform Admin",
		})
	})

	// API routes that proxy to Python tenant management service
	api := r.Group("/api")
	{
		// Tenant management routes
		tenants := api.Group("/tenants")
		{
			tenants.GET("", handlers.ListTenants)
			tenants.POST("", handlers.CreateTenant)
			tenants.GET("/:id", handlers.GetTenant)
			tenants.DELETE("/:id", handlers.DeleteTenant)
		}

		// System status
		api.GET("/status", handlers.GetSystemStatus)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "admin-ui",
		})
	})

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "5050"
	}

	// Start the server
	log.Printf("Starting Admin UI service on port %s...", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
