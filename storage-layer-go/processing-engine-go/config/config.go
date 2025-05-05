package config

import (
	"os"
	"strconv"
)

// Config represents the application configuration
type Config struct {
	KafkaBroker       string
	InputTopic        string
	OutputTopic       string
	ConsumerGroup     string
	MetricsPort       int
	StorageServiceURL string
	MaxReadings       int
	KafkaEnabled      bool
	Debug             bool
}

// DefaultTenantConfigs defines default processing parameters for tenants
var DefaultTenantConfigs = map[string]struct {
	AnomalyThreshold float64
	SamplingRate     float64
}{
	"tenant1": {
		AnomalyThreshold: 3.0,
		SamplingRate:     1.0,
	},
	"tenant2": {
		AnomalyThreshold: 2.5,
		SamplingRate:     0.5,
	},
}

// DefaultConfig is the default processing parameters
var DefaultConfig = struct {
	AnomalyThreshold float64
	SamplingRate     float64
}{
	AnomalyThreshold: 2.0,
	SamplingRate:     1.0,
}

// GetConfig loads the configuration from environment variables
func GetConfig() *Config {
	metricsPort, _ := strconv.Atoi(getEnv("METRICS_PORT", "8000"))
	maxReadings, _ := strconv.Atoi(getEnv("MAX_READINGS", "100"))

	return &Config{
		KafkaBroker:       getEnv("KAFKA_BROKER", "localhost:9092"),
		InputTopic:        getEnv("INPUT_TOPIC", "sensor-data"),
		OutputTopic:       getEnv("OUTPUT_TOPIC", "processed-data"),
		ConsumerGroup:     getEnv("CONSUMER_GROUP", "processing-engine-go-new"),
		MetricsPort:       metricsPort,
		StorageServiceURL: getEnv("STORAGE_SERVICE_URL", "http://storage-layer-service"),
		MaxReadings:       maxReadings,
		KafkaEnabled:      getEnv("KAFKA_ENABLED", "true") == "true",
		Debug:             getEnv("DEBUG", "false") == "true",
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetTenantConfig returns the configuration for a specific tenant
func GetTenantConfig(tenantID string) (float64, float64) {
	if config, exists := DefaultTenantConfigs[tenantID]; exists {
		return config.AnomalyThreshold, config.SamplingRate
	}
	return DefaultConfig.AnomalyThreshold, DefaultConfig.SamplingRate
}
