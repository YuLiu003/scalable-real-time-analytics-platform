package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRouter() *gin.Engine {
	// Switch to test mode to avoid extra logging
	gin.SetMode(gin.TestMode)

	// Setup router
	r := gin.Default()
	r.GET("/health", healthCheck)
	r.POST("/api/data", ingestData)

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

func TestIngestData(t *testing.T) {
	router := setupRouter()

	// Test with valid data
	w := httptest.NewRecorder()
	validData := `{"device_id": "test-001", "temperature": 25.5}`
	req, _ := http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(validData)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Data logged successfully", response["message"])

	// Test with empty JSON
	w = httptest.NewRecorder()
	emptyData := `{}`
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(emptyData)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	// Test with invalid JSON
	w = httptest.NewRecorder()
	invalidJSON := `{invalid json}`
	req, _ = http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(invalidJSON)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}
