package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/processor"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	proc *processor.Processor
)

func main() {
	// Load configuration
	cfg := config.GetConfig()

	// Initialize processor
	var err error
	proc, err = processor.NewProcessor(cfg)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Start processor
	if err := proc.Start(); err != nil {
		log.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Stop()

	// Set up API server
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// API routes
	r.GET("/health", healthCheck)
	r.GET("/api/stats", getStats)
	r.GET("/api/stats/device/:id", getDeviceStats)

	// Prometheus metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start server in a goroutine
	go func() {
		addr := fmt.Sprintf(":%d", cfg.MetricsPort)
		log.Printf("Starting HTTP server on %s", addr)
		if err := http.ListenAndServe(addr, r); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down processing engine...")
}

// healthCheck handles health check requests
func healthCheck(c *gin.Context) {
	_, status := proc.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "processing-engine-go",
		"running":   status.Running,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getStats handles requests for processing statistics
func getStats(c *gin.Context) {
	stats, status := proc.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"data":       stats,
		"processing": status,
	})
}

// getDeviceStats handles requests for device-specific statistics
func getDeviceStats(c *gin.Context) {
	deviceID := c.Param("id")
	stats, _ := proc.GetStats()

	device, exists := stats.Devices[deviceID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("No data for device %s", deviceID),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"device_id": deviceID,
		"data":      device,
	})
}
