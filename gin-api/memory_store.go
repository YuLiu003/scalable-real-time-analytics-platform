package main

import (
	"log"
	"sync"
)

// MemoryStore defines the structure for the in-memory store.
type MemoryStore struct {
	store map[string][]SensorData
	mutex *sync.RWMutex
}

// NewMemoryStore creates and initializes a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string][]SensorData),
		mutex: &sync.RWMutex{},
	}
}

const defaultTenantKey = "_no_tenant_"

// storeData adds a SensorData record to the in-memory store for the appropriate tenant.
// It is thread-safe.
func (ms *MemoryStore) storeData(data SensorData) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	tenantID := defaultTenantKey // Default if isolation is off or tenant_id is missing
	if id, ok := data["tenant_id"]; ok {
		if tenantIDStr, ok := id.(string); ok && tenantIDStr != "" {
			tenantID = tenantIDStr
		}
	}

	ms.store[tenantID] = append(ms.store[tenantID], data)
	log.Printf("Stored data in memory for tenant '%s'. Count: %d", tenantID, len(ms.store[tenantID]))
}

// getDataByDevice retrieves all records for a specific device ID, respecting tenant isolation.
// It is thread-safe.
func (ms *MemoryStore) getDataByDevice(requestingTenantID string, deviceID string, enableIsolation bool) []SensorData {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	var results []SensorData

	if enableIsolation {
		// If isolation is on, only look within the requesting tenant's data
		for _, record := range ms.store[requestingTenantID] {
			if recordDeviceID, ok := record["device_id"].(string); ok && recordDeviceID == deviceID {
				results = append(results, record)
			}
		}
		log.Printf("Retrieved %d records for device '%s' for tenant '%s' from memory", len(results), deviceID, requestingTenantID)
	} else {
		// If isolation is off, check all tenants' data
		for tenantID, records := range ms.store {
			for _, record := range records {
				if recordDeviceID, ok := record["device_id"].(string); ok && recordDeviceID == deviceID {
					results = append(results, record)
				}
			}
			log.Printf("Retrieved %d records for device '%s' (tenant isolation disabled) from memory (checked tenant %s)", len(results), deviceID, tenantID)
		}
	}

	return results
}

// getAllData retrieves all data currently in the memory store.
// Used for the debug endpoint. It is thread-safe.
func (ms *MemoryStore) getAllData() map[string][]SensorData {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	// Create a copy to avoid race conditions if the caller modifies the map
	dataCopy := make(map[string][]SensorData, len(ms.store))
	for key, value := range ms.store {
		// Also copy the slice to prevent modification issues
		dataCopy[key] = append([]SensorData(nil), value...)
	}
	log.Printf("Retrieved all (%d tenants) data from memory for debug endpoint", len(dataCopy))
	return dataCopy
}

// clearAllData removes all data from the memory store.
// Primarily intended for use in tests. It is thread-safe.
func (ms *MemoryStore) clearAllData() {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.store = make(map[string][]SensorData)
	log.Println("Cleared all data from memory store.")
}

// Helper function to get data count for a specific tenant (useful for tests)
func (ms *MemoryStore) getTenantDataCount(tenantID string) int {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	return len(ms.store[tenantID])
}
