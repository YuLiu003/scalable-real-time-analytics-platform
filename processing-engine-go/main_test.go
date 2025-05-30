package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/models"
	processor "github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/processor-sarama"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *processor.Processor) {
	// Use test mode to disable Gin's logger
	gin.SetMode(gin.TestMode)

	// Create a test configuration
	cfg := &config.Config{
		KafkaEnabled: false, // Disable Kafka for tests
		MaxReadings:  10,    // Use a small value for tests
		Debug:        true,
	}

	// Create a processor for testing
	testProc, _ := processor.NewProcessor(cfg)
	proc = testProc // Set the global proc variable

	// Set up router with our handlers
	r := gin.New()
	r.GET("/health", healthCheck)
	r.GET("/api/stats", getStats)
	r.GET("/api/stats/device/:id", getDeviceStats)

	return r, testProc
}

func TestMain(m *testing.M) {
	// Set up test environment
	gin.SetMode(gin.TestMode)

	// Disable Kafka for testing
	os.Setenv("KAFKA_ENABLED", "false")

	// Initialize processor for testing
	cfg := config.GetConfig()
	proc, _ = processor.NewProcessor(cfg)

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func TestMetricsMiddleware(t *testing.T) {
	// Set up test router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(metricsMiddleware())

	// Add a test endpoint
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Create test request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Process request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthCheck(t *testing.T) {
	router, _ := setupTestRouter()

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.Nil(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "processing-engine-go", response["service"])
}

func TestGetStats(t *testing.T) {
	router, p := setupTestRouter()

	// Add some test data to the processor
	now := time.Now()
	temp := 25.5
	humidity := 60.0
	testData := models.SensorData{
		DeviceID:    "test-device",
		Temperature: &temp,
		Humidity:    &humidity,
		Timestamp:   &now,
		TenantID:    "tenant1",
	}

	// Process the test data
	result, err := p.ProcessData(testData)
	assert.Nil(t, err)
	assert.NotNil(t, result)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats", nil)
	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)

	assert.Nil(t, err)
	assert.Equal(t, "ok", response["status"])

	// Check that data contains temperature and humidity stats
	data := response["data"].(map[string]interface{})
	assert.NotNil(t, data["temperature"])
	assert.NotNil(t, data["humidity"])

	// Check that there's a device with our test device ID
	devices := data["devices"].(map[string]interface{})
	assert.NotNil(t, devices["test-device"])
}

func TestGetDeviceStats(t *testing.T) {
	router, p := setupTestRouter()

	// Add some test data to the processor
	now := time.Now()
	temp := 26.5
	humidity := 65.0
	testData := models.SensorData{
		DeviceID:    "test-device-2",
		Temperature: &temp,
		Humidity:    &humidity,
		Timestamp:   &now,
		TenantID:    "tenant1",
	}

	// Process the test data
	result, err := p.ProcessData(testData)
	assert.Nil(t, err)
	assert.NotNil(t, result)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats/device/test-device-2", nil)
	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)

	assert.Nil(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "test-device-2", response["device_id"])

	// Check device data
	deviceData := response["data"].(map[string]interface{})
	assert.NotNil(t, deviceData["temperature"])
	assert.NotNil(t, deviceData["humidity"])
}

func TestDeviceNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	// Create a test request for a non-existent device
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/stats/device/non-existent", nil)
	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, 404, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.Nil(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "non-existent")
}

func TestIngestData(t *testing.T) {
	// Set up test router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/data", ingestData)
	t.Run("ValidData", func(t *testing.T) {
		// Create test data with correct pointer syntax
		temp := 25.5
		humidity := 60.0
		timestamp := time.Now()

		data := models.SensorData{
			DeviceID:    "test-device",
			Temperature: &temp,
			Humidity:    &humidity,
			Timestamp:   &timestamp,
			TenantID:    "test-tenant",
		}

		jsonData, _ := json.Marshal(data)

		// Create test request
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/data", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		// Process request
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "success", response["status"])
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		// Create test request with invalid JSON
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte("{invalid json}")))
		req.Header.Set("Content-Type", "application/json")

		// Process request
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("EmptyBody", func(t *testing.T) {
		// Create test request with empty body
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte("")))
		req.Header.Set("Content-Type", "application/json")

		// Process request
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestRecordHTTPRequest(t *testing.T) {
	// Test that the function doesn't panic
	assert.NotPanics(t, func() {
		RecordHTTPRequest("/test", "GET", "200", 0.1)
	})
}

func TestAPIEndpoints(t *testing.T) {
	// Test multiple endpoints in sequence
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(metricsMiddleware())
	router.GET("/health", healthCheck)
	router.GET("/api/stats", getStats)
	router.GET("/api/stats/device/:id", getDeviceStats)
	router.POST("/api/data", ingestData)

	endpoints := []struct {
		method     string
		path       string
		statusCode int
	}{
		{"GET", "/health", http.StatusOK},
		{"GET", "/api/stats", http.StatusOK},
		{"GET", "/api/stats/device/test-device", http.StatusOK},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.method+"_"+endpoint.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, endpoint.statusCode, w.Code)
		})
	}
}
