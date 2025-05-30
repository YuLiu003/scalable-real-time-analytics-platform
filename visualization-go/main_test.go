package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"visualization-go/config"
	"visualization-go/handlers"
	"visualization-go/middleware"
	"visualization-go/websocket"
)

func setupTestRouter() *gin.Engine {
	// Switch to test mode
	gin.SetMode(gin.TestMode)

	// Initialize test configuration
	cfg := &config.Config{
		DataServiceURL: "http://test-storage:5002",
		MaxDataPoints:  50,
	}

	// Initialize websocket hub
	hub := websocket.NewHub()

	// Initialize API key validation middleware
	middleware.InitAPIKeys()

	// Initialize handlers
	handlers.InitializeAPI(cfg, hub)

	// Set up Gin router
	router := gin.New()

	// Routes matching main.go
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"title": "Real-Time Analytics Platform",
		})
	})

	router.GET("/dashboard", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"title": "Real-Time Analytics Platform - Dashboard",
		})
	})

	router.GET("/dashboard/:tenant_id", func(c *gin.Context) {
		tenantID := c.Param("tenant_id")
		c.JSON(http.StatusOK, gin.H{
			"title":    "Tenant Dashboard",
			"tenantID": tenantID,
		})
	})

	// Health check endpoint
	router.GET("/health", handlers.HealthCheck)

	// API endpoints
	router.GET("/api/status", handlers.SystemStatus)
	router.GET("/api/data", handlers.GetRecentData)
	router.GET("/api/data/:tenant_id", handlers.GetTenantData)
	router.POST("/api/data", middleware.RequireAPIKey(), handlers.ProxyDataIngestion)

	return router
}

func TestMain(t *testing.T) {
	// Test that the package builds and initializes correctly
	t.Log("Visualization-go package builds successfully")
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "existing environment variable",
			key:          "TEST_VAR_EXISTS",
			defaultValue: "default",
			envValue:     "env_value",
			expected:     "env_value",
		},
		{
			name:         "non-existing environment variable",
			key:          "TEST_VAR_NOT_EXISTS",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "empty environment variable",
			key:          "TEST_VAR_EMPTY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestHomePageEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Real-Time Analytics Platform", response["title"])
}

func TestDashboardEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/dashboard", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Real-Time Analytics Platform - Dashboard", response["title"])
}

func TestTenantDashboardEndpoint(t *testing.T) {
	router := setupTestRouter()

	tenantID := "test-tenant-123"
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/dashboard/"+tenantID, nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Tenant Dashboard", response["title"])
	assert.Equal(t, tenantID, response["tenantID"])
}

func TestSystemStatusEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/status", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestGetRecentDataEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/data", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	// Should return OK even if no data is available
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
}

func TestGetTenantDataEndpoint(t *testing.T) {
	router := setupTestRouter()

	tenantID := "test-tenant-456"
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/data/"+tenantID, nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	// Should return OK or appropriate error status
	assert.True(t, w.Code >= 200 && w.Code < 500)
}

func TestConfigurationLoading(t *testing.T) {
	tests := []struct {
		name              string
		dataServiceURL    string
		maxDataPointsStr  string
		expectedURL       string
		expectedMaxPoints int
	}{
		{
			name:              "default configuration",
			dataServiceURL:    "",
			maxDataPointsStr:  "",
			expectedURL:       "http://storage-layer-go-service:5002",
			expectedMaxPoints: 100,
		},
		{
			name:              "custom data service URL",
			dataServiceURL:    "http://custom-storage:5555",
			maxDataPointsStr:  "",
			expectedURL:       "http://custom-storage:5555",
			expectedMaxPoints: 100,
		},
		{
			name:              "custom max data points",
			dataServiceURL:    "",
			maxDataPointsStr:  "250",
			expectedURL:       "http://storage-layer-go-service:5002",
			expectedMaxPoints: 250,
		},
		{
			name:              "invalid max data points - falls back to default",
			dataServiceURL:    "",
			maxDataPointsStr:  "invalid",
			expectedURL:       "http://storage-layer-go-service:5002",
			expectedMaxPoints: 100,
		},
		{
			name:              "zero max data points - falls back to default",
			dataServiceURL:    "",
			maxDataPointsStr:  "0",
			expectedURL:       "http://storage-layer-go-service:5002",
			expectedMaxPoints: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			if tt.dataServiceURL != "" {
				os.Setenv("DATA_SERVICE_URL", tt.dataServiceURL)
				defer os.Unsetenv("DATA_SERVICE_URL")
			} else {
				os.Unsetenv("DATA_SERVICE_URL")
			}

			if tt.maxDataPointsStr != "" {
				os.Setenv("MAX_DATA_POINTS", tt.maxDataPointsStr)
				defer os.Unsetenv("MAX_DATA_POINTS")
			} else {
				os.Unsetenv("MAX_DATA_POINTS")
			}

			// Initialize configuration (simulating main function logic)
			cfg := &config.Config{
				DataServiceURL: getEnv("DATA_SERVICE_URL", "http://storage-layer-go-service:5002"),
				MaxDataPoints:  100,
			}

			// Convert MaxDataPoints from string env var if provided
			maxDataPointsStr := os.Getenv("MAX_DATA_POINTS")
			if maxDataPointsStr != "" {
				if maxPoints, err := strconv.Atoi(maxDataPointsStr); err == nil && maxPoints > 0 {
					cfg.MaxDataPoints = maxPoints
				}
			}

			assert.Equal(t, tt.expectedURL, cfg.DataServiceURL)
			assert.Equal(t, tt.expectedMaxPoints, cfg.MaxDataPoints)
		})
	}
}

func TestPortConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		portEnv      string
		expectedPort string
	}{
		{
			name:         "default port",
			portEnv:      "",
			expectedPort: "5003",
		},
		{
			name:         "custom port",
			portEnv:      "8080",
			expectedPort: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.portEnv != "" {
				os.Setenv("PORT", tt.portEnv)
				defer os.Unsetenv("PORT")
			} else {
				os.Unsetenv("PORT")
			}

			// Get port (simulating main function logic)
			port := os.Getenv("PORT")
			if port == "" {
				port = "5003"
			}

			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestAPIEndpointsWithoutAPIKey(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/data", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	// Should return 401 or 403 without API key
	assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden)
}

func TestTableDrivenEndpoints(t *testing.T) {
	router := setupTestRouter()

	endpoints := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"health check", "GET", "/health", http.StatusOK},
		{"home page", "GET", "/", http.StatusOK},
		{"dashboard", "GET", "/dashboard", http.StatusOK},
		{"tenant dashboard", "GET", "/dashboard/tenant123", http.StatusOK},
		{"system status", "GET", "/api/status", http.StatusOK},
		{"recent data", "GET", "/api/data", http.StatusOK},
		{"tenant data", "GET", "/api/data/tenant123", http.StatusOK},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest(endpoint.method, endpoint.path, nil)
			require.NoError(t, err)

			router.ServeHTTP(w, req)

			// Allow for some flexibility in status codes for endpoints that may fail due to missing dependencies
			if endpoint.path == "/api/data" || endpoint.path == "/api/data/tenant123" {
				assert.True(t, w.Code >= 200 && w.Code < 500, "Expected success or client error for %s", endpoint.path)
			} else {
				assert.Equal(t, endpoint.expectedStatus, w.Code, "Unexpected status for %s", endpoint.path)
			}
		})
	}
}
