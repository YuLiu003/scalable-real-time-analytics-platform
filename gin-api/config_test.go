package main

import (
	"os"
	"reflect" // Import reflect for DeepEqual
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Combined test for both isolation flag and map
	tests := []struct {
		name              string
		isolationEnvValue string
		mapEnvValue       string
		expectedIsolation bool
		expectedMap       map[string]string
	}{
		{
			name:              "EnabledAndValidMap",
			isolationEnvValue: "true",
			mapEnvValue:       `{"key1":"tA","key2":"tB"}`,
			expectedIsolation: true,
			expectedMap:       map[string]string{"key1": "tA", "key2": "tB"},
		},
		{
			name:              "DisabledAndEmptyMap",
			isolationEnvValue: "false",
			mapEnvValue:       `{}`, // Empty JSON map
			expectedIsolation: false,
			expectedMap:       map[string]string{},
		},
		{
			name:              "DefaultIsolationAndUnsetMap",
			isolationEnvValue: "", // Default false
			mapEnvValue:       "", // Unset env var -> empty map
			expectedIsolation: false,
			expectedMap:       map[string]string{},
		},
		{
			name:              "EnabledAndInvalidMap",
			isolationEnvValue: "TRUE",
			mapEnvValue:       `{invalid`, // Invalid JSON -> empty map
			expectedIsolation: true,
			expectedMap:       map[string]string{},
		},
	}

	originalIsolation := os.Getenv("ENABLE_TENANT_ISOLATION")
	originalMapEnv := os.Getenv("TENANT_API_KEY_MAP")
	defer func() {
		os.Setenv("ENABLE_TENANT_ISOLATION", originalIsolation)
		os.Setenv("TENANT_API_KEY_MAP", originalMapEnv)
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars for this subtest
			t.Setenv("ENABLE_TENANT_ISOLATION", tt.isolationEnvValue)
			t.Setenv("TENANT_API_KEY_MAP", tt.mapEnvValue)

			isolationFlag, tenantMap := loadConfig() // Call the refactored function

			if isolationFlag != tt.expectedIsolation {
				t.Errorf("loadConfig() isolation flag mismatch; got %t, want %t", isolationFlag, tt.expectedIsolation)
			}

			// Use reflect.DeepEqual for reliable map comparison
			if !reflect.DeepEqual(tenantMap, tt.expectedMap) {
				t.Errorf("loadConfig() tenant map mismatch; got %v, want %v", tenantMap, tt.expectedMap)
			}
		})
	}
}

func TestGetTenantIDForKey(t *testing.T) {
	// Define the map directly for this test - no global state involved
	testMap := map[string]string{
		"key1": "tenantA",
		"key2": "tenantB",
	}

	tests := []struct {
		name        string
		apiKey      string
		mapToUse    map[string]string // Explicitly pass the map
		expectedID  string
		expectFound bool
	}{
		{"FoundKey1", "key1", testMap, "tenantA", true},
		{"FoundKey2", "key2", testMap, "tenantB", true},
		{"NotFoundKey", "key3", testMap, "", false},
		{"EmptyKey", "", testMap, "", false},
		{"NilMap", "key1", nil, "", false}, // Test with a nil map
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, found := getTenantIDForKey(tt.apiKey, tt.mapToUse) // Pass map argument
			if found != tt.expectFound {
				t.Errorf("getTenantIDForKey(%s) found mismatch; got %t, want %t", tt.apiKey, found, tt.expectFound)
			}
			if id != tt.expectedID {
				t.Errorf("getTenantIDForKey(%s) id mismatch; got '%s', want '%s'", tt.apiKey, id, tt.expectedID)
			}
		})
	}
}
