package models

import (
	"time"
)

// TenantConfig stores tenant-specific processing parameters
type TenantConfig struct {
	AnomalyThreshold float64
	SamplingRate     float64
}

// SensorData represents raw sensor input data
type SensorData struct {
	DeviceID    string     `json:"device_id"`
	Temperature *float64   `json:"temperature,omitempty"`
	Humidity    *float64   `json:"humidity,omitempty"`
	Timestamp   *time.Time `json:"timestamp,omitempty"`
	TenantID    string     `json:"tenant_id,omitempty"`
}

// ProcessedData represents processed sensor data with analysis results
type ProcessedData struct {
	DeviceID     string     `json:"device_id"`
	Temperature  *float64   `json:"temperature,omitempty"`
	Humidity     *float64   `json:"humidity,omitempty"`
	Timestamp    *time.Time `json:"timestamp,omitempty"`
	ProcessedAt  time.Time  `json:"processed_at"`
	IsAnomaly    bool       `json:"is_anomaly"`
	AnomalyScore float64    `json:"anomaly_score"`
	TenantID     string     `json:"tenant_id,omitempty"`
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message,omitempty"`
}

// SystemStatus represents the status of the entire system
type SystemStatus struct {
	Status    string                   `json:"status"`
	Services  map[string]ServiceStatus `json:"services"`
	Timestamp string                   `json:"timestamp"`
}

// DataResponse is a response for data operations
type DataResponse struct {
	Status  string     `json:"status"`
	Message string     `json:"message,omitempty"`
	Data    SensorData `json:"data,omitempty"`
}

// RecentDataResponse is a response containing recent data
type RecentDataResponse struct {
	Status string       `json:"status"`
	Data   []SensorData `json:"data"`
}
