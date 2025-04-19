package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// --- Global Variables ---
// Note: Most globals are defined in their respective files (config.go, middleware.go, etc.)
// Only constants or truly main-specific globals would go here.
// const defaultTenantKey = "_no_tenant_" // Defined in memory_store.go

// --- Main Function ---
func main() {
	// Set Gin mode
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Load Configurations
	appValidApiKeys := loadApiKeys()
	appEnableTenantIsolation, appTenantApiKeyMap := loadConfig()
	if err := initKafkaProducer(); err != nil {
		log.Printf("Warning: Failed to initialize Kafka producer: %v. Proceeding without Kafka.", err)
	}
	defer closeKafkaProducer()

	// Create Memory Store instance
	memStore := NewMemoryStore()

	// Setup Router & Middleware
	router := gin.Default()
	router.Use(prometheusMiddleware()) // Apply Prometheus middleware globally

	// --- Public Endpoints ---
	registerPublicEndpoints(router, appEnableTenantIsolation, appTenantApiKeyMap, appValidApiKeys, memStore)

	// --- API Group (Authenticated) ---
	registerApiEndpoints(router, appEnableTenantIsolation, appTenantApiKeyMap, appValidApiKeys, memStore)

	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting Gin server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}

// --- Endpoint Registration ---

func registerPublicEndpoints(router *gin.Engine, enableIsolation bool, tenantMap map[string]string, validKeys map[string]bool, memStore *MemoryStore) {
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Metrics endpoint
	router.GET("/metrics", prometheusHandler())

	// Auth test endpoint - wrap handler to pass keys
	router.GET("/auth-test", func(c *gin.Context) {
		handleAuthTest(c, validKeys)
	})

	// Debug endpoint - wrap handler to pass config and store
	router.GET("/debug/tenant-data", func(c *gin.Context) {
		handleDebugTenantData(c, enableIsolation, tenantMap, memStore)
	})
}

func registerApiEndpoints(router *gin.Engine, enableIsolation bool, tenantMap map[string]string, validKeys map[string]bool, memStore *MemoryStore) {
	api := router.Group("/api")
	api.Use(apiKeyAuthMiddleware(validKeys))
	{
		api.POST("/data", func(c *gin.Context) {
			handlePostData(c, enableIsolation, tenantMap, memStore)
		})
		api.GET("/data", func(c *gin.Context) {
			handleGetData(c, enableIsolation, tenantMap, memStore)
		})
	}
}

// --- Handlers ---

func handleAuthTest(c *gin.Context, validKeys map[string]bool) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Authentication failed: Missing API key"})
		return
	}
	if _, ok := validKeys[apiKey]; ok {
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Authentication successful", "key_received": apiKey[:min(3, len(apiKey))] + "***"})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Authentication failed: Invalid API key"})
	}
}

func handleDebugTenantData(c *gin.Context, enableIsolation bool, tenantMap map[string]string, memStore *MemoryStore) {
	allData := memStore.getAllData()
	response := gin.H{
		"tenant_isolation_enabled": enableIsolation,
		"tenant_api_key_map":       tenantMap,
		"tenant_data":              allData,
		"num_tenants_in_memory":    len(allData),
		"total_records_in_memory":  countTotalRecords(allData),
	}
	c.JSON(http.StatusOK, response)
}

func handlePostData(c *gin.Context, enableIsolation bool, tenantMap map[string]string, memStore *MemoryStore) {
	var data SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}
	if _, ok := data["device_id"]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing device_id in data"})
		return
	}

	apiKey := c.GetHeader("X-API-Key")
	if enableIsolation {
		tenantID, found := getTenantIDForKey(apiKey, tenantMap)
		if !found {
			log.Printf("Warning: Unknown tenant for API key starting with %s...", apiKey[:min(3, len(apiKey))])
			c.JSON(http.StatusForbidden, gin.H{"error": "Unknown or unauthorized tenant for this API key"})
			return
		}
		data["tenant_id"] = tenantID
		c.Set("tenantID", tenantID) // Set context for metrics
		log.Printf("Processing data for tenant '%s', device '%v'", tenantID, data["device_id"])
	} else {
		log.Printf("Processing data (tenant isolation disabled) for device '%v'", data["device_id"])
	}

	if err := produceMessage(c.Request.Context(), data); err != nil {
		log.Printf("Error sending message to Kafka: %v", err)
		// Consider returning 500 if Kafka is critical
	}

	memStore.storeData(data)
	c.JSON(http.StatusOK, gin.H{"status": "success", "received": data})
}

func handleGetData(c *gin.Context, enableIsolation bool, tenantMap map[string]string, memStore *MemoryStore) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing device_id query parameter"})
		return
	}

	apiKey := c.GetHeader("X-API-Key")
	requestingTenantID := defaultTenantKey
	if enableIsolation {
		var found bool
		requestingTenantID, found = getTenantIDForKey(apiKey, tenantMap)
		if !found {
			log.Printf("Error: Valid API key %s... has no tenant mapping in GET /data", apiKey[:min(3, len(apiKey))])
			c.JSON(http.StatusForbidden, gin.H{"error": "API key not associated with a tenant"})
			return
		}
		c.Set("tenantID", requestingTenantID) // Set context for metrics
	}

	results := memStore.getDataByDevice(requestingTenantID, deviceID, enableIsolation)

	// Ensure we always return a JSON array, even if empty
	if results == nil {
		results = make([]SensorData, 0) // Create an empty slice if it was nil
	}
	c.JSON(http.StatusOK, results)
}

// --- Helper Functions ---
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func countTotalRecords(allData map[string][]SensorData) int {
	count := 0
	for _, records := range allData {
		count += len(records)
	}
	return count
}

// --- Configuration Loading (config.go) ---
// loadConfig() is called in main(), returns values

// --- API Key Loading (middleware.go) ---
// loadApiKeys() is called in main() // Returns map now

// --- Kafka Producer (kafka_producer.go) ---
// initKafkaProducer() and closeKafkaProducer() are called in main()
// produceMessage() is called in handlePostData() // Uses global

// --- Memory Store (memory_store.go) ---
// Now passed as an instance to relevant functions.

// --- Metrics (metrics.go) ---
// prometheusMiddleware() is used in main()
// prometheusHandler() is used in main()

// --- Auth Middleware (middleware.go) ---
// apiKeyAuthMiddleware() is used in main() // Accepts map now
