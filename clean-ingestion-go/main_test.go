package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	assert.Equal(t, "Data cleaned and logged successfully", response["message"])

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

// TestValidateRequiredFields tests the validateRequiredFields function
func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid data with device_id",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"temperature": 25.5,
			},
			wantErr: false,
		},
		{
			name: "valid data with sensor_id",
			data: map[string]interface{}{
				"sensor_id": "sensor-001",
				"humidity":  60.0,
			},
			wantErr: false,
		},
		{
			name: "valid data with both device_id and sensor_id",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"sensor_id":   "sensor-001",
				"temperature": 25.5,
			},
			wantErr: false,
		},
		{
			name: "missing both device_id and sensor_id",
			data: map[string]interface{}{
				"temperature": 25.5,
			},
			wantErr: true,
		},
		{
			name: "empty device_id",
			data: map[string]interface{}{
				"device_id":   "",
				"temperature": 25.5,
			},
			wantErr: true,
		},
		{
			name: "non-string device_id",
			data: map[string]interface{}{
				"device_id":   123,
				"temperature": 25.5,
			},
			wantErr: true,
		},
		{
			name: "valid data with RFC3339 timestamp",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"timestamp":   "2023-05-29T10:30:00Z",
				"temperature": 25.5,
			},
			wantErr: false,
		},
		{
			name: "valid data with Unix timestamp",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"timestamp":   1685357400.0,
				"temperature": 25.5,
			},
			wantErr: false,
		},
		{
			name: "invalid timestamp format",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"timestamp":   "invalid-timestamp",
				"temperature": 25.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequiredFields(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCleanData tests the cleanData function
func TestCleanData(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		validate func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "basic sensor data cleaning",
			input: map[string]interface{}{
				"device_id":   "test-001",
				"temperature": 25.5,
				"humidity":    60.0,
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "test-001", result["device_id"])
				assert.Equal(t, 25.5, result["temperature"])
				assert.Equal(t, 60.0, result["humidity"])
				assert.Contains(t, result, "processed_at")
				assert.Contains(t, result, "quality_score")

				// Check that processed_at is a valid RFC3339 timestamp
				processedAt, ok := result["processed_at"].(string)
				assert.True(t, ok)
				_, err := time.Parse(time.RFC3339, processedAt)
				assert.NoError(t, err)

				// Check quality score is a float64
				qualityScore, ok := result["quality_score"].(float64)
				assert.True(t, ok)
				assert.GreaterOrEqual(t, qualityScore, 0.0)
				assert.LessOrEqual(t, qualityScore, 1.0)
			},
		},
		{
			name: "timestamp normalization",
			input: map[string]interface{}{
				"device_id":   "test-001",
				"timestamp":   1685357400.0, // Unix timestamp
				"temperature": 25.5,
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				timestamp, ok := result["timestamp"].(string)
				assert.True(t, ok)
				_, err := time.Parse(time.RFC3339, timestamp)
				assert.NoError(t, err)
			},
		},
		{
			name: "humidity clamping",
			input: map[string]interface{}{
				"device_id": "test-001",
				"humidity":  150.0, // Out of range
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				humidity, ok := result["humidity"].(float64)
				assert.True(t, ok)
				assert.Equal(t, 100.0, humidity) // Should be clamped to 100
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanData(tt.input)
			tt.validate(t, result)
		})
	}
}

// TestNormalizeTimestamp tests the normalizeTimestamp function
func TestNormalizeTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expectRFC bool
	}{
		{
			name:      "RFC3339 string",
			input:     "2023-05-29T10:30:00Z",
			expectRFC: true,
		},
		{
			name:      "Unix timestamp float64",
			input:     1685357400.0,
			expectRFC: true,
		},
		{
			name:      "Unix timestamp int",
			input:     1685357400,
			expectRFC: true,
		},
		{
			name:      "invalid input",
			input:     "invalid",
			expectRFC: true, // Should return current time in RFC3339
		},
		{
			name:      "nil input",
			input:     nil,
			expectRFC: true, // Should return current time in RFC3339
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTimestamp(tt.input)
			if tt.expectRFC {
				_, err := time.Parse(time.RFC3339, result)
				assert.NoError(t, err, "Result should be valid RFC3339: %s", result)
			}
		})
	}
}

