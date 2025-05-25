package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// SECURITY NOTE: The API key "test-key-1" used in these tests is for
// TESTING PURPOSES ONLY and should NEVER be used in production.
// Production deployments must use securely generated API keys.

func setupRouter() *gin.Engine {
	// Switch to test mode to avoid extra logging
	gin.SetMode(gin.TestMode)

	// Set up test environment variables for API keys
	os.Setenv("API_KEY_1", "test-key-1")
	os.Setenv("API_KEY_2", "test-key-2")

	// Initialize API keys for testing
	initAPIKeys()

	// Setup router
	r := gin.Default()
	r.POST("/api/data", requireAPIKey(), ingestData)
	r.GET("/health", healthCheck)

	return r
}

func TestHealthEndpoint(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	// Verify response body
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "healthy", response["status"])
}

func TestAPIKeyAuthMiddleware(t *testing.T) {
	router := setupRouter()

	// Test with no API key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/data", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)

	// Test with invalid API key
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/data", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	router.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)

	// Test with valid API key
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(`{"device_id": "test"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// Test with auth bypass
	os.Setenv("BYPASS_AUTH", "true")
	defer os.Setenv("BYPASS_AUTH", "false")

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(`{"device_id": "test"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestIngestData(t *testing.T) {
	router := setupRouter()

	// Test with valid data
	w := httptest.NewRecorder()
	validData := `{"device_id": "test-001", "temperature": 25.5}`
	req, _ := http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(validData)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Data received successfully", response["message"])

	// Test with missing required data
	w = httptest.NewRecorder()
	invalidData := `{}`
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(invalidData)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code) // Now should return 400 since we added validation

	// Verify error message
	var errorResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.Equal(t, "error", errorResponse["status"])
	assert.Equal(t, "device_id is required", errorResponse["message"])

	// Test with empty device_id
	w = httptest.NewRecorder()
	emptyDeviceID := `{"device_id": ""}`
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(emptyDeviceID)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	// Test with invalid JSON
	w = httptest.NewRecorder()
	invalidJSON := `{invalid json}`
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(invalidJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}
