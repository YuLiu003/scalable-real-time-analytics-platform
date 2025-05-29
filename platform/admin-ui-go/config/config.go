package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	Port                int
	TenantServiceURL    string
	APIGatewayURL       string
	DataServiceURL      string
	ProcessingEngineURL string
	StorageLayerURL     string
	KafkaURL            string
	DebugMode           bool
}

// Load initializes configuration from environment variables
func Load() *Config {
	return &Config{
		Port:                getEnvAsInt("PORT", 5050),
		TenantServiceURL:    getEnv("TENANT_SERVICE_URL", "http://tenant-management-service:5010"),
		APIGatewayURL:       getEnv("API_GATEWAY_URL", "http://gin-api-service"),
		DataServiceURL:      getEnv("DATA_SERVICE_URL", "http://data-ingestion-service"),
		ProcessingEngineURL: getEnv("PROCESSING_ENGINE_URL", "http://processing-engine-service"),
		StorageLayerURL:     getEnv("STORAGE_LAYER_URL", "http://storage-layer-service"),
		KafkaURL:            getEnv("KAFKA_URL", "http://kafka:9092"),
		DebugMode:           getEnvAsBool("DEBUG_MODE", false),
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt gets an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsBool gets an environment variable as a boolean or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}
