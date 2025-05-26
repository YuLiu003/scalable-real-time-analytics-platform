// Package handlers provides HTTP request handlers for the real-time analytics platform API endpoints.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"real-time-analytics-platform/config"
	"real-time-analytics-platform/metrics"
	"real-time-analytics-platform/models"
	"real-time-analytics-platform/websocket"

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

	// Try to contact the data ingestion service
	start := time.Now()
	client := &http.Client{Timeout: 2 * time.Second}
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/health", dataServiceURL), nil)
	if err != nil {
		metrics.DataServiceErrors.Inc()
		services["data-ingestion"] = models.ServiceStatus{
			Status:  "error",
			Service: "data-ingestion",
			Message: fmt.Sprintf("Error creating request: %s", err.Error()),
		}
	} else {
		resp, err := client.Do(req)
		if err != nil {
			metrics.DataServiceErrors.Inc()
			services["data-ingestion"] = models.ServiceStatus{
				Status:  "error",
				Service: "data-ingestion",
				Message: err.Error(),
			}
		} else {
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			if resp.StatusCode == http.StatusOK {
				var status models.ServiceStatus
				if err := json.NewDecoder(resp.Body).Decode(&status); err == nil {
					services["data-ingestion"] = status
				}
			}
		}
	}
	metrics.DataServiceLatency.Observe(time.Since(start).Seconds())

	c.JSON(http.StatusOK, models.SystemStatus{
		Status:    "ok",
		Services:  services,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ProxyDataIngestion forwards data to the data ingestion service and keeps a local copy
func ProxyDataIngestion(c *gin.Context) {
	var data models.SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, models.DataResponse{
			Status:  "error",
			Message: "Invalid JSON data",
		})
		return
	}

	// Add timestamp if not provided
	if data.Timestamp == nil {
		now := time.Now()
		data.Timestamp = &now
	}

	// Store in memory for visualization
	mu.Lock()
	recentData = append([]models.SensorData{data}, recentData...)
	if len(recentData) > maxDataPoints {
		recentData = recentData[:maxDataPoints]
	}
	mu.Unlock()

	// Emit data to WebSocket clients
	jsonData, err := json.Marshal(data)
	if err == nil {
		hub.BroadcastMessage(jsonData)
	}

	// Forward the request to the data ingestion service if available
	jsonData, _ = json.Marshal(data)
	reader := bytes.NewReader(jsonData)
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	
	var result models.DataResponse
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, 
		fmt.Sprintf("%s/api/data", dataServiceURL), 
		reader)
	if err != nil {
		metrics.DataServiceErrors.Inc()
		result = models.DataResponse{
			Status:  "partial_success",
			Message: fmt.Sprintf("Data stored locally but request creation failed: %s", err.Error()),
			Data:    data,
		}
	} else {
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			metrics.DataServiceErrors.Inc()
			result = models.DataResponse{
				Status:  "partial_success",
				Message: fmt.Sprintf("Data stored locally but not forwarded: %s", err.Error()),
				Data:    data,
			}
		} else {
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			body, _ := io.ReadAll(resp.Body)
			metrics.DataServiceLatency.Observe(time.Since(start).Seconds())

			// Parse the response
			if err := json.Unmarshal(body, &result); err != nil {
				log.Printf("Error parsing data service response: %v", err)
				result = models.DataResponse{
					Status:  "partial_success",
					Message: "Data stored locally but service response invalid",
					Data:    data,
				}
			}
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetRecentData returns recently received data
func GetRecentData(c *gin.Context) {
	mu.Lock()
	data := make([]models.SensorData, len(recentData))
	copy(data, recentData)
	mu.Unlock()

	c.JSON(http.StatusOK, models.RecentDataResponse{
		Status: "ok",
		Data:   data,
	})
}

// WebSocket handles WebSocket connections
func WebSocket(c *gin.Context) {
	websocket.ServeWs(hub, c.Writer, c.Request)
}
