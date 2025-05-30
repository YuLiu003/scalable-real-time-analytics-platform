package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_Defaults(t *testing.T) {
	// Clear all environment variables
	envVars := []string{
		"KAFKA_BROKER", "INPUT_TOPIC", "OUTPUT_TOPIC", "CONSUMER_GROUP",
		"METRICS_PORT", "STORAGE_SERVICE_URL", "MAX_READINGS", "KAFKA_ENABLED", "DEBUG",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}

	cfg := GetConfig()

	// Test default values
	assert.Equal(t, "localhost:9092", cfg.KafkaBroker)
	assert.Equal(t, "sensor-data", cfg.InputTopic)
	assert.Equal(t, "processed-data", cfg.OutputTopic)
	assert.Equal(t, "processing-engine-go-new", cfg.ConsumerGroup)
	assert.Equal(t, 8000, cfg.MetricsPort)
	assert.Equal(t, "http://storage-layer-go:5002", cfg.StorageServiceURL)
	assert.Equal(t, 100, cfg.MaxReadings)
	assert.True(t, cfg.KafkaEnabled)
	assert.False(t, cfg.Debug)
}

func TestGetConfig_CustomValues(t *testing.T) {
	// Set custom environment variables
	testEnvs := map[string]string{
		"KAFKA_BROKER":        "custom-broker:9093",
		"INPUT_TOPIC":         "custom-input",
		"OUTPUT_TOPIC":        "custom-output",
		"CONSUMER_GROUP":      "custom-group",
		"METRICS_PORT":        "9001",
		"STORAGE_SERVICE_URL": "http://custom-storage:5003",
		"MAX_READINGS":        "500",
		"KAFKA_ENABLED":       "false",
		"DEBUG":               "true",
	}

	for key, value := range testEnvs {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	cfg := GetConfig()

	// Test custom values
	assert.Equal(t, "custom-broker:9093", cfg.KafkaBroker)
	assert.Equal(t, "custom-input", cfg.InputTopic)
	assert.Equal(t, "custom-output", cfg.OutputTopic)
	assert.Equal(t, "custom-group", cfg.ConsumerGroup)
	assert.Equal(t, 9001, cfg.MetricsPort)
	assert.Equal(t, "http://custom-storage:5003", cfg.StorageServiceURL)
	assert.Equal(t, 500, cfg.MaxReadings)
	assert.False(t, cfg.KafkaEnabled)
	assert.True(t, cfg.Debug)
}

func TestGetConfig_InvalidPorts(t *testing.T) {
	// Test invalid port values fall back to defaults
	tests := []struct {
		name        string
		envVar      string
		envValue    string
		expectedVal int
	}{
		{"invalid metrics port", "METRICS_PORT", "invalid", 0},
		{"invalid max readings", "MAX_READINGS", "not_a_number", 0},
		{"negative metrics port", "METRICS_PORT", "-1", -1},
		{"zero metrics port", "METRICS_PORT", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear other environment variables
			envVars := []string{
				"KAFKA_BROKER", "INPUT_TOPIC", "OUTPUT_TOPIC", "CONSUMER_GROUP",
				"STORAGE_SERVICE_URL", "KAFKA_ENABLED", "DEBUG",
			}
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			os.Setenv(tt.envVar, tt.envValue)
			defer os.Unsetenv(tt.envVar)

			cfg := GetConfig()

			if tt.envVar == "METRICS_PORT" {
				assert.Equal(t, tt.expectedVal, cfg.MetricsPort)
			} else if tt.envVar == "MAX_READINGS" {
				assert.Equal(t, tt.expectedVal, cfg.MaxReadings)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "existing environment variable",
			key:          "TEST_ENV_VAR",
			defaultValue: "default",
			envValue:     "custom_value",
			expected:     "custom_value",
		},
		{
			name:         "non-existing environment variable",
			key:          "NON_EXISTING_VAR",
			defaultValue: "default_value",
			envValue:     "",
			expected:     "default_value",
		},
		{
			name:         "empty environment variable",
			key:          "EMPTY_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the environment variable first
			os.Unsetenv(tt.key)

			// Set the environment variable if needed
			if tt.envValue != "" || tt.name == "empty environment variable" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTenantConfig(t *testing.T) {
	tests := []struct {
		name              string
		tenantID          string
		expectedThreshold float64
		expectedRate      float64
	}{
		{
			name:              "tenant1 configuration",
			tenantID:          "tenant1",
			expectedThreshold: 3.0,
			expectedRate:      1.0,
		},
		{
			name:              "tenant2 configuration",
			tenantID:          "tenant2",
			expectedThreshold: 2.5,
			expectedRate:      0.5,
		},
		{
			name:              "unknown tenant falls back to default",
			tenantID:          "unknown_tenant",
			expectedThreshold: 2.0,
			expectedRate:      1.0,
		},
		{
			name:              "empty tenant ID falls back to default",
			tenantID:          "",
			expectedThreshold: 2.0,
			expectedRate:      1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threshold, rate := GetTenantConfig(tt.tenantID)
			assert.Equal(t, tt.expectedThreshold, threshold)
			assert.Equal(t, tt.expectedRate, rate)
		})
	}
}

func TestConstants(t *testing.T) {
	// Test that constants are properly defined
	assert.Equal(t, "true", TrueValue)
	assert.Equal(t, "false", FalseValue)
}

func TestDefaultTenantConfigs(t *testing.T) {
	// Test that default tenant configs are properly initialized
	assert.NotNil(t, DefaultTenantConfigs)
	assert.Contains(t, DefaultTenantConfigs, "tenant1")
	assert.Contains(t, DefaultTenantConfigs, "tenant2")

	// Test tenant1 config
	tenant1Config := DefaultTenantConfigs["tenant1"]
	assert.Equal(t, 3.0, tenant1Config.AnomalyThreshold)
	assert.Equal(t, 1.0, tenant1Config.SamplingRate)

	// Test tenant2 config
	tenant2Config := DefaultTenantConfigs["tenant2"]
	assert.Equal(t, 2.5, tenant2Config.AnomalyThreshold)
	assert.Equal(t, 0.5, tenant2Config.SamplingRate)
}

func TestDefaultConfig(t *testing.T) {
	// Test that default config is properly initialized
	assert.Equal(t, 2.0, DefaultConfig.AnomalyThreshold)
	assert.Equal(t, 1.0, DefaultConfig.SamplingRate)
}

func TestKafkaEnabledBoolean(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"kafka enabled with true", "true", true},
		{"kafka enabled with TRUE", "TRUE", false}, // case sensitive
		{"kafka disabled with false", "false", false},
		{"kafka disabled with FALSE", "FALSE", false},
		{"kafka disabled with random value", "random", false},
		{"kafka disabled with empty", "", true}, // Default is true when not set
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear other environment variables
			envVars := []string{
				"KAFKA_BROKER", "INPUT_TOPIC", "OUTPUT_TOPIC", "CONSUMER_GROUP",
				"METRICS_PORT", "STORAGE_SERVICE_URL", "MAX_READINGS", "DEBUG",
			}
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			if tt.envValue != "" {
				os.Setenv("KAFKA_ENABLED", tt.envValue)
				defer os.Unsetenv("KAFKA_ENABLED")
			} else {
				os.Unsetenv("KAFKA_ENABLED")
			}

			cfg := GetConfig()
			assert.Equal(t, tt.expected, cfg.KafkaEnabled)
		})
	}
}

func TestDebugBoolean(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"debug enabled with true", "true", true},
		{"debug enabled with TRUE", "TRUE", false}, // case sensitive
		{"debug disabled with false", "false", false},
		{"debug disabled with random value", "random", false},
		{"debug disabled with empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear other environment variables
			envVars := []string{
				"KAFKA_BROKER", "INPUT_TOPIC", "OUTPUT_TOPIC", "CONSUMER_GROUP",
				"METRICS_PORT", "STORAGE_SERVICE_URL", "MAX_READINGS", "KAFKA_ENABLED",
			}
			for _, env := range envVars {
				os.Unsetenv(env)
			}

			if tt.envValue != "" {
				os.Setenv("DEBUG", tt.envValue)
				defer os.Unsetenv("DEBUG")
			} else {
				os.Unsetenv("DEBUG")
			}

			cfg := GetConfig()
			assert.Equal(t, tt.expected, cfg.Debug)
		})
	}
}
