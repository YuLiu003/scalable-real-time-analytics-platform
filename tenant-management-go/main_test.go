package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/config"
	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/handlers"
	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() (*gin.Engine, *MemoryTenantStore) {
	// Switch to test mode
	gin.SetMode(gin.TestMode)

	// Create test store
	store := NewMemoryTenantStore()

	// Create test tenants
	createTestTenants(store)

	// Create router
	r := gin.New()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "tenant-management-go",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Create tenant handler
	tenantHandler := handlers.NewTenantHandler(store)

	// API routes
	api := r.Group("/api")
	tenants := api.Group("/tenants")
	tenants.GET("", tenantHandler.ListTenants)
	tenants.GET("/:id", tenantHandler.GetTenant)
	tenants.POST("", tenantHandler.CreateTenant)
	tenants.PUT("/:id", tenantHandler.UpdateTenant)
	tenants.DELETE("/:id", tenantHandler.DeleteTenant)
	tenants.GET("/:id/stats", tenantHandler.GetTenantStats)
	tenants.GET("/:id/quota", tenantHandler.GetTenantQuota)

	// API key validation endpoint
	api.GET("/validate", tenantHandler.ValidateAPIKey)

	return r, store
}

func createTestTenants(store *MemoryTenantStore) {
	testTenants := []*models.Tenant{
		{
			ID:            "test-tenant-1",
			Name:          "Test Tenant 1",
			APIKey:        "test-api-key-1",
			Tier:          "free",
			Active:        true,
			CreatedAt:     time.Now(),
			QuotaLimit:    1000,
			RateLimit:     10,
			SamplingRate:  0.5,
			RetentionDays: 7,
		},
		{
			ID:            "test-tenant-2",
			Name:          "Test Tenant 2",
			APIKey:        "test-api-key-2",
			Tier:          "premium",
			Active:        true,
			CreatedAt:     time.Now(),
			QuotaLimit:    10000,
			RateLimit:     100,
			SamplingRate:  1.0,
			RetentionDays: 90,
		},
		{
			ID:            "inactive-tenant",
			Name:          "Inactive Tenant",
			APIKey:        "inactive-api-key",
			Tier:          "standard",
			Active:        false,
			CreatedAt:     time.Now(),
			QuotaLimit:    5000,
			RateLimit:     50,
			SamplingRate:  0.8,
			RetentionDays: 30,
		},
	}

	for _, tenant := range testTenants {
		store.CreateTenant(tenant)
	}
}

func TestMain(t *testing.T) {
	// Test that the package builds and initializes correctly
	t.Log("Tenant-management-go package builds successfully")
}

func TestMemoryTenantStore_GetTenants(t *testing.T) {
	store := NewMemoryTenantStore()
	createTestTenants(store)

	tenants, err := store.GetTenants()
	assert.NoError(t, err)
	assert.Len(t, tenants, 3)

	// Verify tenant data
	tenantNames := make(map[string]bool)
	for _, tenant := range tenants {
		tenantNames[tenant.Name] = true
	}

	assert.True(t, tenantNames["Test Tenant 1"])
	assert.True(t, tenantNames["Test Tenant 2"])
	assert.True(t, tenantNames["Inactive Tenant"])
}

func TestMemoryTenantStore_GetTenant(t *testing.T) {
	store := NewMemoryTenantStore()
	createTestTenants(store)

	tests := []struct {
		name      string
		tenantID  string
		wantError bool
	}{
		{"existing tenant", "test-tenant-1", false},
		{"non-existing tenant", "non-existent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant, err := store.GetTenant(tt.tenantID)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, tenant)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tenant)
				assert.Equal(t, tt.tenantID, tenant.ID)
			}
		})
	}
}

func TestMemoryTenantStore_GetTenantByAPIKey(t *testing.T) {
	store := NewMemoryTenantStore()
	createTestTenants(store)

	tests := []struct {
		name       string
		apiKey     string
		wantError  bool
		expectedID string
	}{
		{"valid API key 1", "test-api-key-1", false, "test-tenant-1"},
		{"valid API key 2", "test-api-key-2", false, "test-tenant-2"},
		{"invalid API key", "invalid-key", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant, err := store.GetTenantByAPIKey(tt.apiKey)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, tenant)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tenant)
				assert.Equal(t, tt.expectedID, tenant.ID)
				assert.Equal(t, tt.apiKey, tenant.APIKey)
			}
		})
	}
}