// TestValidateSensorValue tests the validateSensorValue function
func TestValidateSensorValue(t *testing.T) {
	tests := []struct {
		name       string
		sensorType string
		value      float64
		expected   float64
	}{
		{
			name:       "normal temperature",
			sensorType: "temperature",
			value:      25.5,
			expected:   25.5,
		},
		{
			name:       "extreme temperature",
			sensorType: "temperature",
			value:      -60.0,
			expected:   -60.0, // Should pass through but log warning
		},
		{
			name:       "normal humidity",
			sensorType: "humidity",
			value:      60.0,
			expected:   60.0,
		},
		{
			name:       "humidity over 100",
			sensorType: "humidity",
			value:      150.0,
			expected:   100.0, // Should be clamped
		},
		{
			name:       "negative humidity",
			sensorType: "humidity",
			value:      -10.0,
			expected:   0.0, // Should be clamped
		},
		{
			name:       "unknown sensor type",
			sensorType: "pressure",
			value:      1013.25,
			expected:   1013.25, // Should pass through unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateSensorValue(tt.sensorType, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateQualityScore tests the calculateQualityScore function
func TestCalculateQualityScore(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected float64
	}{
		{
			name: "complete data",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"sensor_id":   "sensor-001",
				"timestamp":   "2023-05-29T10:30:00Z",
				"temperature": 25.5,
				"humidity":    60.0,
			},
			expected: 1.0, // Full score
		},
		{
			name: "missing sensor_id",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"timestamp":   "2023-05-29T10:30:00Z",
				"temperature": 25.5,
			},
			expected: 0.9, // -0.1 for missing sensor_id
		},
		{
			name: "missing timestamp",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"temperature": 25.5,
			},
			expected: 0.8, // -0.1 for missing sensor_id, -0.1 for missing timestamp
		},
		{
			name: "extreme temperature",
			data: map[string]interface{}{
				"device_id":   "test-001",
				"temperature": -60.0, // Extreme value
			},
			expected: 0.6, // -0.1 for missing sensor_id, -0.1 for missing timestamp, -0.2 for extreme temperature
		},
		{
			name: "minimal data",
			data: map[string]interface{}{
				"device_id": "test-001",
			},
			expected: 0.8, // -0.1 for missing sensor_id, -0.1 for missing timestamp
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateQualityScore(tt.data)
			assert.InDelta(t, tt.expected, result, 0.001, "Quality score should match expected value")
		})
	}
}

// TestIngestDataWithValidation tests the complete flow including validation
func TestIngestDataWithValidation(t *testing.T) {
	router := setupRouter()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "valid data with device_id",
			requestBody:    `{"device_id": "test-001", "temperature": 25.5}`,
			expectedStatus: 200,
			expectedMsg:    "Data cleaned and logged successfully",
		},
		{
			name:           "valid data with sensor_id",
			requestBody:    `{"sensor_id": "sensor-001", "humidity": 60.0}`,
			expectedStatus: 200,
			expectedMsg:    "Data cleaned and logged successfully",
		},
		{
			name:           "missing required fields",
			requestBody:    `{"temperature": 25.5}`,
			expectedStatus: 400,
			expectedMsg:    "",
		},
		{
			name:           "empty device_id",
			requestBody:    `{"device_id": "", "temperature": 25.5}`,
			expectedStatus: 400,
			expectedMsg:    "",
		},
		{
			name:           "invalid timestamp",
			requestBody:    `{"device_id": "test-001", "timestamp": "invalid", "temperature": 25.5}`,
			expectedStatus: 400,
			expectedMsg:    "",
		},
		{
			name:           "valid data with extreme humidity",
			requestBody:    `{"device_id": "test-001", "humidity": 150.0}`,
			expectedStatus: 200,
			expectedMsg:    "Data cleaned and logged successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/data", bytes.NewBuffer([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == 200 {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, tt.expectedMsg, response["message"])

				// Verify that the response contains cleaned data
				cleanedData, ok := response["data"].(map[string]interface{})
				assert.True(t, ok)
				assert.Contains(t, cleanedData, "processed_at")
				assert.Contains(t, cleanedData, "quality_score")
			}
		})
	}
}
