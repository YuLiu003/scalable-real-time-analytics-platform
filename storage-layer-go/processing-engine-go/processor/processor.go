package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/models"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics
var (
	messagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "process_messages_total",
			Help: "The total number of processed messages",
		},
		[]string{"status"},
	)

	processingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "processing_latency_seconds",
			Help:    "Time spent processing messages",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	tenantMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tenant_messages_processed",
			Help: "Messages processed by tenant",
		},
		[]string{"tenant_id"},
	)

	tenantProcessingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tenant_processing_latency_seconds",
			Help:    "Processing latency by tenant",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tenant_id"},
	)
)

// Processor handles the processing of data from Kafka
type Processor struct {
	cfg           *config.Config
	consumer      *kafka.Consumer
	producer      *kafka.Producer
	processedData models.ProcessedStats
	status        models.ProcessingStatus
	mu            sync.RWMutex // Mutex for thread-safe access to shared data
}

// NewProcessor creates a new processor instance
func NewProcessor(cfg *config.Config) (*Processor, error) {
	p := &Processor{
		cfg: cfg,
		processedData: models.ProcessedStats{
			Devices: make(map[string]*models.DeviceStats),
		},
		status: models.ProcessingStatus{
			Running: false,
		},
	}

	// Initialize temperature min/max values
	p.processedData.Temperature.Min = math.Inf(1)
	p.processedData.Temperature.Max = math.Inf(-1)
	p.processedData.Humidity.Min = math.Inf(1)
	p.processedData.Humidity.Max = math.Inf(-1)

	return p, nil
}

// Start begins processing messages from Kafka
func (p *Processor) Start() error {
	if !p.cfg.KafkaEnabled {
		log.Println("Kafka is disabled. Processor not started.")
		return nil
	}

	// Set up Kafka consumer
	var err error
	p.consumer, err = kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  p.cfg.KafkaBroker,
		"group.id":           p.cfg.ConsumerGroup,
		"auto.offset.reset":  "latest",
		"enable.auto.commit": true,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %v", err)
	}

	// Set up Kafka producer
	p.producer, err = kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": p.cfg.KafkaBroker,
	})
	if err != nil {
		return fmt.Errorf("failed to create producer: %v", err)
	}

	// Subscribe to the input topic
	err = p.consumer.SubscribeTopics([]string{p.cfg.InputTopic}, nil)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %v", err)
	}

	p.mu.Lock()
	p.status.Running = true
	p.mu.Unlock()

	// Start processing messages in a goroutine
	go p.processMessages()

	log.Printf("Processing engine started. Consumer group: %s, Input topic: %s, Output topic: %s",
		p.cfg.ConsumerGroup, p.cfg.InputTopic, p.cfg.OutputTopic)

	return nil
}

// Stop stops the processor
func (p *Processor) Stop() {
	p.mu.Lock()
	p.status.Running = false
	p.mu.Unlock()

	if p.consumer != nil {
		p.consumer.Close()
	}
	if p.producer != nil {
		p.producer.Flush(1000)
		p.producer.Close()
	}
}

// processMessages continuously processes messages from Kafka
func (p *Processor) processMessages() {
	for p.status.Running {
		msg, err := p.consumer.ReadMessage(time.Second)
		if err != nil {
			// Timeout or error
			if err.(kafka.Error).Code() != kafka.ErrTimedOut {
				log.Printf("Consumer error: %v", err)
				p.mu.Lock()
				p.status.Errors++
				p.mu.Unlock()
			}
			continue
		}

		// Process the message
		var rawData models.SensorData
		if err := json.Unmarshal(msg.Value, &rawData); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			messagesProcessed.WithLabelValues("error").Inc()
			p.mu.Lock()
			p.status.Errors++
			p.mu.Unlock()
			continue
		}

		result, err := p.processData(rawData)
		if err != nil {
			log.Printf("Error processing data: %v", err)
			messagesProcessed.WithLabelValues("error").Inc()
			p.mu.Lock()
			p.status.Errors++
			p.mu.Unlock()
			continue
		}

		// Produce the result to the output topic
		resultBytes, err := json.Marshal(result)
		if err != nil {
			log.Printf("Error marshaling result: %v", err)
			continue
		}

		p.producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &p.cfg.OutputTopic,
				Partition: kafka.PartitionAny,
			},
			Value: resultBytes,
		}, nil)

		// Update status
		p.mu.Lock()
		p.status.MessagesProcessed++
		p.status.LastProcessed = time.Now().Format(time.RFC3339)
		p.mu.Unlock()
	}
}

