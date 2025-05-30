package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Test default configuration
	t.Run("DefaultConfiguration", func(t *testing.T) {
		config := Load()

		assert.Equal(t, 5003, config.Port)
		assert.Equal(t, "http://data-ingestion-service", config.DataServiceURL)
		assert.Equal(t, 100, config.MaxDataPoints)
		assert.Equal(t, false, config.DebugMode)
	})

	// Test custom configuration
	t.Run("CustomConfiguration", func(t *testing.T) {
		// Set environment variables
		os.Setenv("PORT", "9090")
		os.Setenv("DATA_SERVICE_URL", "http://custom-data-service")
		os.Setenv("MAX_DATA_POINTS", "500")
		os.Setenv("DEBUG_MODE", "true")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("DATA_SERVICE_URL")
			os.Unsetenv("MAX_DATA_POINTS")
			os.Unsetenv("DEBUG_MODE")
		}()

		config := Load()

		assert.Equal(t, 9090, config.Port)
		assert.Equal(t, "http://custom-data-service", config.DataServiceURL)
		assert.Equal(t, 500, config.MaxDataPoints)
		assert.Equal(t, true, config.DebugMode)
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("ExistingVar", func(t *testing.T) {
		os.Setenv("TEST_VAR", "test_value")
		defer os.Unsetenv("TEST_VAR")

		result := getEnv("TEST_VAR", "default")
		assert.Equal(t, "test_value", result)
	})

	t.Run("NonExistingVar", func(t *testing.T) {
		result := getEnv("NON_EXISTING_VAR", "default")
		assert.Equal(t, "default", result)
	})

	t.Run("EmptyVar", func(t *testing.T) {
		os.Setenv("EMPTY_VAR", "")
		defer os.Unsetenv("EMPTY_VAR")

		result := getEnv("EMPTY_VAR", "default")
		assert.Equal(t, "default", result)
	})
}

func TestGetEnvAsInt(t *testing.T) {
	t.Run("ValidInt", func(t *testing.T) {
		os.Setenv("INT_VAR", "123")
		defer os.Unsetenv("INT_VAR")

		result := getEnvAsInt("INT_VAR", 456)
		assert.Equal(t, 123, result)
	})

	t.Run("InvalidInt", func(t *testing.T) {
		os.Setenv("INVALID_INT", "not_a_number")
		defer os.Unsetenv("INVALID_INT")

		result := getEnvAsInt("INVALID_INT", 456)
		assert.Equal(t, 456, result)
	})

	t.Run("NonExistingInt", func(t *testing.T) {
		result := getEnvAsInt("NON_EXISTING_INT", 456)
		assert.Equal(t, 456, result)
	})
}

func TestGetEnvAsBool(t *testing.T) {
	t.Run("TrueValues", func(t *testing.T) {
		trueValues := []string{"true", "TRUE", "1", "t", "T"}
		for _, val := range trueValues {
			os.Setenv("BOOL_VAR", val)
			result := getEnvAsBool("BOOL_VAR", false)
			assert.True(t, result, "Expected true for value: %s", val)
		}
		os.Unsetenv("BOOL_VAR")
	})

	t.Run("FalseValues", func(t *testing.T) {
		falseValues := []string{"false", "FALSE", "0", "f", "F"}
		for _, val := range falseValues {
			os.Setenv("BOOL_VAR", val)
			result := getEnvAsBool("BOOL_VAR", true)
			assert.False(t, result, "Expected false for value: %s", val)
		}
		os.Unsetenv("BOOL_VAR")
	})

	t.Run("InvalidValues", func(t *testing.T) {
		// Invalid values should return the default
		os.Setenv("BOOL_VAR", "random")
		result := getEnvAsBool("BOOL_VAR", true)
		assert.True(t, result, "Expected default true for invalid value")

		result2 := getEnvAsBool("BOOL_VAR", false)
		assert.False(t, result2, "Expected default false for invalid value")
		os.Unsetenv("BOOL_VAR")
	})

	t.Run("NonExistingBool", func(t *testing.T) {
		result := getEnvAsBool("NON_EXISTING_BOOL", true)
		assert.True(t, result)
	})
}
