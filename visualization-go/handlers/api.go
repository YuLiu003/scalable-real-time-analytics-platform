package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"visualization-go/config"
	"visualization-go/metrics"
	"visualization-go/models"
	"visualization-go/websocket"

	"github.com/gin-gonic/gin"
)

// Store for recent data
var (
	recentData     []models.SensorData
	maxDataPoints  int
	dataServiceURL string
	hub            *websocket.Hub
	mu             sync.Mutex
)

// InitializeAPI sets up API routes and dependencies
func InitializeAPI(cfg *config.Config, wsHub *websocket.Hub) {
	maxDataPoints = cfg.MaxDataPoints
	dataServiceURL = cfg.DataServiceURL
	hub = wsHub
}

// SystemStatus returns the status of system services
func SystemStatus(c *gin.Context) {
	// Set up initial response
	services := map[string]models.ServiceStatus{
		"visualization": {
			Status:    "ok",
			Service:   "visualization",
			Timestamp: time.Now().Format(time.RFC3339),
		},
		"data-ingestion": {
			Status:  "unknown",
			Service: "data-ingestion",
		},
		"processing-engine": {
			Status:  "unknown",
			Service: "processing-engine",
		},
		"storage-layer": {
			Status:  "unknown",
			Service: "storage-layer",
		},
	}

	// Create a client with a reasonable timeout
	client := &http.Client{Timeout: 2 * time.Second}

	// Check data ingestion service
	checkService(client, "data-ingestion", "http://data-ingestion-go-service/health", services)

	// Check processing engine service
	checkService(client, "processing-engine", "http://processing-engine-go:8000/health", services)

	// Check storage layer service
	checkService(client, "storage-layer", dataServiceURL+"/health", services)

	// Create the final response
	response := models.SystemStatus{
		Status:    "ok",
		Services:  services,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Validate the response will be valid JSON
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("ERROR: Failed to marshal system status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate system status",
		})
		return
	}

	// Double-check that the JSON is valid
	if !json.Valid(jsonBytes) {
		log.Printf("ERROR: Generated invalid JSON for system status: %s", string(jsonBytes))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Generated invalid JSON for system status",
		})
		return
	}

	// Log the response for debugging
	log.Printf("System status response: %s", string(jsonBytes))

	// Return the validated response
	c.Data(http.StatusOK, "application/json", jsonBytes)
}

// Helper function to check service health
func checkService(client *http.Client, serviceName, healthEndpoint string, services map[string]models.ServiceStatus) {
	start := time.Now()
	resp, err := client.Get(healthEndpoint)
	if err != nil {
		metrics.DataServiceErrors.Inc()
		services[serviceName] = models.ServiceStatus{
			Status:    "error",
			Service:   serviceName,
			Message:   err.Error(),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		return
	}

	defer resp.Body.Close()
	metrics.DataServiceLatency.Observe(time.Since(start).Seconds())

	if resp.StatusCode == http.StatusOK {
		// Read the entire body first
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			services[serviceName] = models.ServiceStatus{
				Status:    "error",
				Service:   serviceName,
				Message:   fmt.Sprintf("Error reading response: %s", readErr.Error()),
				Timestamp: time.Now().Format(time.RFC3339),
			}
			return
		}

		// Check if the body is empty or not JSON
		if len(bodyBytes) == 0 {
			services[serviceName] = models.ServiceStatus{
				Status:    "healthy",
				Service:   serviceName,
				Message:   "Service is running but returned empty response",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			return
		}

		// Try to validate if it's valid JSON before attempting to decode
		if !json.Valid(bodyBytes) {
			services[serviceName] = models.ServiceStatus{
				Status:    "healthy",
				Service:   serviceName,
				Message:   fmt.Sprintf("Service is running but returned invalid JSON: %s", string(bodyBytes)),
				Timestamp: time.Now().Format(time.RFC3339),
			}
			return
		}

		// Now try to decode the valid JSON
		var status models.ServiceStatus
		if err := json.Unmarshal(bodyBytes, &status); err == nil {
			services[serviceName] = status
		} else {
			// JSON is valid but doesn't match our expected structure
			services[serviceName] = models.ServiceStatus{
				Status:    "healthy",
				Service:   serviceName,
				Message:   fmt.Sprintf("Service is running but returned unexpected JSON format: %s", err.Error()),
				Timestamp: time.Now().Format(time.RFC3339),
			}
		}
	} else {
		// Service responded but with an error status
		bodyBytes, _ := io.ReadAll(resp.Body)
		services[serviceName] = models.ServiceStatus{
			Status:    "error",
			Service:   serviceName,
			Message:   fmt.Sprintf("Service returned status %d: %s", resp.StatusCode, string(bodyBytes)),
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}
}

// ProxyDataIngestion forwards data to the data ingestion service and keeps a local copy
func ProxyDataIngestion(c *gin.Context) {
	var data models.SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, models.DataResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid JSON data: %s", err.Error()),
		})
		return
	}

	// Validate API key
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, models.DataResponse{
			Status:  "error",
			Message: "Missing API key",
		})
		return
	}

	// Validate required fields
	if data.Temperature == nil || data.Humidity == nil {
		c.JSON(http.StatusBadRequest, models.DataResponse{
			Status:  "error",
			Message: "Missing required fields: temperature and humidity",
		})
		return
	}

	// Validate tenant ID
	if data.TenantID == "" {
		c.JSON(http.StatusBadRequest, models.DataResponse{
			Status:  "error",
			Message: "Missing tenant ID",
		})
		return
	}

	// Add timestamp if not provided
	if data.Timestamp == nil {
		now := time.Now()
		data.Timestamp = &now
	}

	// Add unique debug log to confirm code is being built and deployed
	log.Printf("[UNIQUE_DEBUG] ProxyDataIngestion called and forwarding to /api/store")
	// Forward the request to the data ingestion service first
	jsonData, _ := json.Marshal(data)
	reader := bytes.NewReader(jsonData)
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}

	// Create request to forward with headers
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/store", dataServiceURL), reader)
	if err != nil {
		metrics.DataServiceErrors.Inc()
		c.JSON(http.StatusInternalServerError, models.DataResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to create request: %s", err.Error()),
		})
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		metrics.DataServiceErrors.Inc()
		c.JSON(http.StatusInternalServerError, models.DataResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to forward data: %s", err.Error()),
		})
		return
	}
	defer resp.Body.Close()

	// Read and validate response
	body, err := io.ReadAll(resp.Body)
	metrics.DataServiceLatency.Observe(time.Since(start).Seconds())
	log.Printf("[DEBUG] Storage service response status: %d, body: %s", resp.StatusCode, string(body))
	if err != nil {
		metrics.DataServiceErrors.Inc()
		c.JSON(http.StatusInternalServerError, models.DataResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to read response: %s", err.Error()),
		})
		return
	}

	// Only store data locally if the data service accepted it and response is valid JSON
	if resp.StatusCode == http.StatusOK && json.Valid(body) {
		// Store in memory for visualization
		mu.Lock()
		recentData = append([]models.SensorData{data}, recentData...)
		if len(recentData) > maxDataPoints {
			recentData = recentData[:maxDataPoints]
		}
		mu.Unlock()

		// Emit data to WebSocket clients
		if jsonData, err := json.Marshal(data); err == nil {
			hub.BroadcastToTenant(jsonData, data.TenantID)
		}
	}

	// Return the data service response as-is
	c.Data(resp.StatusCode, "application/json", body)
}

