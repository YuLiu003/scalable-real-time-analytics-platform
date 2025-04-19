package main

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
)

var testStore *MemoryStore // Global store for testing

// Helper function to reset the store and config before each test/subtest
func resetStoreAndConfig() {
	testStore = NewMemoryStore() // Initialize the store
	// Reset relevant config defaults (can be overridden by tests)
	// No global config state needed here anymore
}

func TestStoreData(t *testing.T) {
	resetStoreAndConfig()

	data1 := SensorData{"device_id": "dev1", "temp": 20.5, "tenant_id": "tenantA"}
	data2 := SensorData{"device_id": "dev2", "temp": 21.0, "tenant_id": "tenantB"}
	data3 := SensorData{"device_id": "dev3", "temp": 22.0} // No tenant_id

	testStore.storeData(data1)
	testStore.storeData(data2)
	testStore.storeData(data3)

	// Use getAllData to inspect the store state
	// NOTE: getAllData returns a *copy*, so it's safe to inspect without external lock
	currentStoreState := testStore.getAllData()

	if len(currentStoreState) != 3 {
		t.Errorf("Expected 3 tenants in store (tenantA, tenantB, %s), got %d", defaultTenantKey, len(currentStoreState))
	}
	if len(currentStoreState["tenantA"]) != 1 {
		t.Errorf("Expected 1 record for tenantA, got %d", len(currentStoreState["tenantA"]))
	}
	if len(currentStoreState["tenantB"]) != 1 {
		t.Errorf("Expected 1 record for tenantB, got %d", len(currentStoreState["tenantB"]))
	}
	if len(currentStoreState[defaultTenantKey]) != 1 {
		t.Errorf("Expected 1 record for defaultTenantKey, got %d", len(currentStoreState[defaultTenantKey]))
	}

	if !reflect.DeepEqual(currentStoreState["tenantA"][0], data1) {
		t.Errorf("Data mismatch for tenantA: got %v, want %v", currentStoreState["tenantA"][0], data1)
	}
	if !reflect.DeepEqual(currentStoreState[defaultTenantKey][0], data3) {
		t.Errorf("Data mismatch for defaultTenantKey: got %v, want %v", currentStoreState[defaultTenantKey][0], data3)
	}
}

func TestGetDataByDevice_IsolationDisabled(t *testing.T) {
	resetStoreAndConfig()
	// enableTenantIsolation = false // Setting is passed directly now

	dataA1 := SensorData{"device_id": "dev1", "temp": 20, "tenant_id": "tenantA"}
	dataA2 := SensorData{"device_id": "dev1", "temp": 21, "tenant_id": "tenantA"}
	dataB1 := SensorData{"device_id": "dev1", "temp": 22, "tenant_id": "tenantB"}
	dataB2 := SensorData{"device_id": "dev2", "temp": 23, "tenant_id": "tenantB"}

	testStore.storeData(dataA1)
	testStore.storeData(dataA2)
	testStore.storeData(dataB1)
	testStore.storeData(dataB2)

	results := testStore.getDataByDevice("tenantA", "dev1", false) // Pass isolation flag = false

	if len(results) != 3 {
		t.Fatalf("Expected 3 records for dev1 (isolation off), got %d", len(results))
	}

	// Check if all expected records are present (order doesn't matter)
	expected := []SensorData{dataA1, dataA2, dataB1}
	foundCount := 0
	for _, exp := range expected {
		for _, res := range results {
			if reflect.DeepEqual(exp, res) {
				foundCount++
				break
			}
		}
	}
	if foundCount != len(expected) {
		t.Errorf("Mismatch in retrieved records for dev1 (isolation off). Expected %v, Got %v", expected, results)
	}

	resultsDev2 := testStore.getDataByDevice("tenantB", "dev2", false)
	if len(resultsDev2) != 1 {
		t.Fatalf("Expected 1 record for dev2 (isolation off), got %d", len(resultsDev2))
	}
	if !reflect.DeepEqual(resultsDev2[0], dataB2) {
		t.Errorf("Mismatch for dev2 record: got %v, want %v", resultsDev2[0], dataB2)
	}

	resultsDev3 := testStore.getDataByDevice("tenantA", "dev3", false) // Non-existent device
	if len(resultsDev3) != 0 {
		t.Errorf("Expected 0 records for dev3, got %d", len(resultsDev3))
	}
}

