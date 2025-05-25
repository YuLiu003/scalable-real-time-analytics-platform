package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var validAPIKeys []string

// InitAPIKeys initializes the valid API keys
func InitAPIKeys() {
	apiKey1 := os.Getenv("API_KEY_1")
	apiKey2 := os.Getenv("API_KEY_2")

	// Default keys if environment variables are not set
	if apiKey1 == "" {
		apiKey1 = "test-api-key-123456"
	}
	if apiKey2 == "" {
		apiKey2 = "test-api-key-789012"
	}

	// Create the validAPIKeys slice
	validAPIKeys = []string{apiKey1, apiKey2}

	// Also check for comma-separated API_KEYS variable
	if apiKeys := os.Getenv("API_KEYS"); apiKeys != "" {
		keys := strings.Split(apiKeys, ",")
		for _, key := range keys {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey != "" && !contains(validAPIKeys, trimmedKey) {
				validAPIKeys = append(validAPIKeys, trimmedKey)
			}
		}
	}
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RequireAPIKey is a middleware that checks for a valid API key
func RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if auth is bypassed
		bypassAuth := os.Getenv("BYPASS_AUTH")
		if bypassAuth == "true" {
			c.Next()
			return
		}

		// Check for API key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Missing API key",
			})
			c.Abort()
			return
		}

		// Validate API key
		valid := false
		for _, key := range validAPIKeys {
			if apiKey == key {
				valid = true
				break
			}
		}

		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid API key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
