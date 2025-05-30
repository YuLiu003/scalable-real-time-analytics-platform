package processor

import (
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessor(t *testing.T) {
	cfg := &config.Config{
		KafkaBroker:       "localhost:9092",
		InputTopic:        "test-input",
		OutputTopic:       "test-output",
		ConsumerGroup:     "test-group",
		MetricsPort:       8000,
		StorageServiceURL: "http://localhost:5002",
		MaxReadings:       100,
		KafkaEnabled:      false, // Disable Kafka for tests
		Debug:             true,
	}

	processor, err := NewProcessor(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, processor)
	assert.Equal(t, cfg, processor.cfg)
	assert.NotNil(t, processor.processedData.Devices)
	assert.False(t, processor.status.Running)

	// Check that initial min/max values are set to extremes
	assert.Equal(t, float64(999999), processor.processedData.Temperature.Min)
	assert.Equal(t, float64(-999999), processor.processedData.Temperature.Max)
	assert.Equal(t, float64(999999), processor.processedData.Humidity.Min)
	assert.Equal(t, float64(-999999), processor.processedData.Humidity.Max)
}

func TestProcessor_StartWithKafkaDisabled(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Starting with Kafka disabled should not return an error
	err = processor.Start()
	assert.NoError(t, err)
}

func TestProcessor_GetStats(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Test initial stats
	stats, status := processor.GetStats()
	assert.NotNil(t, stats)
	assert.NotNil(t, status)
	assert.Equal(t, 0, stats.Temperature.Count)
	assert.Equal(t, 0, stats.Humidity.Count)
	assert.Empty(t, stats.Devices)
	assert.False(t, status.Running)
}

func TestProcessor_ProcessData(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Test processing valid sensor data
	temperature := 25.5
	humidity := 60.0
	timestamp := time.Now()

	sensorData := models.SensorData{
		DeviceID:    "test-device-001",
		TenantID:    "tenant1",
		Temperature: &temperature,
		Humidity:    &humidity,
		Timestamp:   &timestamp,
	}

	result, err := processor.ProcessData(sensorData)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-device-001", result.DeviceID)
	assert.Equal(t, "tenant1", result.TenantID)
	assert.Equal(t, &temperature, result.Temperature)
	assert.Equal(t, &humidity, result.Humidity)
	assert.NotEmpty(t, result.ProcessedAt)
}

func TestProcessor_ProcessDataValidation(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Test processing sensor data without device ID (should fail)
	temperature := 25.5
	humidity := 60.0
	timestamp := time.Now()

	sensorData := models.SensorData{
		DeviceID:    "", // Empty device ID should cause validation error
		TenantID:    "tenant1",
		Temperature: &temperature,
		Humidity:    &humidity,
		Timestamp:   &timestamp,
	}

	result, err := processor.ProcessData(sensorData)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "device_id is required")
}

func TestProcessor_UpdateProcessingStatistics(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Process some test data
	temperature := 25.5
	humidity := 60.0
	timestamp := time.Now()

	sensorData := models.SensorData{
		DeviceID:    "test-device",
		TenantID:    "tenant1",
		Temperature: &temperature,
		Humidity:    &humidity,
		Timestamp:   &timestamp,
	}

	// Process the data to update statistics
	_, err = processor.ProcessData(sensorData)
	require.NoError(t, err)

	// Verify statistics were updated
	stats, _ := processor.GetStats()

	// Check global temperature stats
	assert.Equal(t, 1, stats.Temperature.Count)
	assert.Equal(t, 25.5, stats.Temperature.Sum)
	assert.Equal(t, 25.5, stats.Temperature.Min)
	assert.Equal(t, 25.5, stats.Temperature.Max)
	assert.Equal(t, 25.5, stats.Temperature.Avg)

	// Check global humidity stats
	assert.Equal(t, 1, stats.Humidity.Count)
	assert.Equal(t, 60.0, stats.Humidity.Sum)
	assert.Equal(t, 60.0, stats.Humidity.Min)
	assert.Equal(t, 60.0, stats.Humidity.Max)
	assert.Equal(t, 60.0, stats.Humidity.Avg)

	// Check device stats
	assert.Contains(t, stats.Devices, "test-device")
	deviceStats := stats.Devices["test-device"]
	assert.NotNil(t, deviceStats)
	assert.Contains(t, deviceStats.Temperature.Readings, 25.5)
	assert.Contains(t, deviceStats.Humidity.Readings, 60.0)
}

