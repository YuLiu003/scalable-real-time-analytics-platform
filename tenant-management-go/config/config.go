package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config represents the application configuration
type Config struct {
	Port              int
	DatabaseURL       string
	DatabaseType      string
	JWTSecret         string
	APIKeyEnabled     bool
	DefaultQuotaLimit int
	DefaultRateLimit  int
	TierConfigs       map[string]TierConfig
	DebugMode         bool
}

// TierConfig represents tier-specific configuration
type TierConfig struct {
	SamplingRate     float64
	AnomalyThreshold float64
	QuotaLimit       int
	RateLimit        int
	RetentionDays    int
}

// GetConfig retrieves the application configuration from environment variables
func GetConfig() *Config {
	port, err := strconv.Atoi(getEnv("PORT", "5010"))
	if err != nil {
		log.Printf("Invalid PORT, using default: %v", err)
		port = 5010
	}

	defaultQuotaLimit, err := strconv.Atoi(getEnv("DEFAULT_QUOTA_LIMIT", "10000"))
	if err != nil {
		log.Printf("Invalid DEFAULT_QUOTA_LIMIT, using default: %v", err)
		defaultQuotaLimit = 10000
	}

	defaultRateLimit, err := strconv.Atoi(getEnv("DEFAULT_RATE_LIMIT", "100"))
	if err != nil {
		log.Printf("Invalid DEFAULT_RATE_LIMIT, using default: %v", err)
		defaultRateLimit = 100
	}

	// Get JWT secret from environment, with a clear warning if using default
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		jwtSecret = "secret-tenant-management-token"
		log.Printf("WARNING: Using default JWT_SECRET. This is insecure for production environments.")
	}

	return &Config{
		Port:              port,
		DatabaseURL:       getEnv("DATABASE_URL", "sqlite3://tenant-mgmt.db"),
		DatabaseType:      getEnv("DATABASE_TYPE", "sqlite"),
		JWTSecret:         jwtSecret,
		APIKeyEnabled:     getEnvBool("API_KEY_ENABLED", true),
		DefaultQuotaLimit: defaultQuotaLimit,
		DefaultRateLimit:  defaultRateLimit,
		TierConfigs: map[string]TierConfig{
			"free": {
				SamplingRate:     0.5,
				AnomalyThreshold: 1.0,
				QuotaLimit:       5000,
				RateLimit:        20,
				RetentionDays:    7,
			},
			"standard": {
				SamplingRate:     0.8,
				AnomalyThreshold: 0.8,
				QuotaLimit:       50000,
				RateLimit:        100,
				RetentionDays:    30,
			},
			"premium": {
				SamplingRate:     1.0,
				AnomalyThreshold: 0.5,
				QuotaLimit:       500000,
				RateLimit:        1000,
				RetentionDays:    90,
			},
		},
		DebugMode: getEnvBool("DEBUG_MODE", false),
	}
}

// GetTierConfig returns configuration for a specific tier
func (c *Config) GetTierConfig(tier string) TierConfig {
	tier = strings.ToLower(tier)
	if config, exists := c.TierConfigs[tier]; exists {
		return config
	}

	// Return standard tier config as default
	return c.TierConfigs["standard"]
}

// Helper function to get environment variable with default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to get boolean environment variable
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolValue
}