// processData processes a single piece of data
func (p *Processor) processData(data models.SensorData) (*models.ProcessedDataPayload, error) {
	startTime := time.Now()

	// Get the tenant ID or use a default
	tenantID := data.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	// Get tenant-specific configuration
	anomalyThreshold, samplingRate := config.GetTenantConfig(tenantID)

	// Apply sampling rate to skip some messages for lower-tier tenants
	if samplingRate < 1.0 && rand.Float64() > samplingRate {
		// Skip processing
		return nil, fmt.Errorf("skipped due to sampling rate")
	}

	// Calculate anomaly score
	anomalyScore := 0.0

	// Check if temperature is outside normal range
	if data.Temperature > 30 || data.Temperature < 10 {
		anomalyScore += 1.0
	}

	// Check if humidity is outside normal range
	if data.Humidity > 80 || data.Humidity < 20 {
		anomalyScore += 1.0
	}

	// Determine if anomaly based on threshold
	isAnomaly := anomalyScore >= anomalyThreshold

	// Update global stats for temperature
	p.mu.Lock()
	p.processedData.Temperature.Count++
	p.processedData.Temperature.Sum += data.Temperature
	p.processedData.Temperature.Min = math.Min(p.processedData.Temperature.Min, data.Temperature)
	p.processedData.Temperature.Max = math.Max(p.processedData.Temperature.Max, data.Temperature)
	p.processedData.Temperature.Avg = p.processedData.Temperature.Sum / float64(p.processedData.Temperature.Count)

	// Update global stats for humidity
	p.processedData.Humidity.Count++
	p.processedData.Humidity.Sum += data.Humidity
	p.processedData.Humidity.Min = math.Min(p.processedData.Humidity.Min, data.Humidity)
	p.processedData.Humidity.Max = math.Max(p.processedData.Humidity.Max, data.Humidity)
	p.processedData.Humidity.Avg = p.processedData.Humidity.Sum / float64(p.processedData.Humidity.Count)

	// Update device-specific stats
	deviceID := data.DeviceID
	if deviceID == "" {
		deviceID = "unknown"
	}

	// Create device stats if not exist
	if _, exists := p.processedData.Devices[deviceID]; !exists {
		p.processedData.Devices[deviceID] = &models.DeviceStats{
			Temperature: struct {
				Readings []float64 `json:"readings"`
				Avg      float64   `json:"avg"`
				Min      float64   `json:"min"`
				Max      float64   `json:"max"`
			}{
				Readings: make([]float64, 0),
				Min:      math.Inf(1),
				Max:      math.Inf(-1),
			},
			Humidity: struct {
				Readings []float64 `json:"readings"`
				Avg      float64   `json:"avg"`
				Min      float64   `json:"min"`
				Max      float64   `json:"max"`
			}{
				Readings: make([]float64, 0),
				Min:      math.Inf(1),
				Max:      math.Inf(-1),
			},
		}
	}

	device := p.processedData.Devices[deviceID]

	// Update device temperature stats
	device.Temperature.Readings = append(device.Temperature.Readings, data.Temperature)
	device.Temperature.Min = math.Min(device.Temperature.Min, data.Temperature)
	device.Temperature.Max = math.Max(device.Temperature.Max, data.Temperature)

	// Calculate average
	tempSum := 0.0
	for _, v := range device.Temperature.Readings {
		tempSum += v
	}
	device.Temperature.Avg = tempSum / float64(len(device.Temperature.Readings))

	// Update device humidity stats
	device.Humidity.Readings = append(device.Humidity.Readings, data.Humidity)
	device.Humidity.Min = math.Min(device.Humidity.Min, data.Humidity)
	device.Humidity.Max = math.Max(device.Humidity.Max, data.Humidity)

	// Calculate average
	humSum := 0.0
	for _, v := range device.Humidity.Readings {
		humSum += v
	}
	device.Humidity.Avg = humSum / float64(len(device.Humidity.Readings))

	// Keep only recent readings
	maxReadings := p.cfg.MaxReadings
	if len(device.Temperature.Readings) > maxReadings {
		device.Temperature.Readings = device.Temperature.Readings[len(device.Temperature.Readings)-maxReadings:]
	}
	if len(device.Humidity.Readings) > maxReadings {
		device.Humidity.Readings = device.Humidity.Readings[len(device.Humidity.Readings)-maxReadings:]
	}

	// Update last seen
	now := time.Now()
	if data.Timestamp != nil {
		device.LastSeen = data.Timestamp
	} else {
		device.LastSeen = &now
	}

	p.mu.Unlock()

	// Create processed data result
	timestamp := now.Format(time.RFC3339)
	if data.Timestamp != nil {
		timestamp = data.Timestamp.Format(time.RFC3339)
	}

	result := &models.ProcessedDataPayload{
		DeviceID:     deviceID,
		Temperature:  data.Temperature,
		Humidity:     data.Humidity,
		Timestamp:    timestamp,
		ProcessedAt:  time.Now().Format(time.RFC3339),
		TenantID:     tenantID,
		IsAnomaly:    isAnomaly,
		AnomalyScore: anomalyScore,
	}

	// Set temperature stats
	result.TemperatureStats.Avg = device.Temperature.Avg
	result.TemperatureStats.Min = device.Temperature.Min
	result.TemperatureStats.Max = device.Temperature.Max

	// Set humidity stats
	result.HumidityStats.Avg = device.Humidity.Avg
	result.HumidityStats.Min = device.Humidity.Min
	result.HumidityStats.Max = device.Humidity.Max

	// Store processed data
	go p.storeProcessedData(result)

	// Update metrics
	processingLatency.WithLabelValues("success").Observe(time.Since(startTime).Seconds())
	messagesProcessed.WithLabelValues("success").Inc()
	tenantMessagesProcessed.WithLabelValues(tenantID).Inc()
	tenantProcessingLatency.WithLabelValues(tenantID).Observe(time.Since(startTime).Seconds())

	return result, nil
}

