package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	Port           int
	DataServiceURL string
	MaxDataPoints  int
	DebugMode      bool
}

// Load initializes configuration from environment variables
func Load() *Config {
	return &Config{
		Port:           getEnvAsInt("PORT", 5003),
		DataServiceURL: getEnv("DATA_SERVICE_URL", "http://data-ingestion-service"),
		MaxDataPoints:  getEnvAsInt("MAX_DATA_POINTS", 100),
		DebugMode:      getEnvAsBool("DEBUG_MODE", false),
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
