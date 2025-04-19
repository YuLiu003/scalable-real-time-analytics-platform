package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

// loadConfig loads configuration from environment variables.
// It returns the tenant isolation flag and the tenant-API key map.
func loadConfig() (bool, map[string]string) {
	// Load tenant isolation setting
	enableTenantIsolationStr := os.Getenv("ENABLE_TENANT_ISOLATION")
	enableTenantIsolation := strings.ToLower(enableTenantIsolationStr) == "true"
	log.Printf("Tenant Isolation Enabled: %t", enableTenantIsolation)

	// Load tenant API key map
	tenantApiKeyMap := make(map[string]string) // Initialize local map
	tenantMapStr := os.Getenv("TENANT_API_KEY_MAP")
	if tenantMapStr == "" {
		log.Println("Warning: TENANT_API_KEY_MAP not set. Using empty map.")
		// Keep map empty
	} else {
		err := json.Unmarshal([]byte(tenantMapStr), &tenantApiKeyMap)
		if err != nil {
			log.Printf("Warning: Could not parse TENANT_API_KEY_MAP: %v. Using empty map.", err)
			tenantApiKeyMap = make(map[string]string) // Reset to empty on error
		} else {
			log.Printf("Loaded tenant map: %v", tenantApiKeyMap)
		}
	}
	return enableTenantIsolation, tenantApiKeyMap
}

// getTenantIDForKey looks up the tenant ID for a given API key within the provided map.
// Returns the tenant ID and a boolean indicating if it was found.
func getTenantIDForKey(apiKey string, tenantMap map[string]string) (string, bool) {
	if tenantMap == nil {
		return "", false
	}
	tenantID, ok := tenantMap[apiKey]
	return tenantID, ok
}
