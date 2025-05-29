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

// Integration tests for Clean Ingestion Service
// These tests verify data cleaning and validation workflows

func TestIntegrationDataCleaningWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := setupRouter()

	t.Run("Complete data cleaning workflow", func(t *testing.T) {
		// Test data with various quality issues that should be cleaned
		testCases := []struct {
			name         string
			inputData    map[string]interface{}
			expectStatus int
			expectClean  bool
		}{
			{
				name: "Valid clean data",
				inputData: map[string]interface{}{
					"device_id":   "clean-test-001",
					"timestamp":   time.Now().Unix(),
					"temperature": 23.5,
					"humidity":    65.2,
				},
				expectStatus: http.StatusOK,
				expectClean:  true,
			},
			{
				name: "Data with outliers",
				inputData: map[string]interface{}{
					"device_id":   "outlier-test-001",
					"timestamp":   time.Now().Unix(),
					"temperature": 150.0, // Unrealistic temperature
					"humidity":    120.0, // Invalid humidity
				},
				expectStatus: http.StatusOK,
				expectClean:  false, // Should be flagged for cleaning
			},
			{
				name: "Data with missing fields",
				inputData: map[string]interface{}{
					"device_id": "incomplete-test-001",
					// Missing timestamp and sensor data
				},
				expectStatus: http.StatusBadRequest,
				expectClean:  false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				jsonData, err := json.Marshal(tc.inputData)
				require.NoError(t, err)

				w := httptest.NewRecorder()
				req, err := http.NewRequest("POST", "/api/data", bytes.NewBuffer(jsonData))
				require.NoError(t, err)

				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)

				assert.Equal(t, tc.expectStatus, w.Code)

				if tc.expectStatus == http.StatusOK {
					var response map[string]interface{}
					err = json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)

					assert.Equal(t, "success", response["status"])
				}
			})
		}
	})
}

func TestIntegrationDataValidationRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := setupRouter()

	t.Run("Temperature validation rules", func(t *testing.T) {
		// Test temperature range validation
		temperatures := []float64{-50.0, -10.0, 0.0, 25.0, 50.0, 100.0}

		for _, temp := range temperatures {
			testData := map[string]interface{}{
				"device_id":   fmt.Sprintf("temp-test-%v", temp),
				"timestamp":   time.Now().Unix(),
				"temperature": temp,
			}

			jsonData, err := json.Marshal(testData)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/api/data", bytes.NewBuffer(jsonData))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			// All reasonable temperatures should be accepted
			if temp >= -40.0 && temp <= 80.0 {
				assert.Equal(t, http.StatusOK, w.Code, "Temperature %v should be valid", temp)
			}
		}
	})
}

func TestIntegrationConcurrentDataCleaning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := setupRouter()

	t.Run("Concurrent data cleaning requests", func(t *testing.T) {
		const numWorkers = 5
		const requestsPerWorker = 10

		resultChan := make(chan bool, numWorkers*requestsPerWorker)

		// Start multiple workers sending cleaning requests
		for worker := 0; worker < numWorkers; worker++ {
			go func(workerID int) {
				for req := 0; req < requestsPerWorker; req++ {
					testData := map[string]interface{}{
						"device_id":   fmt.Sprintf("concurrent-worker-%d-req-%d", workerID, req),
						"timestamp":   time.Now().Unix(),
						"temperature": 20.0 + float64(req),
						"humidity":    50.0 + float64(workerID),
					}

					jsonData, _ := json.Marshal(testData)
					w := httptest.NewRecorder()
					httpReq, _ := http.NewRequest("POST", "/api/data", bytes.NewBuffer(jsonData))
					httpReq.Header.Set("Content-Type", "application/json")

					router.ServeHTTP(w, httpReq)
					resultChan <- w.Code == http.StatusOK
				}
			}(worker)
		}

		// Collect results
		successCount := 0
		totalRequests := numWorkers * requestsPerWorker

		for i := 0; i < totalRequests; i++ {
			if <-resultChan {
				successCount++
			}
		}

		// At least 90% of requests should succeed under concurrent load
		successRate := float64(successCount) / float64(totalRequests)
		assert.Greater(t, successRate, 0.9, "Success rate should be > 90%")
	})
}

func TestIntegrationHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := setupRouter()

	t.Run("Health check under load", func(t *testing.T) {
		// Perform multiple health checks to ensure stability
		for i := 0; i < 10; i++ {
			w := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/health", nil)
			require.NoError(t, err)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "healthy", response["status"])
		}
	})
}
