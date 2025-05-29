package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Default configuration values
	defaultDBPath        = "/data/analytics.db"
	defaultAPIPort       = 5002
	defaultMetricsPort   = 8001
	defaultRetentionDays = 60
	defaultKafkaBroker   = "kafka:9092"
	defaultKafkaTopic    = "processed-data"

	// Default tenant retention policies
	tenantRetentionPolicies = map[string]RetentionPolicy{
		"tenant1": {Days: 90},
		"tenant2": {Days: 30},
	}

	// Database connection
	db *DB
)

func main() {
	log.Println("[DEBUG] (main) Storage Layer image build: 2025-05-23T00:10Z")
	// Initialize logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting storage layer service...")

	// Load configuration from environment variables
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	apiPortStr := os.Getenv("API_PORT")
	apiPort := defaultAPIPort
	if apiPortStr != "" {
		if p, err := strconv.Atoi(apiPortStr); err == nil {
			apiPort = p
		}
	}

	metricsPortStr := os.Getenv("METRICS_PORT")
	metricsPort := defaultMetricsPort
	if metricsPortStr != "" {
		if p, err := strconv.Atoi(metricsPortStr); err == nil {
			metricsPort = p
		}
	}

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = defaultKafkaBroker
	}

	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		kafkaTopic = defaultKafkaTopic
	}

	// Initialize database
	var err error
	db, err = NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Start Kafka consumer
	log.Printf("[DEBUG] (main) About to start Kafka consumer for topic %s on broker %s", kafkaTopic, kafkaBroker)
	kafkaConsumer, err := NewKafkaConsumer(kafkaBroker, kafkaTopic, db)
	if err != nil {
		log.Fatalf("FATAL: Failed to create Kafka consumer: %v", err)
	}

	if err := kafkaConsumer.Start(); err != nil {
		log.Fatalf("FATAL: Failed to start Kafka consumer: %v", err)
	}
	log.Printf("[DEBUG] (main) Kafka consumer started successfully")

	// Ensure Kafka consumer stops when service shuts down
	defer kafkaConsumer.Stop()

	// Start metrics server
	go func() {
		metricsAddr := fmt.Sprintf(":%d", metricsPort)
		log.Printf("Starting metrics server on %s", metricsAddr)
		http.Handle("/metrics", promhttp.Handler())
		metricsServer := &http.Server{
			Addr:              metricsAddr,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := metricsServer.ListenAndServe(); err != nil {
			log.Printf("Metrics server stopped: %v", err)
		}
	}()

	// Start retention policy goroutine
	go runRetentionPolicy()

	// Create Gin router
	router := gin.Default()

	// API endpoints
	router.GET("/health", healthCheckHandler)
	router.GET("/", indexHandler)
	router.POST("/api/store", storeDataHandler)
	router.GET("/api/data", getDataHandler)
	router.GET("/api/data/:tenant_id", getTenantDataHandler)
	router.GET("/api/data/:tenant_id/:device_id", getTenantDeviceDataHandler)
	router.GET("/api/stats/:tenant_id", getTenantStatsHandler)

	// Start API server in goroutine
	apiAddr := fmt.Sprintf(":%d", apiPort)
	log.Printf("Starting API server on %s", apiAddr)
	srv := &http.Server{
		Addr:              apiAddr,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Shutdown gracefully
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}

// runRetentionPolicy runs the retention policy periodically
func runRetentionPolicy() {
	ticker := time.NewTicker(24 * time.Hour) // Run once per day
	defer ticker.Stop()

	for {
		log.Println("Running retention policy check")
		if err := db.ApplyRetentionPolicies(tenantRetentionPolicies, defaultRetentionDays); err != nil {
			log.Printf("Error applying retention policies: %v", err)
		}

		<-ticker.C
	}
}

// healthCheckHandler handles the health check endpoint
func healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "storage-layer",
		"database":  "connected",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// indexHandler handles the index endpoint
func indexHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "storage-layer",
		"message": "Data Storage Layer",
		"endpoints": map[string]string{
			"/":                               "API information (this endpoint)",
			"/health":                         "Health check",
			"/api/store":                      "Store data (POST)",
			"/api/data":                       "Get recent data (GET)",
			"/api/data/:tenant_id":            "Get data for specific tenant (GET)",
			"/api/data/:tenant_id/:device_id": "Get data for specific device (GET)",
			"/api/stats/:tenant_id":           "Get statistics for specific tenant (GET)",
		},
	})
}

// storeDataHandler handles the data storage endpoint
func storeDataHandler(c *gin.Context) {
	var data SensorDataRequest

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid JSON data: " + err.Error(),
		})
		return
	}

	// Validate required fields
	if data.DeviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing required field: device_id",
		})
		return
	}

	if data.TenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing required field: tenant_id",
		})
		return
	}

	// Set default timestamp if missing
	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	// Store the data
	if err := db.StoreSensorData(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to store data: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data stored successfully",
	})
}

// getDataHandler is a placeholder that redirects to tenant-specific endpoints
func getDataHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "info",
		"message": "Please specify a tenant ID: /api/data/:tenant_id",
	})
}

// getTenantDataHandler handles the tenant data retrieval endpoint
func getTenantDataHandler(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing tenant ID",
		})
		return
	}

	limit := 100
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	data, err := db.GetTenantData(tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve data: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"tenant_id": tenantID,
		"count":     len(data),
		"data":      data,
	})
}

// getTenantDeviceDataHandler handles the tenant device data retrieval endpoint
func getTenantDeviceDataHandler(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	deviceID := c.Param("device_id")

	if tenantID == "" || deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing tenant ID or device ID",
		})
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	data, err := db.GetTenantDeviceData(tenantID, deviceID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve data: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"tenant_id": tenantID,
		"device_id": deviceID,
		"count":     len(data),
		"data":      data,
	})
}

// getTenantStatsHandler handles the tenant statistics endpoint
func getTenantStatsHandler(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing tenant ID",
		})
		return
	}

	stats, err := db.GetTenantStats(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve statistics: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"tenant_id": tenantID,
		"stats":     stats,
	})
}
