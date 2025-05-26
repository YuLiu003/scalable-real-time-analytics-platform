// Package main provides the entry point for the tenant management service.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/config"
	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/handlers"
	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/models"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// In-memory implementation of TenantStore for quick testing
type MemoryTenantStore struct {
	tenants map[string]*models.Tenant
	stats   map[string]*models.TenantStats
	quotas  map[string]*models.TenantQuota
}

func NewMemoryTenantStore() *MemoryTenantStore {
	return &MemoryTenantStore{
		tenants: make(map[string]*models.Tenant),
		stats:   make(map[string]*models.TenantStats),
		quotas:  make(map[string]*models.TenantQuota),
	}
}

func (s *MemoryTenantStore) GetTenants() ([]models.Tenant, error) {
	var tenants []models.Tenant
	for _, t := range s.tenants {
		tenants = append(tenants, *t)
	}
	return tenants, nil
}

func (s *MemoryTenantStore) GetTenant(id string) (*models.Tenant, error) {
	t, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant not found")
	}
	return t, nil
}

func (s *MemoryTenantStore) GetTenantByApiKey(apiKey string) (*models.Tenant, error) {
	for _, t := range s.tenants {
		if t.ApiKey == apiKey {
			return t, nil
		}
	}
	return nil, fmt.Errorf("tenant not found")
}

func (s *MemoryTenantStore) CreateTenant(tenant *models.Tenant) error {
	s.tenants[tenant.ID] = tenant
	return nil
}

func (s *MemoryTenantStore) UpdateTenant(tenant *models.Tenant) error {
	s.tenants[tenant.ID] = tenant
	return nil
}

func (s *MemoryTenantStore) DeleteTenant(id string) error {
	delete(s.tenants, id)
	return nil
}

func (s *MemoryTenantStore) GetTenantStats(id string) (*models.TenantStats, error) {
	stats, ok := s.stats[id]
	if !ok {
		// Return empty stats if not found
		return &models.TenantStats{
			TenantID: id,
		}, nil
	}
	return stats, nil
}

func (s *MemoryTenantStore) GetTenantQuota(id string) (*models.TenantQuota, error) {
	quota, ok := s.quotas[id]
	if !ok {
		// Return empty quota if not found
		return &models.TenantQuota{
			TenantID:    id,
			LastUpdated: time.Now(),
		}, nil
	}
	return quota, nil
}

func (s *MemoryTenantStore) UpdateTenantQuota(quota *models.TenantQuota) error {
	s.quotas[quota.TenantID] = quota
	return nil
}

func main() {
	// Load configuration
	cfg := config.GetConfig()

	if !cfg.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize the store
	store := NewMemoryTenantStore()

	// Create default tenants for testing
	createDefaultTenants(store)

	// Create the router
	r := gin.Default()

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
	{
		// Tenant management endpoints
		tenants := api.Group("/tenants")
		{
			tenants.GET("", tenantHandler.ListTenants)
			tenants.GET("/:id", tenantHandler.GetTenant)
			tenants.POST("", tenantHandler.CreateTenant)
			tenants.PUT("/:id", tenantHandler.UpdateTenant)
			tenants.DELETE("/:id", tenantHandler.DeleteTenant)
			tenants.GET("/:id/stats", tenantHandler.GetTenantStats)
			tenants.GET("/:id/quota", tenantHandler.GetTenantQuota)
		}

		// API key validation endpoint
		api.GET("/validate", tenantHandler.ValidateAPIKey)
	}

	// Prometheus metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start the server
	port := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting tenant management service on %s", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// createDefaultTenants creates default tenants for testing
func createDefaultTenants(store *MemoryTenantStore) {
	// Get API keys from environment variables or use default values if not set
	apiKey1 := getEnvWithDefault("API_KEY_1", "test-key-1")
	apiKey2 := getEnvWithDefault("API_KEY_2", "test-key-2")
	apiKey3 := getEnvWithDefault("API_KEY_3", "test-key-3")

	// Create a free tier tenant
	freeTenant := &models.Tenant{
		ID:            "tenant1",
		Name:          "Free Tier Test",
		ApiKey:        apiKey1,
		Tier:          "free",
		Active:        true,
		CreatedAt:     time.Now(),
		QuotaLimit:    5000,
		RateLimit:     20,
		SamplingRate:  0.5,
		RetentionDays: 7,
	}
	if err := store.CreateTenant(freeTenant); err != nil {
		log.Printf("Failed to create free tenant: %v", err)
	}

	// Create a standard tier tenant
	standardTenant := &models.Tenant{
		ID:            "tenant2",
		Name:          "Standard Tier Test",
		ApiKey:        apiKey2,
		Tier:          "standard",
		Active:        true,
		CreatedAt:     time.Now(),
		QuotaLimit:    50000,
		RateLimit:     100,
		SamplingRate:  0.8,
		RetentionDays: 30,
	}
	if err := store.CreateTenant(standardTenant); err != nil {
		log.Printf("Failed to create standard tenant: %v", err)
	}

	// Create a premium tier tenant
	premiumTenant := &models.Tenant{
		ID:            "tenant3",
		Name:          "Premium Tier Test",
		ApiKey:        apiKey3,
		Tier:          "premium",
		Active:        true,
		CreatedAt:     time.Now(),
		QuotaLimit:    500000,
		RateLimit:     1000,
		SamplingRate:  1.0,
		RetentionDays: 90,
	}
	if err := store.CreateTenant(premiumTenant); err != nil {
		log.Printf("Failed to create premium tenant: %v", err)
	}

	log.Println("Created default tenants for testing")
}

// Helper function to get environment variable with default value
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
