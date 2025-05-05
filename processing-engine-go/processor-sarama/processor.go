package processor

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/config"
	"github.com/YuLiu003/real-time-analytics-platform/processing-engine-go-new/models"
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
	consumer      sarama.Consumer
	producer      sarama.SyncProducer
	processedData models.ProcessedStats
	status        models.ProcessingStatus
	mu            sync.RWMutex // Mutex for thread-safe access to shared data
	done          chan struct{}
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
		done: make(chan struct{}),
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

	// Set up Kafka configuration
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	// Add connection timeout
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second

	// Retry connection to Kafka
	const maxRetries = 5
	const retryDelay = 5 * time.Second

	var consumerErr, producerErr error

	// Create consumer with retries
	for retry := 0; retry < maxRetries; retry++ {
		p.consumer, consumerErr = sarama.NewConsumer([]string{p.cfg.KafkaBroker}, config)
		if consumerErr == nil {
			break
		}

		log.Printf("Failed to connect to Kafka consumer (attempt %d/%d): %v",
			retry+1, maxRetries, consumerErr)

		if retry < maxRetries-1 {
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if consumerErr != nil {
		return fmt.Errorf("failed to create consumer after %d attempts: %v", maxRetries, consumerErr)
	}

	// Create producer with retries
	for retry := 0; retry < maxRetries; retry++ {
		p.producer, producerErr = sarama.NewSyncProducer([]string{p.cfg.KafkaBroker}, config)
		if producerErr == nil {
			break
		}

		log.Printf("Failed to connect to Kafka producer (attempt %d/%d): %v",
			retry+1, maxRetries, producerErr)

		if retry < maxRetries-1 {
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if producerErr != nil {
		// Clean up consumer if producer fails
		if p.consumer != nil {
			p.consumer.Close()
		}
		return fmt.Errorf("failed to create producer after %d attempts: %v", maxRetries, producerErr)
	}

	// Get the partition consumer with retries
	var partitionConsumer sarama.PartitionConsumer
	var partitionErr error

	for retry := 0; retry < maxRetries; retry++ {
		partitionConsumer, partitionErr = p.consumer.ConsumePartition(
			p.cfg.InputTopic, 0, sarama.OffsetNewest)
		if partitionErr == nil {
			break
		}

		log.Printf("Failed to create partition consumer (attempt %d/%d): %v",
			retry+1, maxRetries, partitionErr)

		if retry < maxRetries-1 {
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if partitionErr != nil {
		// Clean up resources
		if p.consumer != nil {
			p.consumer.Close()
		}
		if p.producer != nil {
			p.producer.Close()
		}
		return fmt.Errorf("failed to create partition consumer after %d attempts: %v",
			maxRetries, partitionErr)
	}

	p.mu.Lock()
	p.status.Running = true
	p.status.LastStarted = time.Now().Format(time.RFC3339)
	p.mu.Unlock()

	// Start processing messages in a goroutine
	go p.processMessages(partitionConsumer)

	log.Printf("Processing engine started. Input topic: %s, Output topic: %s",
		p.cfg.InputTopic, p.cfg.OutputTopic)

	return nil
}

// Stop stops the processor
func (p *Processor) Stop() {
	p.mu.Lock()
	p.status.Running = false
	p.mu.Unlock()

	close(p.done)

	if p.consumer != nil {
		p.consumer.Close()
	}
	if p.producer != nil {
		p.producer.Close()
	}
}

// processMessages continuously processes messages from Kafka
func (p *Processor) processMessages(partitionConsumer sarama.PartitionConsumer) {
	for {
		select {
		case <-p.done:
			return
		case msg := <-partitionConsumer.Messages():
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

			_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
				Topic: p.cfg.OutputTopic,
				Value: sarama.ByteEncoder(resultBytes),
			})
			if err != nil {
				log.Printf("Error sending message to Kafka: %v", err)
				continue
			}

			// Update status
			p.mu.Lock()
			p.status.MessagesProcessed++
			p.status.LastProcessed = time.Now().Format(time.RFC3339)
			p.mu.Unlock()
		}
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
	if data.Temperature != nil && (*data.Temperature > 30 || *data.Temperature < 10) {
		anomalyScore += 1.0
	}

	// Check if humidity is outside normal range
	if data.Humidity != nil && (*data.Humidity > 80 || *data.Humidity < 20) {
		anomalyScore += 1.0
	}

	// Determine if anomaly based on threshold
	isAnomaly := anomalyScore >= anomalyThreshold

	// Format timestamps properly
	var timestampStr string
	if data.Timestamp != nil {
		timestampStr = data.Timestamp.Format(time.RFC3339)
	} else {
		timestampStr = time.Now().Format(time.RFC3339)
	}

	// Create processed result
	result := &models.ProcessedDataPayload{
		DeviceID:     data.DeviceID,
		Temperature:  data.Temperature,
		Humidity:     data.Humidity,
		Timestamp:    timestampStr,
		ProcessedAt:  time.Now().Format(time.RFC3339),
		IsAnomaly:    isAnomaly,
		AnomalyScore: anomalyScore,
		TenantID:     tenantID,
	}

	// Record metrics
	tenantMessagesProcessed.WithLabelValues(tenantID).Inc()
	processingTime := time.Since(startTime).Seconds()
	tenantProcessingLatency.WithLabelValues(tenantID).Observe(processingTime)

	if isAnomaly {
		messagesProcessed.WithLabelValues("anomaly").Inc()
	} else {
		messagesProcessed.WithLabelValues("normal").Inc()
	}

	if data.Temperature != nil {
		p.mu.Lock()
		p.processedData.Temperature.Count++
		p.processedData.Temperature.Sum += *data.Temperature
		p.processedData.Temperature.Min = math.Min(p.processedData.Temperature.Min, *data.Temperature)
		p.processedData.Temperature.Max = math.Max(p.processedData.Temperature.Max, *data.Temperature)
		p.processedData.Temperature.Avg = p.processedData.Temperature.Sum / float64(p.processedData.Temperature.Count)
		p.mu.Unlock()
	}

	if data.Humidity != nil {
		p.mu.Lock()
		p.processedData.Humidity.Count++
		p.processedData.Humidity.Sum += *data.Humidity
		p.processedData.Humidity.Min = math.Min(p.processedData.Humidity.Min, *data.Humidity)
		p.processedData.Humidity.Max = math.Max(p.processedData.Humidity.Max, *data.Humidity)
		p.processedData.Humidity.Avg = p.processedData.Humidity.Sum / float64(p.processedData.Humidity.Count)
		p.mu.Unlock()
	}

	return result, nil
}

// GetStats returns the current processing statistics
func (p *Processor) GetStats() (models.ProcessedStats, models.ProcessingStatus) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.processedData, p.status
}

// ProcessData processes a single piece of data and returns the processed result
func (p *Processor) ProcessData(data models.SensorData) (*models.ProcessedDataPayload, error) {
	return p.processData(data)
}