func TestMemoryTenantStore_CreateTenant(t *testing.T) {
	store := NewMemoryTenantStore()

	newTenant := &models.Tenant{
		ID:            "new-tenant",
		Name:          "New Test Tenant",
		APIKey:        "new-api-key",
		Tier:          "free",
		Active:        true,
		CreatedAt:     time.Now(),
		QuotaLimit:    1000,
		RateLimit:     10,
		SamplingRate:  0.5,
		RetentionDays: 7,
	}

	err := store.CreateTenant(newTenant)
	assert.NoError(t, err)

	// Verify tenant was created
	retrieved, err := store.GetTenant("new-tenant")
	assert.NoError(t, err)
	assert.Equal(t, newTenant.Name, retrieved.Name)
	assert.Equal(t, newTenant.APIKey, retrieved.APIKey)
}

func TestMemoryTenantStore_UpdateTenant(t *testing.T) {
	store := NewMemoryTenantStore()
	createTestTenants(store)

	// Get existing tenant
	tenant, err := store.GetTenant("test-tenant-1")
	require.NoError(t, err)

	// Update tenant
	tenant.Name = "Updated Tenant Name"
	tenant.Tier = "premium"
	tenant.QuotaLimit = 50000

	err = store.UpdateTenant(tenant)
	assert.NoError(t, err)

	// Verify update
	updated, err := store.GetTenant("test-tenant-1")
	assert.NoError(t, err)
	assert.Equal(t, "Updated Tenant Name", updated.Name)
	assert.Equal(t, "premium", updated.Tier)
	assert.Equal(t, 50000, updated.QuotaLimit)
}

func TestMemoryTenantStore_DeleteTenant(t *testing.T) {
	store := NewMemoryTenantStore()
	createTestTenants(store)

	// Verify tenant exists before deletion
	_, err := store.GetTenant("test-tenant-1")
	assert.NoError(t, err)

	// Delete tenant
	err = store.DeleteTenant("test-tenant-1")
	assert.NoError(t, err)

	// Verify tenant no longer exists
	_, err = store.GetTenant("test-tenant-1")
	assert.Error(t, err)
}

func TestMemoryTenantStore_GetTenantStats(t *testing.T) {
	store := NewMemoryTenantStore()

	// Test getting stats for non-existing tenant (should return empty stats)
	stats, err := store.GetTenantStats("non-existent")
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "non-existent", stats.TenantID)
}

func TestMemoryTenantStore_GetTenantQuota(t *testing.T) {
	store := NewMemoryTenantStore()

	// Test getting quota for non-existing tenant (should return empty quota)
	quota, err := store.GetTenantQuota("non-existent")
	assert.NoError(t, err)
	assert.NotNil(t, quota)
	assert.Equal(t, "non-existent", quota.TenantID)
}

func TestMemoryTenantStore_UpdateTenantQuota(t *testing.T) {
	store := NewMemoryTenantStore()

	quota := &models.TenantQuota{
		TenantID:    "test-tenant",
		LastUpdated: time.Now(),
	}

	err := store.UpdateTenantQuota(quota)
	assert.NoError(t, err)

	// Verify quota was stored
	retrieved, err := store.GetTenantQuota("test-tenant")
	assert.NoError(t, err)
	assert.Equal(t, quota.TenantID, retrieved.TenantID)
}

func TestHealthEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "tenant-management-go", response["service"])
}

func TestListTenantsEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/tenants", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(3), response["count"]) // Should have 3 test tenants
}

func TestGetTenantEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	tests := []struct {
		name           string
		tenantID       string
		expectedStatus int
	}{
		{"existing tenant", "test-tenant-1", http.StatusOK},
		{"non-existing tenant", "non-existent", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/api/tenants/"+tt.tenantID, nil)
			require.NoError(t, err)

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "success", response["status"])
			}
		})
	}
}

func TestCreateTenantEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	newTenant := map[string]interface{}{
		"name":           "API Created Tenant",
		"tier":           "free",
		"active":         true,
		"quota_limit":    2000,
		"rate_limit":     20,
		"sampling_rate":  0.6,
		"retention_days": 14,
	}

	jsonData, err := json.Marshal(newTenant)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/tenants", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

