package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"visualization-go/config"
	"visualization-go/websocket"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInitializeAPI(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Port:           5003,
		DataServiceURL: "http://test-service",
		MaxDataPoints:  100,
		DebugMode:      false,
	}

	hub := websocket.NewHub()

	// Test that InitializeAPI doesn't panic
	assert.NotPanics(t, func() {
		InitializeAPI(cfg, hub)
	})
}

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		DataServiceURL: "http://test-service",
		MaxDataPoints:  100,
	}
	hub := websocket.NewHub()

	InitializeAPI(cfg, hub)

	router := gin.New()
	router.GET("/health", HealthCheck)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestGetRecentDataHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		DataServiceURL: "http://test-service",
		MaxDataPoints:  100,
	}
	hub := websocket.NewHub()

	InitializeAPI(cfg, hub)

	router := gin.New()
	router.GET("/api/data/recent", GetRecentData)

	req, _ := http.NewRequest("GET", "/api/data/recent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "status")
}

func TestGetTenantDataHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		DataServiceURL: "http://test-service",
		MaxDataPoints:  100,
	}
	hub := websocket.NewHub()

	InitializeAPI(cfg, hub)

	router := gin.New()
	router.GET("/api/data/:tenant_id", GetTenantData)

	req, _ := http.NewRequest("GET", "/api/data/test-tenant", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "status")
}

func TestSystemStatusHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		DataServiceURL: "http://test-service",
		MaxDataPoints:  100,
	}
	hub := websocket.NewHub()

	InitializeAPI(cfg, hub)

	router := gin.New()
	router.GET("/api/status", SystemStatus)

	req, _ := http.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "status")
}
