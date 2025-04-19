package main

// SensorData defines the structure for incoming sensor readings.
// We use pointers for optional fields like temperature and humidity.
// Using json:"-" for TenantID initially as it's added server-side.
// Use map[string]interface{} to capture any other dynamic fields.
type SensorData map[string]interface{}

/* // Alternative struct-based approach if schema is more fixed:
type SensorData struct {
	DeviceID    string  `json:"device_id" binding:"required"`
	Temperature *float64 `json:"temperature,omitempty"`
	Humidity    *float64 `json:"humidity,omitempty"`
	TenantID    string  `json:"tenant_id,omitempty"` // Added server-side
	// Add other expected fields here
	Timestamp   *string `json:"timestamp,omitempty"`
}
*/
