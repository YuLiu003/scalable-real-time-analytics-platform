package models

import "time"

// SensorData represents raw data coming from sensors
type SensorData struct {
	DeviceID    string     `json:"device_id"`
	Temperature float64    `json:"temperature"`
	Humidity    float64    `json:"humidity"`
	Timestamp   *time.Time `json:"timestamp,omitempty"`
	TenantID    string     `json:"tenant_id,omitempty"`
}

// DeviceStats represents statistics for a specific device
type DeviceStats struct {
	Temperature struct {
		Readings []float64 `json:"readings"`
		Avg      float64   `json:"avg"`
		Min      float64   `json:"min"`
		Max      float64   `json:"max"`
	} `json:"temperature"`
	Humidity struct {
		Readings []float64 `json:"readings"`
		Avg      float64   `json:"avg"`
		Min      float64   `json:"min"`
		Max      float64   `json:"max"`
	} `json:"humidity"`
	LastSeen *time.Time `json:"last_seen,omitempty"`
}

// ProcessedStats represents the overall processed statistics
type ProcessedStats struct {
	Temperature struct {
		Count int     `json:"count"`
		Sum   float64 `json:"sum"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
		Avg   float64 `json:"avg"`
	} `json:"temperature"`
	Humidity struct {
		Count int     `json:"count"`
		Sum   float64 `json:"sum"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
		Avg   float64 `json:"avg"`
	} `json:"humidity"`
	Devices map[string]*DeviceStats `json:"devices"`
}

// ProcessingStatus represents the current processing status
type ProcessingStatus struct {
	Running           bool   `json:"running"`
	MessagesProcessed int    `json:"messages_processed"`
	LastProcessed     string `json:"last_processed,omitempty"`
	Errors            int    `json:"errors"`
}

// ProcessedDataPayload represents the processed data to be sent to storage
type ProcessedDataPayload struct {
	DeviceID         string  `json:"device_id"`
	Temperature      float64 `json:"temperature"`
	Humidity         float64 `json:"humidity"`
	Timestamp        string  `json:"timestamp"`
	ProcessedAt      string  `json:"processed_at"`
	TenantID         string  `json:"tenant_id,omitempty"`
	IsAnomaly        bool    `json:"is_anomaly,omitempty"`
	AnomalyScore     float64 `json:"anomaly_score,omitempty"`
	TemperatureStats struct {
		Avg float64 `json:"avg"`
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"temperature_stats"`
	HumidityStats struct {
		Avg float64 `json:"avg"`
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"humidity_stats"`
}

// AnalyticsData represents a single data point for analytics
type AnalyticsData struct {
	Timestamp    string  `json:"timestamp"`
	Value        float64 `json:"value"`
	DeviceID     string  `json:"device_id"`
	AnomalyScore float64 `json:"anomaly_score,omitempty"`
}

// AnalyticsResult represents the result of analytics operations
type AnalyticsResult struct {
	Average   float64         `json:"average"`
	Max       float64         `json:"max"`
	Min       float64         `json:"min"`
	Count     int             `json:"count"`
	Anomalies []AnalyticsData `json:"anomalies,omitempty"`
}

// TenantConfig represents tenant-specific configuration
type TenantConfig struct {
	AnomalyThreshold float64 `json:"anomaly_threshold"`
	SamplingRate     float64 `json:"sampling_rate"`
}
