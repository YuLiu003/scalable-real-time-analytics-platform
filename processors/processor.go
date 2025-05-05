package processors

import (
	"log"
	"math/rand"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/models"
)

// Default configurations
var (
	// TenantProcessingConfig stores tenant-specific processing parameters
	TenantProcessingConfig = map[string]models.TenantConfig{
		"tenant1": {
			AnomalyThreshold: 3.0,
			SamplingRate:     1.0,
		},
		"tenant2": {
			AnomalyThreshold: 2.5,
			SamplingRate:     0.5,
		},
	}

	// DefaultConfig is used for tenants without specific configurations
	DefaultConfig = models.TenantConfig{
		AnomalyThreshold: 2.0,
		SamplingRate:     1.0,
	}
)

// GetTenantConfig returns tenant-specific configuration or default
func GetTenantConfig(tenantID string) models.TenantConfig {
	config, exists := TenantProcessingConfig[tenantID]
	if !exists {
		return DefaultConfig
	}
	return config
}

// ProcessMessage processes a message with tenant context preservation
func ProcessMessage(sensorData models.SensorData) (*models.ProcessedData, error) {
	// Validate tenant
	if sensorData.TenantID == "" {
		log.Println("Message received without tenant_id")
		return nil, nil
	}

	// Get tenant-specific configuration
	config := GetTenantConfig(sensorData.TenantID)

	// Apply sampling rate (skip some messages for tenants with lower sampling)
	if config.SamplingRate < 1.0 && rand.Float64() > config.SamplingRate {
		log.Printf("Skipping message due to tenant %s sampling rate", sensorData.TenantID)
		return nil, nil
	}

	// Check required fields
	if sensorData.DeviceID == "" || sensorData.Temperature == nil || sensorData.Humidity == nil {
		log.Printf("Missing required fields in message for device %s", sensorData.DeviceID)
		return nil, nil
	}

	// Process data
	anomalyScore := 0.0

	// Check if temperature is outside normal range
	if *sensorData.Temperature > 30 || *sensorData.Temperature < 10 {
		anomalyScore += 1.0
	}

	// Check if humidity is outside normal range
	if *sensorData.Humidity > 80 || *sensorData.Humidity < 20 {
		anomalyScore += 1.0
	}

	// Add anomaly flag if score exceeds threshold
	isAnomaly := anomalyScore >= config.AnomalyThreshold

	// Create processed result (preserving tenant context)
	result := &models.ProcessedData{
		DeviceID:     sensorData.DeviceID,
		Temperature:  sensorData.Temperature,
		Humidity:     sensorData.Humidity,
		Timestamp:    sensorData.Timestamp,
		ProcessedAt:  time.Now(),
		IsAnomaly:    isAnomaly,
		AnomalyScore: anomalyScore,
		TenantID:     sensorData.TenantID, // Preserve tenant context
	}

	return result, nil
}
