package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// loadApiKeys loads API keys from environment variables and returns them as a map.
func loadApiKeys() map[string]bool {
	validKeys := make(map[string]bool)
	apiKey1 := os.Getenv("API_KEY_1")
	apiKey2 := os.Getenv("API_KEY_2")

	if apiKey1 != "" {
		validKeys[apiKey1] = true
		log.Println("Loaded API_KEY_1")
	} else {
		log.Println("API_KEY_1 not set")
	}
	if apiKey2 != "" {
		validKeys[apiKey2] = true
		log.Println("Loaded API_KEY_2")
	} else {
		log.Println("API_KEY_2 not set")
	}

	if len(validKeys) == 0 {
		log.Println("Warning: No valid API keys loaded!")
	}
	return validKeys
}

// apiKeyAuthMiddleware creates a Gin middleware handler that checks
// for a valid X-API-Key header against the provided map of valid keys.
func apiKeyAuthMiddleware(validKeys map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing API key"})
			return
		}

		// Check against the provided map
		if _, ok := validKeys[apiKey]; !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			return
		}

		// Key is valid, proceed to the next handler
		c.Next()
	}
}
