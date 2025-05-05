package processor

import (
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/models"
	"github.com/stretchr/testify/assert"
)

func TestNewProcessor(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false, // Disable Kafka for tests
		MaxReadings:  10,    // Use a small value for tests
	}

	p, err := NewProcessor(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, cfg, p.cfg)
	assert.NotNil(t, p.processedData.Devices)
}

func TestProcessData(t *testing.T) {
	cfg := &config.Config{
		KafkaEnabled: false,
		MaxReadings:  10,
	}

	p, _ := NewProcessor(cfg)

	// Create test data
	now := time.Now()
	temp := 25.5
	humidity := 60.0
	data := models.SensorData{
		DeviceID:    "test-device",
		Temperature: &temp,
		Humidity:    &humidity,
		Timestamp:   &now,
		TenantID:    "tenant1",
	}

	// Process the data
	result, err := p.processData(data)

	// Check the result
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-device", result.DeviceID)
	assert.Equal(t, &temp, result.Temperature)
	assert.Equal(t, &humidity, result.Humidity)
	assert.Equal(t, "tenant1", result.TenantID)

	// Check the processor's internal state
	stats, status := p.GetStats()
	assert.Equal(t, 1, stats.Temperature.Count)
	assert.Equal(t, 25.5, stats.Temperature.Sum)
	assert.Equal(t, 25.5, stats.Temperature.Min)
	assert.Equal(t, 25.5, stats.Temperature.Max)
	assert.Equal(t, 25.5, stats.Temperature.Avg)

	assert.Equal(t, 1, stats.Humidity.Count)
	assert.Equal(t, 60.0, stats.Humidity.Sum)
	assert.Equal(t, 60.0, stats.Humidity.Min)
	assert.Equal(t, 60.0, stats.Humidity.Max)
	assert.Equal(t, 60.0, stats.Humidity.Avg)

	assert.Len(t, stats.Devices, 1)
	assert.Contains(t, stats.Devices, "test-device")

	device := stats.Devices["test-device"]
	assert.Equal(t, 25.5, device.Temperature.Avg)
	assert.Equal(t, 60.0, device.Humidity.Avg)

	// Check status
	assert.False(t, status.Running) // Should be false since we disabled Kafka
	assert.Equal(t, 0, status.MessagesProcessed)
}

// ProcessTestData is a method specifically for testing that processes a single piece of data
func (p *Processor) ProcessTestData(data models.SensorData) (*models.ProcessedDataPayload, error) {
	return p.processData(data)
}
