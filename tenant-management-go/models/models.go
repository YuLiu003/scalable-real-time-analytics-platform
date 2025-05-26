// Package models defines the data structures for tenant management.
package models

import (
	"time"
)

// Tenant represents a tenant in the platform
type Tenant struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name"`
	ApiKey        string    `json:"api_key"`
	Tier          string    `json:"tier"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	QuotaLimit    int       `json:"quota_limit"`    // Messages per day
	RateLimit     int       `json:"rate_limit"`     // Messages per minute
	SamplingRate  float64   `json:"sampling_rate"`  // Percentage of messages to process
	RetentionDays int       `json:"retention_days"` // Data retention in days
}

// TenantConfig represents tenant configuration for other services
type TenantConfig struct {
	ID               string  `json:"id"`
	ApiKey           string  `json:"api_key"`
	Tier             string  `json:"tier"`
	SamplingRate     float64 `json:"sampling_rate"`
	AnomalyThreshold float64 `json:"anomaly_threshold"`
	DataRetention    int     `json:"data_retention"`
	Active           bool    `json:"active"`
}

// TenantQuota represents tenant usage quotas
type TenantQuota struct {
	TenantID        string    `json:"tenant_id"`
	DailyUsage      int       `json:"daily_usage"`
	MonthlyUsage    int       `json:"monthly_usage"`
	LastUpdated     time.Time `json:"last_updated"`
	QuotaExceeded   bool      `json:"quota_exceeded"`
	RateLimitStatus bool      `json:"rate_limit_status"`
}

// TenantStats represents tenant analytics statistics
type TenantStats struct {
	TenantID      string  `json:"tenant_id"`
	MessageCount  int     `json:"message_count"`
	AnomalyCount  int     `json:"anomaly_count"`
	AnomalyRate   float64 `json:"anomaly_rate"`
	AvgLatency    float64 `json:"avg_latency"`
	DeviceCount   int     `json:"device_count"`
	ProcessedData int64   `json:"processed_data"`
}

// TenantRequest represents a request to create/update a tenant
type TenantRequest struct {
	Name          string  `json:"name" binding:"required"`
	Tier          string  `json:"tier" binding:"required"`
	ApiKey        string  `json:"api_key,omitempty"`
	QuotaLimit    int     `json:"quota_limit"`
	RateLimit     int     `json:"rate_limit"`
	SamplingRate  float64 `json:"sampling_rate"`
	RetentionDays int     `json:"retention_days"`
	Active        bool    `json:"active"`
}
