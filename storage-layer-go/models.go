package main

import "time"

// SensorData represents the processed sensor data that's stored in the database
type SensorData struct {
	ID           int64     `json:"id,omitempty"`
	DeviceID     string    `json:"device_id"`
	Temperature  *float64  `json:"temperature,omitempty"`
	Humidity     *float64  `json:"humidity,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	IsAnomaly    bool      `json:"is_anomaly,omitempty"`
	AnomalyScore *float64  `json:"anomaly_score,omitempty"`
	TenantID     string    `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// SensorDataRequest represents the data coming in from the API
type SensorDataRequest struct {
	DeviceID     string    `json:"device_id" binding:"required"`
	Temperature  *float64  `json:"temperature"`
	Humidity     *float64  `json:"humidity"`
	Timestamp    time.Time `json:"timestamp"`
	IsAnomaly    *bool     `json:"is_anomaly"`
	AnomalyScore *float64  `json:"anomaly_score"`
	TenantID     string    `json:"tenant_id" binding:"required"`
	ProcessedAt  time.Time `json:"processed_at,omitempty"` // Set this if missing
}

// RetentionPolicy represents the data retention policy for a tenant
type RetentionPolicy struct {
	Days int `json:"days"`
}

// TenantStats represents aggregated statistics for a tenant
type TenantStats struct {
	DeviceCount    int     `json:"device_count"`
	TotalRecords   int     `json:"total_records"`
	AnomalyCount   int     `json:"anomaly_count"`
	AvgTemperature float64 `json:"avg_temperature,omitempty"`
	AvgHumidity    float64 `json:"avg_humidity,omitempty"`
	LastUpdated    string  `json:"last_updated"`
}