func TestProcessor_MultipleDataPoints(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Process multiple data points
	temperatures := []float64{20.0, 25.0, 30.0}
	humidities := []float64{50.0, 60.0, 70.0}

	for i := 0; i < 3; i++ {
		timestamp := time.Now()
		sensorData := models.SensorData{
			DeviceID:    "test-device",
			TenantID:    "tenant1",
			Temperature: &temperatures[i],
			Humidity:    &humidities[i],
			Timestamp:   &timestamp,
		}

		_, err = processor.ProcessData(sensorData)
		require.NoError(t, err)
	}

	// Verify aggregated statistics
	stats, _ := processor.GetStats()

	// Check global temperature stats
	assert.Equal(t, 3, stats.Temperature.Count)
	assert.Equal(t, 75.0, stats.Temperature.Sum) // 20 + 25 + 30
	assert.Equal(t, 20.0, stats.Temperature.Min)
	assert.Equal(t, 30.0, stats.Temperature.Max)
	assert.Equal(t, 25.0, stats.Temperature.Avg) // 75/3

	// Check global humidity stats
	assert.Equal(t, 3, stats.Humidity.Count)
	assert.Equal(t, 180.0, stats.Humidity.Sum) // 50 + 60 + 70
	assert.Equal(t, 50.0, stats.Humidity.Min)
	assert.Equal(t, 70.0, stats.Humidity.Max)
	assert.Equal(t, 60.0, stats.Humidity.Avg) // 180/3
}

func TestProcessor_TenantConfiguration(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Test with tenant1 configuration (anomaly threshold = 3.0)
	temperature := 35.0 // High temperature to trigger anomaly
	humidity := 80.0    // High humidity to trigger anomaly
	timestamp := time.Now()

	sensorData := models.SensorData{
		DeviceID:    "test-device",
		TenantID:    "tenant1",
		Temperature: &temperature,
		Humidity:    &humidity,
		Timestamp:   &timestamp,
	}

	result, err := processor.ProcessData(sensorData)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tenant1", result.TenantID)
	// The result should contain anomaly information
	assert.GreaterOrEqual(t, result.AnomalyScore, 0.0)
}

func TestProcessor_Stop(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Should be able to stop processor without issues
	processor.Stop()

	// Verify we can still get stats after stopping
	stats, _ := processor.GetStats()
	assert.NotNil(t, stats)
}

func TestProcessor_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		Debug:        true,
	}

	processor, err := NewProcessor(cfg)
	require.NoError(t, err)

	// Test concurrent read/write access to verify thread safety
	done := make(chan bool)
	numGoroutines := 5

	// Start multiple goroutines that update stats
	for i := 0; i < numGoroutines; i++ {
		go func(deviceNum int) {
			defer func() { done <- true }()

			temperature := 20.0 + float64(deviceNum)
			humidity := 50.0 + float64(deviceNum)
			timestamp := time.Now()

			sensorData := models.SensorData{
				DeviceID:    "concurrent-device",
				TenantID:    "tenant1",
				Temperature: &temperature,
				Humidity:    &humidity,
				Timestamp:   &timestamp,
			}

			processor.ProcessData(sensorData)
		}(i)
	}

	// Start goroutines that read stats
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			// Read global stats
			processor.GetStats()
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines*2; i++ {
		<-done
	}

	// Verify final state (should have processed data from concurrent operations)
	stats, _ := processor.GetStats()
	assert.GreaterOrEqual(t, stats.Temperature.Count, 1)
	assert.GreaterOrEqual(t, stats.Humidity.Count, 1)
}
