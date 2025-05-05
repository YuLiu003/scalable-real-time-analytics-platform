package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/models"
	processor "github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/processor-sarama"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	proc *processor.Processor
)

// metricsMiddleware records metrics for each HTTP request
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime).Seconds()

		// Record metrics
		endpoint := strings.Split(c.Request.URL.Path, "?")[0]
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		RecordHTTPRequest(endpoint, method, status, duration)
	}
}

func main() {
	// Load configuration
	cfg := config.GetConfig()

	// Initialize processor
	var err error
	proc, err = processor.NewProcessor(cfg)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Start processor with retry logic
	const maxRetries = 3
	var startErr error

	for i := 0; i < maxRetries; i++ {
		startErr = proc.Start()
		if startErr == nil {
			break
		}

		log.Printf("Failed to start processor (attempt %d/%d): %v", i+1, maxRetries, startErr)

		if i < maxRetries-1 {
			retryDelay := time.Duration(2*(i+1)) * time.Second
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if startErr != nil {
		log.Printf("WARNING: Failed to connect to Kafka after %d attempts. "+
			"Will continue running with limited functionality.", maxRetries)
	}

	defer proc.Stop()

	// Set up API server
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// Add metrics middleware
	r.Use(metricsMiddleware())

	// API routes
	r.GET("/health", healthCheck)
	r.GET("/api/stats", getStats)
	r.GET("/api/stats/device/:id", getDeviceStats)

	// Data ingestion endpoint
	r.POST("/api/data", ingestData)

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

// ingestData handles data ingestion requests
func ingestData(c *gin.Context) {
	var sensorData models.SensorData
	if err := c.ShouldBindJSON(&sensorData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Set timestamp if not provided
	if sensorData.Timestamp == nil {
		now := time.Now()
		sensorData.Timestamp = &now
	}

	// Process the data
	result, err := proc.ProcessData(sensorData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Processing error: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   result,
	})
}