func TestCreateTenantEndpoint_InvalidData(t *testing.T) {
	router, _ := setupTestRouter()

	tests := []struct {
		name string
		data string
	}{
		{"invalid JSON", `{"name": "Test", "invalid_json": }`},
		{"empty data", `{}`},
		{"missing required fields", `{"name": "Test"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/api/tenants", bytes.NewBufferString(tt.data))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.True(t, w.Code >= 400 && w.Code < 500, "Expected client error status")
		})
	}
}

func TestUpdateTenantEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	updateData := map[string]interface{}{
		"name":           "Updated Tenant Name",
		"tier":           "premium",
		"quota_limit":    100000,
		"rate_limit":     500,
		"sampling_rate":  1.0,
		"retention_days": 90,
	}

	jsonData, err := json.Marshal(updateData)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("PUT", "/api/tenants/test-tenant-1", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

func TestDeleteTenantEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", "/api/tenants/test-tenant-1", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])

	// Verify tenant was deleted
	w2 := httptest.NewRecorder()
	req2, err := http.NewRequest("GET", "/api/tenants/test-tenant-1", nil)
	require.NoError(t, err)

	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestGetTenantStatsEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/tenants/test-tenant-1/stats", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

func TestGetTenantQuotaEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/tenants/test-tenant-1/quota", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

func TestValidateAPIKeyEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
	}{
		{"valid API key", "test-api-key-1", http.StatusOK},
		{"invalid API key", "invalid-key", http.StatusUnauthorized},
		{"missing API key", "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/api/validate", nil)
			require.NoError(t, err)

			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestGetEnvWithDefault(t *testing.T) {
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

			result := getEnvWithDefault(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateDefaultTenants(t *testing.T) {
	store := NewMemoryTenantStore()

	// Set test environment variables
	os.Setenv("API_KEY_1", "test-env-key-1")
	os.Setenv("API_KEY_2", "test-env-key-2")
	os.Setenv("API_KEY_3", "test-env-key-3")
	defer func() {
		os.Unsetenv("API_KEY_1")
		os.Unsetenv("API_KEY_2")
		os.Unsetenv("API_KEY_3")
	}()

	createDefaultTenants(store)

	// Verify default tenants were created
	tenants, err := store.GetTenants()
	assert.NoError(t, err)
	assert.Len(t, tenants, 3)

	// Check tenant tiers
	tierCounts := make(map[string]int)
	for _, tenant := range tenants {
		tierCounts[tenant.Tier]++
	}

	assert.Equal(t, 1, tierCounts["free"])
	assert.Equal(t, 1, tierCounts["standard"])
	assert.Equal(t, 1, tierCounts["premium"])
}

func TestTenantTierConfiguration(t *testing.T) {
	store := NewMemoryTenantStore()
	createDefaultTenants(store)

	tenants, err := store.GetTenants()
	require.NoError(t, err)

	// Find each tier and verify configuration
	for _, tenant := range tenants {
		switch tenant.Tier {
		case "free":
			assert.Equal(t, 5000, tenant.QuotaLimit)
			assert.Equal(t, 20, tenant.RateLimit)
			assert.Equal(t, 0.5, tenant.SamplingRate)
			assert.Equal(t, 7, tenant.RetentionDays)
		case "standard":
			assert.Equal(t, 50000, tenant.QuotaLimit)
			assert.Equal(t, 100, tenant.RateLimit)
			assert.Equal(t, 0.8, tenant.SamplingRate)
			assert.Equal(t, 30, tenant.RetentionDays)
		case "premium":
			assert.Equal(t, 500000, tenant.QuotaLimit)
			assert.Equal(t, 1000, tenant.RateLimit)
			assert.Equal(t, 1.0, tenant.SamplingRate)
			assert.Equal(t, 90, tenant.RetentionDays)
		}
	}
}

func TestTableDrivenAPIEndpoints(t *testing.T) {
	router, _ := setupTestRouter()

	endpoints := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"health check", "GET", "/health", http.StatusOK},
		{"list tenants", "GET", "/api/tenants", http.StatusOK},
		{"get tenant", "GET", "/api/tenants/test-tenant-1", http.StatusOK},
		{"get non-existent tenant", "GET", "/api/tenants/non-existent", http.StatusNotFound},
		{"get tenant stats", "GET", "/api/tenants/test-tenant-1/stats", http.StatusOK},
		{"get tenant quota", "GET", "/api/tenants/test-tenant-1/quota", http.StatusOK},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest(endpoint.method, endpoint.path, nil)
			require.NoError(t, err)

			router.ServeHTTP(w, req)

			assert.Equal(t, endpoint.expectedStatus, w.Code, "Unexpected status for %s %s", endpoint.method, endpoint.path)
		})
	}
}

func TestTenantActivationStates(t *testing.T) {
	store := NewMemoryTenantStore()
	createTestTenants(store)

	// Verify active tenants
	activeTenant, err := store.GetTenant("test-tenant-1")
	assert.NoError(t, err)
	assert.True(t, activeTenant.Active)

	// Verify inactive tenant
	inactiveTenant, err := store.GetTenant("inactive-tenant")
	assert.NoError(t, err)
	assert.False(t, inactiveTenant.Active)
}

func TestConfigurationLoading(t *testing.T) {
	// Test configuration loading indirectly by ensuring the service starts correctly
	// This is tested through other endpoint tests, but we can add specific config tests here
	cfg := config.GetConfig()
	assert.NotNil(t, cfg)
}