func TestGetDataByDevice_IsolationEnabled(t *testing.T) {
	resetStoreAndConfig()
	// enableTenantIsolation = true // Setting is passed directly now

	dataA1 := SensorData{"device_id": "dev1", "temp": 20, "tenant_id": "tenantA"}
	dataA2 := SensorData{"device_id": "dev1", "temp": 21, "tenant_id": "tenantA"}
	dataB1 := SensorData{"device_id": "dev1", "temp": 22, "tenant_id": "tenantB"} // Same device, different tenant
	dataB2 := SensorData{"device_id": "dev2", "temp": 23, "tenant_id": "tenantB"}

	testStore.storeData(dataA1)
	testStore.storeData(dataA2)
	testStore.storeData(dataB1)
	testStore.storeData(dataB2)

	resultsA := testStore.getDataByDevice("tenantA", "dev1", true) // Pass isolation flag = true
	if len(resultsA) != 2 {
		t.Fatalf("Expected 2 records for dev1 tenantA (isolation on), got %d", len(resultsA))
	}
	expectedA := []SensorData{dataA1, dataA2}
	foundCountA := 0
	for _, exp := range expectedA {
		for _, res := range resultsA {
			if reflect.DeepEqual(exp, res) {
				foundCountA++
				break
			}
		}
	}
	if foundCountA != len(expectedA) {
		t.Errorf("Mismatch in retrieved records for dev1 tenantA (isolation on). Expected %v, Got %v", expectedA, resultsA)
	}

	resultsB := testStore.getDataByDevice("tenantB", "dev1", true)
	if len(resultsB) != 1 {
		t.Fatalf("Expected 1 record for dev1 tenantB (isolation on), got %d", len(resultsB))
	}
	if !reflect.DeepEqual(resultsB[0], dataB1) {
		t.Errorf("Mismatch for dev1 tenantB record: got %v, want %v", resultsB[0], dataB1)
	}

	resultsC := testStore.getDataByDevice("tenantC", "dev1", true) // Tenant with no data
	if len(resultsC) != 0 {
		t.Errorf("Expected 0 records for dev1 tenantC, got %d", len(resultsC))
	}
}

func TestGetAllData(t *testing.T) {
	resetStoreAndConfig()

	dataA1 := SensorData{"device_id": "dev1", "tenant_id": "tenantA"}
	dataB1 := SensorData{"device_id": "dev2", "tenant_id": "tenantB"}
	dataD1 := SensorData{"device_id": "dev3"}

	testStore.storeData(dataA1)
	testStore.storeData(dataB1)
	testStore.storeData(dataD1)

	allData := testStore.getAllData()

	if len(allData) != 3 {
		t.Fatalf("Expected 3 tenants in getAllData result, got %d", len(allData))
	}
	if _, ok := allData["tenantA"]; !ok {
		t.Error("Missing tenantA in getAllData result")
	}
	if _, ok := allData["tenantB"]; !ok {
		t.Error("Missing tenantB in getAllData result")
	}
	if _, ok := allData[defaultTenantKey]; !ok {
		t.Error("Missing defaultTenantKey in getAllData result")
	}

	if len(allData["tenantA"]) != 1 || !reflect.DeepEqual(allData["tenantA"][0], dataA1) {
		t.Errorf("Data mismatch for tenantA in getAllData: got %v", allData["tenantA"])
	}
	// Check immutability: modify returned map and ensure original is unchanged
	allData["tenantA"][0]["device_id"] = "modified"

	// Get the original data again to check if it was modified
	// We need to re-fetch using a method that reads the *current* state
	// Since getAllData returns a copy, we can't just look at the internal map directly.
	// Let's fetch data for tenantA, dev1 specifically to check.
	// Assume isolation is off for this check as tenant/device is specific.
	originalData := testStore.getDataByDevice("tenantA", "dev1", false)
	if len(originalData) == 1 && originalData[0]["device_id"] == "modified" {
		t.Error("getAllData did not return a deep copy, original store was modified")
	}
}

// Basic test for concurrency safety (not exhaustive)
func TestStoreData_Concurrent(t *testing.T) {
	resetStoreAndConfig()

	numGoroutines := 50
	numWritesPerRoutine := 20
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < numWritesPerRoutine; j++ {
				tenant := "tenant" + strconv.Itoa(routineID%5) // Cycle through 5 tenants
				data := SensorData{
					"device_id": "dev" + strconv.Itoa(routineID*100+j),
					"tenant_id": tenant,
					"value":     j,
				}
				testStore.storeData(data) // Use the method on the store instance
			}
		}(i)
	}

	wg.Wait()

	// Check total count using getAllData
	currentStoreState := testStore.getAllData()
	totalStored := 0
	for _, records := range currentStoreState {
		totalStored += len(records)
	}

	expectedTotal := numGoroutines * numWritesPerRoutine
	if totalStored != expectedTotal {
		t.Errorf("Concurrent storeData: expected %d total records, got %d", expectedTotal, totalStored)
	}
}
