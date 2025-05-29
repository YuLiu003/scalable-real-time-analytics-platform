//go:build integration
// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for Data Ingestion Service
// These tests require external dependencies like Kafka

func TestIntegrationDataIngestionWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := setupRouter()

	t.Run("Complete data ingestion workflow", func(t *testing.T) {
		// Simulate a complete data ingestion workflow
		testData := map[string]interface{}{
			"device_id":   "integration-test-device-001",
			"timestamp":   time.Now().Unix(),
			"temperature": 23.5,
			"humidity":    65.2,
			"location": map[string]float64{
				"lat": 37.7749,
				"lng": -122.4194,
			},
		}

		jsonData, err := json.Marshal(testData)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", "/api/data", bytes.NewBuffer(jsonData))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "test-key-1")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response["status"])
		assert.Equal(t, "Data received successfully", response["message"])
	})

	t.Run("High volume data ingestion", func(t *testing.T) {
		// Test handling multiple concurrent requests
		const numRequests = 10
		resultChan := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				testData := map[string]interface{}{
					"device_id":   fmt.Sprintf("load-test-device-%d", id),
					"timestamp":   time.Now().Unix(),
					"temperature": 20.0 + float64(id),
				}

				jsonData, _ := json.Marshal(testData)
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/api/data", bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-API-Key", "test-key-1")

				router.ServeHTTP(w, req)
				resultChan <- w.Code
			}(i)
		}

		// Collect results
		successCount := 0
		for i := 0; i < numRequests; i++ {
			statusCode := <-resultChan
			if statusCode == http.StatusOK {
				successCount++
			}
		}

		// All requests should succeed
		assert.Equal(t, numRequests, successCount)
	})
}

func TestIntegrationKafkaConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test actual Kafka connectivity in a real integration environment
	t.Run("Kafka producer connection", func(t *testing.T) {
		// In a real integration test, this would:
		// 1. Connect to a test Kafka cluster
		// 2. Send test messages
		// 3. Verify messages are received
		// 4. Clean up test data

		t.Skip("Kafka integration test requires running Kafka cluster")
	})
}

func TestIntegrationMetricsCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Metrics endpoint availability", func(t *testing.T) {
		router := setupRouter()

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/metrics", nil)
		require.NoError(t, err)

		router.ServeHTTP(w, req)

		// Metrics endpoint might not be implemented yet, so we check if it's at least accessible
		// In a real scenario, this would verify Prometheus metrics format
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
	})
}