// GetRecentData returns recently received data
func GetRecentData(c *gin.Context) {
	mu.Lock()
	data := make([]models.SensorData, len(recentData))
	copy(data, recentData)
	mu.Unlock()

	// Create the response
	response := models.RecentDataResponse{
		Status: "ok",
		Data:   data,
	}

	// Marshal to JSON with validation
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("ERROR: Failed to marshal data response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate data response",
		})
		return
	}

	// Double-check that the JSON is valid
	if !json.Valid(jsonBytes) {
		log.Printf("ERROR: Generated invalid JSON for data response: %s", string(jsonBytes))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Generated invalid JSON for data response",
		})
		return
	}

	// Log the first part of the response for debugging
	if len(jsonBytes) > 200 {
		log.Printf("Data response (truncated): %s...", string(jsonBytes[:200]))
	} else {
		log.Printf("Data response: %s", string(jsonBytes))
	}

	// Return the validated response
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", jsonBytes)
}

// GetTenantData returns data for a specific tenant
func GetTenantData(c *gin.Context) {
	// Extract tenant ID from the URL parameter
	tenantID := c.Param("tenant_id")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing tenant ID",
		})
		return
	}

	// Filter data by tenant ID
	mu.Lock()
	var filteredData []models.SensorData
	for _, d := range recentData {
		if d.TenantID == tenantID {
			filteredData = append(filteredData, d)
		}
	}
	mu.Unlock()

	// If no data is available, generate sample data
	if len(filteredData) == 0 {
		log.Printf("No data available for tenant %s, generating sample data", tenantID)
		now := time.Now()
		timestamp := now.Add(-10 * time.Minute)
		temp := 24.5
		humidity := 50.0
		sampleData := models.SensorData{
			DeviceID:    "device-001",
			Temperature: &temp,
			Humidity:    &humidity,
			Timestamp:   &timestamp,
			TenantID:    tenantID,
		}
		filteredData = append(filteredData, sampleData)

		// Store sample data in memory
		mu.Lock()
		recentData = append([]models.SensorData{sampleData}, recentData...)
		if len(recentData) > maxDataPoints {
			recentData = recentData[:maxDataPoints]
		}
		mu.Unlock()
	}

	response := models.RecentDataResponse{
		Status: "ok",
		Data:   filteredData,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("ERROR: Failed to marshal tenant data response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate tenant data response",
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", jsonBytes)
}

// WebSocket handles WebSocket connections
func WebSocket(c *gin.Context) {
	websocket.ServeWs(hub, c.Writer, c.Request)
}
