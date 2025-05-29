package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ServiceInfo represents basic status for a service
type ServiceInfo struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Endpoint string `json:"endpoint"`
}

// SystemStatusResponse represents the overall system status
type SystemStatusResponse struct {
	Status    string                 `json:"status"`
	Services  map[string]ServiceInfo `json:"services"`
	Timestamp string                 `json:"timestamp"`
	Resources map[string]int         `json:"resources"`
}

// GetSystemStatus returns the current system status
func GetSystemStatus(c *gin.Context) {
	// This is where you'd check actual services
	// We'll create a mock response for now

	// Check if services are up
	apiStatus := checkServiceHealth("http://gin-api-service/health")
	dataStatus := checkServiceHealth("http://data-ingestion-service/health")
	processingStatus := checkServiceHealth("http://processing-engine-service/health")
	storageStatus := checkServiceHealth("http://storage-layer-service/health")
	kafkaStatus := checkKafkaHealth("http://kafka:9092")

	// Build response
	response := SystemStatusResponse{
		Status: "operational",
		Services: map[string]ServiceInfo{
			"api": {
				Name:     "API Gateway",
				Status:   apiStatus,
				Endpoint: "http://gin-api-service",
			},
			"data-ingestion": {
				Name:     "Data Ingestion",
				Status:   dataStatus,
				Endpoint: "http://data-ingestion-service",
			},
			"processing-engine": {
				Name:     "Processing Engine",
				Status:   processingStatus,
				Endpoint: "http://processing-engine-service",
			},
			"storage-layer": {
				Name:     "Storage Layer",
				Status:   storageStatus,
				Endpoint: "http://storage-layer-service",
			},
			"kafka": {
				Name:     "Kafka",
				Status:   kafkaStatus,
				Endpoint: "http://kafka:9092",
			},
			"tenant-management": {
				Name:     "Tenant Management",
				Status:   "operational", // We assume the tenant management service is up
				Endpoint: tenantServiceURL,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Resources: map[string]int{
			"active_tenants": countActiveTenants(),
			"total_messages": 0, // Mock value
			"storage_used":   0, // Mock value
		},
	}

	// If any critical service is down, mark overall status as degraded
	for _, service := range response.Services {
		if service.Status == "error" {
			response.Status = "degraded"
			break
		}
	}

	c.JSON(http.StatusOK, response)
}

// checkServiceHealth pings a service health endpoint
func checkServiceHealth(url string) string {
	// In a real app, we'd actually make a request to the url
	log.Printf("Would check health for service at: %s", url)
	// For demo purposes, let's return a mocked response
	return "operational"
}

// checkKafkaHealth checks Kafka connectivity
func checkKafkaHealth(brokerURL string) string {
	// In a real app, we'd try to connect to Kafka at the brokerURL
	log.Printf("Would check Kafka connectivity at: %s", brokerURL)
	// For demo purposes, let's return a mocked response
	return "operational"
}

// countActiveTenants retrieves the count of active tenants
func countActiveTenants() int {
	// In a real app, we'd fetch this from the tenant service
	// For now, we'll return a mock value
	url := fmt.Sprintf("%s/api/tenants", tenantServiceURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make a request to the tenant service
	response, err := client.Get(url)
	if err != nil {
		// If the request fails, return a default value
		return 0
	}
	defer response.Body.Close()

	// Parse the response
	var result map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0
	}

	// Extract the tenant count
	if tenants, ok := result["tenants"].([]interface{}); ok {
		return len(tenants)
	}

	return 0
}
