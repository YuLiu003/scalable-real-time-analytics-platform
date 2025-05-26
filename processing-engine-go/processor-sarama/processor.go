package processor

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"processing-engine-go/config"
	"processing-engine-go/models"
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
	consumerGroup sarama.ConsumerGroup
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

	// Initialize temperature min/max values with realistic defaults
	// These will be updated when first data arrives
	p.processedData.Temperature.Min = 999999  // Will be updated on first temperature reading
	p.processedData.Temperature.Max = -999999 // Will be updated on first temperature reading
	p.processedData.Humidity.Min = 999999     // Will be updated on first humidity reading
	p.processedData.Humidity.Max = -999999    // Will be updated on first humidity reading

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
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin

	// Add connection timeout
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second

	// Retry connection to Kafka
	const maxRetries = 5
	const retryDelay = 5 * time.Second

	var consumerErr, producerErr error

	// Create consumer group with retries
	for retry := 0; retry < maxRetries; retry++ {
		p.consumerGroup, consumerErr = sarama.NewConsumerGroup([]string{p.cfg.KafkaBroker}, "processing-engine-group", config)
		if consumerErr == nil {
			break
		}

		log.Printf("Failed to connect to Kafka consumer group (attempt %d/%d): %v",
			retry+1, maxRetries, consumerErr)

		if retry < maxRetries-1 {
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if consumerErr != nil {
		return fmt.Errorf("failed to create consumer group after %d attempts: %v", maxRetries, consumerErr)
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
		// Clean up consumer group if producer fails
		if p.consumerGroup != nil {
			if err := p.consumerGroup.Close(); err != nil {
				log.Printf("Error closing consumer group during cleanup: %v", err)
			}
		}
		return fmt.Errorf("failed to create producer after %d attempts: %v", maxRetries, producerErr)
	}

	p.mu.Lock()
	p.status.Running = true
	p.status.LastStarted = time.Now().Format(time.RFC3339)
	p.mu.Unlock()

	// Start processing messages in a goroutine
	go p.consumeMessages()

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

	if p.consumerGroup != nil {
		if err := p.consumerGroup.Close(); err != nil {
			log.Printf("Error closing consumer group: %v", err)
		}
	}
	if p.producer != nil {
		if err := p.producer.Close(); err != nil {
			log.Printf("Error closing producer: %v", err)
		}
	}
}

// consumeMessages starts the consumer group and processes messages
func (p *Processor) consumeMessages() {
	ctx := context.Background()
	handler := &ProcessorHandler{processor: p}

	for {
		select {
		case <-p.done:
			return
		default:
			if err := p.consumerGroup.Consume(ctx, []string{p.cfg.InputTopic}, handler); err != nil {
				log.Printf("Error consuming messages: %v", err)
				time.Sleep(time.Second)
			}
		}
	}
}

// ProcessorHandler implements sarama.ConsumerGroupHandler
type ProcessorHandler struct {
	processor *Processor
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *ProcessorHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *ProcessorHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages()
func (h *ProcessorHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			log.Printf("Received message from topic %s, partition %d, offset %d",
				message.Topic, message.Partition, message.Offset)

			// Process the message
			var rawData models.SensorData
			if err := json.Unmarshal(message.Value, &rawData); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				messagesProcessed.WithLabelValues("error").Inc()
				h.processor.mu.Lock()
				h.processor.status.Errors++
				h.processor.mu.Unlock()
				session.MarkMessage(message, "")
				continue
			}

			result, err := h.processor.processData(rawData)
			if err != nil {
				log.Printf("Error processing data: %v", err)
				messagesProcessed.WithLabelValues("error").Inc()
				h.processor.mu.Lock()
				h.processor.status.Errors++
				h.processor.mu.Unlock()
				session.MarkMessage(message, "")
				continue
			}

			// Produce the result to the output topic
			resultBytes, err := json.Marshal(result)
			if err != nil {
				log.Printf("Error marshaling result: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			_, _, err = h.processor.producer.SendMessage(&sarama.ProducerMessage{
				Topic: h.processor.cfg.OutputTopic,
				Value: sarama.ByteEncoder(resultBytes),
			})
			if err != nil {
				log.Printf("Error sending message to Kafka: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			log.Printf("Successfully processed and sent message to %s", h.processor.cfg.OutputTopic)

			// Update status
			h.processor.mu.Lock()
			h.processor.status.MessagesProcessed++
			h.processor.status.LastProcessed = time.Now().Format(time.RFC3339)
			h.processor.mu.Unlock()

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
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
	if samplingRate < 1.0 {
		// Use crypto/rand for secure random sampling
		randomInt, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err == nil {
			randomFloat := float64(randomInt.Int64()) / 1000000.0
			if randomFloat > samplingRate {
				// Skip processing
				return nil, fmt.Errorf("skipped due to sampling rate")
			}
		}
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

	// Update global statistics and device-specific tracking
	p.mu.Lock()

	// Ensure device exists in the devices map
	if p.processedData.Devices[data.DeviceID] == nil {
		p.processedData.Devices[data.DeviceID] = &models.DeviceStats{}
		// Initialize device temperature and humidity stats
		if data.Temperature != nil {
			p.processedData.Devices[data.DeviceID].Temperature.Min = *data.Temperature
			p.processedData.Devices[data.DeviceID].Temperature.Max = *data.Temperature
		}
		if data.Humidity != nil {
			p.processedData.Devices[data.DeviceID].Humidity.Min = *data.Humidity
			p.processedData.Devices[data.DeviceID].Humidity.Max = *data.Humidity
		}
	}

	deviceStats := p.processedData.Devices[data.DeviceID]

	if data.Temperature != nil {
		// Update global temperature stats
		p.processedData.Temperature.Count++
		p.processedData.Temperature.Sum += *data.Temperature

		// Handle first temperature reading globally
		if p.processedData.Temperature.Count == 1 {
			p.processedData.Temperature.Min = *data.Temperature
			p.processedData.Temperature.Max = *data.Temperature
		} else {
			p.processedData.Temperature.Min = math.Min(p.processedData.Temperature.Min, *data.Temperature)
			p.processedData.Temperature.Max = math.Max(p.processedData.Temperature.Max, *data.Temperature)
		}
		p.processedData.Temperature.Avg = p.processedData.Temperature.Sum / float64(p.processedData.Temperature.Count)

		// Update device-specific temperature stats
		deviceStats.Temperature.Readings = append(deviceStats.Temperature.Readings, *data.Temperature)
		if len(deviceStats.Temperature.Readings) > 100 { // Keep only last 100 readings
			deviceStats.Temperature.Readings = deviceStats.Temperature.Readings[1:]
		}

		// Calculate device temperature statistics
		sum := 0.0
		min := deviceStats.Temperature.Readings[0]
		max := deviceStats.Temperature.Readings[0]
		for _, reading := range deviceStats.Temperature.Readings {
			sum += reading
			if reading < min {
				min = reading
			}
			if reading > max {
				max = reading
			}
		}
		deviceStats.Temperature.Avg = sum / float64(len(deviceStats.Temperature.Readings))
		deviceStats.Temperature.Min = min
		deviceStats.Temperature.Max = max
	}

	if data.Humidity != nil {
		// Update global humidity stats
		p.processedData.Humidity.Count++
		p.processedData.Humidity.Sum += *data.Humidity

		// Handle first humidity reading globally
		if p.processedData.Humidity.Count == 1 {
			p.processedData.Humidity.Min = *data.Humidity
			p.processedData.Humidity.Max = *data.Humidity
		} else {
			p.processedData.Humidity.Min = math.Min(p.processedData.Humidity.Min, *data.Humidity)
			p.processedData.Humidity.Max = math.Max(p.processedData.Humidity.Max, *data.Humidity)
		}
		p.processedData.Humidity.Avg = p.processedData.Humidity.Sum / float64(p.processedData.Humidity.Count)

		// Update device-specific humidity stats
		deviceStats.Humidity.Readings = append(deviceStats.Humidity.Readings, *data.Humidity)
		if len(deviceStats.Humidity.Readings) > 100 { // Keep only last 100 readings
			deviceStats.Humidity.Readings = deviceStats.Humidity.Readings[1:]
		}

		// Calculate device humidity statistics
		sum := 0.0
		min := deviceStats.Humidity.Readings[0]
		max := deviceStats.Humidity.Readings[0]
		for _, reading := range deviceStats.Humidity.Readings {
			sum += reading
			if reading < min {
				min = reading
			}
			if reading > max {
				max = reading
			}
		}
		deviceStats.Humidity.Avg = sum / float64(len(deviceStats.Humidity.Readings))
		deviceStats.Humidity.Min = min
		deviceStats.Humidity.Max = max
	}

	// Update last seen time for the device
	if data.Timestamp != nil {
		deviceStats.LastSeen = data.Timestamp
	} else {
		now := time.Now()
		deviceStats.LastSeen = &now
	}

	p.mu.Unlock()

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