// storeProcessedData sends processed data to the storage service
func (p *Processor) storeProcessedData(data *models.ProcessedDataPayload) {
	if p.cfg.StorageServiceURL == "" {
		if p.cfg.Debug {
			log.Println("Storage service URL not configured, skipping data storage")
		}
		return
	}

	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling data for storage: %v", err)
		return
	}

	// Send to storage service
	resp, err := http.Post(
		fmt.Sprintf("%s/api/store", p.cfg.StorageServiceURL),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Printf("Error sending data to storage service: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Storage service returned non-OK status: %d", resp.StatusCode)
	}
}

// GetStats returns the current processing statistics
func (p *Processor) GetStats() (models.ProcessedStats, models.ProcessingStatus) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Make a deep copy to avoid race conditions
	stats := models.ProcessedStats{
		Temperature: p.processedData.Temperature,
		Humidity:    p.processedData.Humidity,
		Devices:     make(map[string]*models.DeviceStats, len(p.processedData.Devices)),
	}

	for k, v := range p.processedData.Devices {
		// Deep copy device stats
		tempReadings := make([]float64, len(v.Temperature.Readings))
		copy(tempReadings, v.Temperature.Readings)

		humReadings := make([]float64, len(v.Humidity.Readings))
		copy(humReadings, v.Humidity.Readings)

		var lastSeen *time.Time
		if v.LastSeen != nil {
			ts := *v.LastSeen
			lastSeen = &ts
		}

		deviceStats := &models.DeviceStats{
			Temperature: struct {
				Readings []float64 `json:"readings"`
				Avg      float64   `json:"avg"`
				Min      float64   `json:"min"`
				Max      float64   `json:"max"`
			}{
				Readings: tempReadings,
				Avg:      v.Temperature.Avg,
				Min:      v.Temperature.Min,
				Max:      v.Temperature.Max,
			},
			Humidity: struct {
				Readings []float64 `json:"readings"`
				Avg      float64   `json:"avg"`
				Min      float64   `json:"min"`
				Max      float64   `json:"max"`
			}{
				Readings: humReadings,
				Avg:      v.Humidity.Avg,
				Min:      v.Humidity.Min,
				Max:      v.Humidity.Max,
			},
			LastSeen: lastSeen,
		}

		stats.Devices[k] = deviceStats
	}

	status := p.status
	return stats, status
}
