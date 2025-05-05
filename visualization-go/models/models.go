package models

import "time"

// SensorData represents the data from sensors
type SensorData struct {
	DeviceID    string     `json:"device_id"`
	Temperature *float64   `json:"temperature,omitempty"`
	Humidity    *float64   `json:"humidity,omitempty"`
	Timestamp   *time.Time `json:"timestamp,omitempty"`
	TenantID    string     `json:"tenant_id,omitempty"`
}

// ProcessedData represents data after processing
type ProcessedData struct {
	DeviceID     string     `json:"device_id"`
	Temperature  *float64   `json:"temperature,omitempty"`
	Humidity     *float64   `json:"humidity,omitempty"`
	Timestamp    *time.Time `json:"timestamp,omitempty"`
	ProcessedAt  time.Time  `json:"processed_at"`
	IsAnomaly    bool       `json:"is_anomaly"`
	AnomalyScore float64    `json:"anomaly_score"`
	TenantID     string     `json:"tenant_id"`
}

// TenantConfig represents tenant-specific configuration
type TenantConfig struct {
	AnomalyThreshold float64 `json:"anomaly_threshold"`
	SamplingRate     float64 `json:"sampling_rate"`
}

// AnalyticsResult represents aggregated statistics
type AnalyticsResult struct {
	Average   float64         `json:"average"`
	Max       float64         `json:"max"`
	Min       float64         `json:"min"`
	Count     int             `json:"count"`
	Anomalies []AnalyticsData `json:"anomalies,omitempty"`
}

// AnalyticsData represents a data point for analytics
type AnalyticsData struct {
	Timestamp    string   `json:"timestamp"`
	Value        float64  `json:"value"`
	DeviceID     string   `json:"device_id"`
	AnomalyScore *float64 `json:"anomaly_score,omitempty"`
}

// ServiceStatus represents a service health status
type ServiceStatus struct {
	Status    string `json:"status"`
	Service   string `json:"service,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message,omitempty"`
}

// SystemStatus represents the overall system status
type SystemStatus struct {
	Status    string                   `json:"status"`
	Services  map[string]ServiceStatus `json:"services"`
	Timestamp string                   `json:"timestamp"`
}

// DataResponse represents a generic response
type DataResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// RecentDataResponse represents a response with recent data
type RecentDataResponse struct {
	Status string       `json:"status"`
	Data   []SensorData `json:"data"`
}
