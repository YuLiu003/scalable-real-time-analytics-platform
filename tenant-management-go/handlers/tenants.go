// Package handlers provides HTTP handlers for tenant management operations.
package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/tenant-management-go/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantStore represents the database interface for tenant operations
type TenantStore interface {
	GetTenants() ([]models.Tenant, error)
	GetTenant(id string) (*models.Tenant, error)
	GetTenantByApiKey(apiKey string) (*models.Tenant, error)
	CreateTenant(tenant *models.Tenant) error
	UpdateTenant(tenant *models.Tenant) error
	DeleteTenant(id string) error
	GetTenantStats(id string) (*models.TenantStats, error)
	GetTenantQuota(id string) (*models.TenantQuota, error)
	UpdateTenantQuota(quota *models.TenantQuota) error
}

// TenantHandler handles tenant-related API endpoints
type TenantHandler struct {
	store TenantStore
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(store TenantStore) *TenantHandler {
	return &TenantHandler{
		store: store,
	}
}

// ListTenants returns all tenants
func (h *TenantHandler) ListTenants(c *gin.Context) {
	tenants, err := h.store.GetTenants()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve tenants",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(tenants),
		"data":   tenants,
	})
}

// GetTenant returns a specific tenant
func (h *TenantHandler) GetTenant(c *gin.Context) {
	id := c.Param("id")
	tenant, err := h.store.GetTenant(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Tenant not found",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   tenant,
	})
}

// CreateTenant creates a new tenant
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req models.TenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// Generate a unique tenant ID and API key if not provided
	id := uuid.New().String()
	apiKey := req.ApiKey
	if apiKey == "" {
		apiKey = generateAPIKey()
	}

	tenant := &models.Tenant{
		ID:            id,
		Name:          req.Name,
		ApiKey:        apiKey,
		Tier:          req.Tier,
		Active:        req.Active,
		CreatedAt:     time.Now(),
		QuotaLimit:    req.QuotaLimit,
		RateLimit:     req.RateLimit,
		SamplingRate:  req.SamplingRate,
		RetentionDays: req.RetentionDays,
	}

	if err := h.store.CreateTenant(tenant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create tenant",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data":   tenant,
	})
}

// UpdateTenant updates an existing tenant
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id := c.Param("id")

	// Check if tenant exists
	existingTenant, err := h.store.GetTenant(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Tenant not found",
			"error":   err.Error(),
		})
		return
	}

	var req models.TenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// Update the tenant with new values
	existingTenant.Name = req.Name
	existingTenant.Tier = req.Tier
	if req.ApiKey != "" {
		existingTenant.ApiKey = req.ApiKey
	}
	existingTenant.Active = req.Active
	existingTenant.QuotaLimit = req.QuotaLimit
	existingTenant.RateLimit = req.RateLimit
	existingTenant.SamplingRate = req.SamplingRate
	existingTenant.RetentionDays = req.RetentionDays

	if err := h.store.UpdateTenant(existingTenant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to update tenant",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   existingTenant,
	})
}

// DeleteTenant deletes a tenant
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	id := c.Param("id")

	// Check if tenant exists
	_, err := h.store.GetTenant(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Tenant not found",
			"error":   err.Error(),
		})
		return
	}

	if err := h.store.DeleteTenant(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to delete tenant",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tenant deleted successfully",
	})
}

// GetTenantStats retrieves statistics for a tenant
func (h *TenantHandler) GetTenantStats(c *gin.Context) {
	id := c.Param("id")

	// Check if tenant exists
	_, err := h.store.GetTenant(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Tenant not found",
			"error":   err.Error(),
		})
		return
	}

	stats, err := h.store.GetTenantStats(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve tenant statistics",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   stats,
	})
}

// GetTenantQuota retrieves quota information for a tenant
func (h *TenantHandler) GetTenantQuota(c *gin.Context) {
	id := c.Param("id")

	// Check if tenant exists
	_, err := h.store.GetTenant(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Tenant not found",
			"error":   err.Error(),
		})
		return
	}

	quota, err := h.store.GetTenantQuota(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve tenant quota",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   quota,
	})
}

// ValidateAPIKey checks if the provided API key is valid and returns the tenant
func (h *TenantHandler) ValidateAPIKey(c *gin.Context) {
	apiKey := c.Query("api_key")
	if apiKey == "" {
		apiKey = c.GetHeader("X-API-Key")
	}

	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "API key is required",
		})
		return
	}

	tenant, err := h.store.GetTenantByApiKey(apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Invalid API key",
		})
		return
	}

	if !tenant.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Tenant is inactive",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": models.TenantConfig{
			ID:               tenant.ID,
			ApiKey:           tenant.ApiKey,
			Tier:             tenant.Tier,
			SamplingRate:     tenant.SamplingRate,
			AnomalyThreshold: getAnomalyThresholdByTier(tenant.Tier),
			DataRetention:    tenant.RetentionDays,
			Active:           tenant.Active,
		},
	})
}

// Helper functions

// generateAPIKey generates a random API key
func generateAPIKey() string {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to UUID if crypto/rand fails
		return "key-" + uuid.New().String()
	}
	// Encode to base64 and prefix with "key-"
	return "key-" + base64.URLEncoding.EncodeToString(bytes)[:32]
}

// getAnomalyThresholdByTier returns the anomaly threshold for a tier
func getAnomalyThresholdByTier(tier string) float64 {
	switch tier {
	case "free":
		return 1.0
	case "standard":
		return 0.8
	case "premium":
		return 0.5
	default:
		return 0.8
	}
}
