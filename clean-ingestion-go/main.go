package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
)

// CleanData represents the structure for incoming data
type CleanData map[string]interface{}

func main() {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting clean ingestion service...")

	// Get configuration from environment
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "kafka:9092"
	}

	inputTopic := os.Getenv("KAFKA_TOPIC")
	if inputTopic == "" {
		inputTopic = "sensor-data"
	}

	outputTopic := "sensor-data-clean"

	log.Printf("Configuration: Kafka broker: %s, Input topic: %s, Output topic: %s", kafkaBroker, inputTopic, outputTopic)

	// Start Kafka consumer in a goroutine
	go startKafkaConsumer(kafkaBroker, inputTopic, outputTopic)

	// Set up the Gin router for health checks
	r := gin.Default()
	r.GET("/health", healthCheck)
	r.POST("/api/data", ingestData)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Server running on port %s", port)
	log.Printf("Kafka broker: %s, Input topic: %s, Output topic: %s", kafkaBroker, inputTopic, outputTopic)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// healthCheck provides a simple health check endpoint
func healthCheck(c *gin.Context) {
	log.Println("Health check requested")
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// ingestData handles the data ingestion endpoint
func ingestData(c *gin.Context) {
	log.Println("Data received")

	var data CleanData
	if err := c.ShouldBindJSON(&data); err != nil {
		log.Printf("Invalid JSON data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid JSON data",
		})
		return
	}

	if len(data) == 0 {
		log.Println("No data provided")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No data provided",
		})
		return
	}

	// Validate required fields
	if err := validateRequiredFields(data); err != nil {
		log.Printf("Validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Clean the data
	cleanedData := cleanData(data)
	log.Printf("Data cleaned: %+v", cleanedData)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data cleaned and logged successfully",
		"data":    cleanedData,
	})
}

// startKafkaConsumer consumes from input topic, cleans data, and produces to output topic
func startKafkaConsumer(broker, inputTopic, outputTopic string) {
	log.Printf("Starting Kafka consumer setup for broker=%s, input=%s, output=%s", broker, inputTopic, outputTopic)

	// Configure Sarama consumer
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	// Configure producer
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3

	log.Printf("Creating consumer group for topic %s", inputTopic)

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup([]string{broker}, "clean-ingestion-group", config)
	if err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}
	defer consumerGroup.Close()

	// Create producer
	producer, err := sarama.NewSyncProducer([]string{broker}, config)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()

	log.Printf("Starting Kafka consumer for topic %s", inputTopic)

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Create consumer handler
	handler := &CleanConsumerHandler{
		producer:    producer,
		outputTopic: outputTopic,
	}

	// Start consuming
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := consumerGroup.Consume(ctx, []string{inputTopic}, handler); err != nil {
					log.Printf("Error consuming: %v", err)
					time.Sleep(time.Second)
				}
			}
		}
	}()

	// Wait for shutdown signal
	<-signals
	log.Println("Shutting down Kafka consumer...")
	cancel()
}

// CleanConsumerHandler implements sarama.ConsumerGroupHandler
type CleanConsumerHandler struct {
	producer    sarama.SyncProducer
	outputTopic string
}

func (h *CleanConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *CleanConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *CleanConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			log.Printf("Received message from %s: %s", message.Topic, string(message.Value))

			// Parse the message
			var data map[string]interface{}
			if err := json.Unmarshal(message.Value, &data); err != nil {
				log.Printf("Failed to parse message: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			// Clean/validate the data (add your cleaning logic here)
			cleanedData := cleanData(data)

			// Produce to output topic
			cleanedBytes, err := json.Marshal(cleanedData)
			if err != nil {
				log.Printf("Failed to marshal cleaned data: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			msg := &sarama.ProducerMessage{
				Topic: h.outputTopic,
				Value: sarama.ByteEncoder(cleanedBytes),
			}

			if message.Key != nil {
				msg.Key = sarama.ByteEncoder(message.Key)
			}

			_, _, err = h.producer.SendMessage(msg)
			if err != nil {
				log.Printf("Failed to send message to %s: %v", h.outputTopic, err)
			} else {
				log.Printf("Successfully sent cleaned message to %s", h.outputTopic)
			}

			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// validateRequiredFields checks for mandatory fields in the data
func validateRequiredFields(data map[string]interface{}) error {
	// Check for device_id or sensor_id
	deviceID, hasDeviceID := data["device_id"]
	sensorID, hasSensorID := data["sensor_id"]

	if !hasDeviceID && !hasSensorID {
		return errors.New("missing required field: device_id or sensor_id")
	}

	if hasDeviceID {
		if deviceIDStr, ok := deviceID.(string); !ok || deviceIDStr == "" {
			return errors.New("device_id must be a non-empty string")
		}
	}

	if hasSensorID {
		if sensorIDStr, ok := sensorID.(string); !ok || sensorIDStr == "" {
			return errors.New("sensor_id must be a non-empty string")
		}
	}

	// Optional: Check for timestamp format if provided
	if timestamp, hasTimestamp := data["timestamp"]; hasTimestamp {
		if timestampStr, ok := timestamp.(string); ok {
			if _, err := time.Parse(time.RFC3339, timestampStr); err != nil {
				// Try Unix timestamp as number
				if _, ok := timestamp.(float64); !ok {
					return errors.New("timestamp must be RFC3339 string or Unix timestamp number")
				}
			}
		}
	}

	return nil
}

// cleanData applies data cleaning and validation rules
func cleanData(data map[string]interface{}) map[string]interface{} {
	cleanedData := make(map[string]interface{})

	// Copy all fields, applying cleaning rules
	for k, v := range data {
		switch k {
		case "timestamp":
			// Ensure timestamp is in RFC3339 format
			cleanedData[k] = normalizeTimestamp(v)
		case "temperature", "humidity":
			// Validate numeric sensor values
			if numVal, ok := v.(float64); ok {
				cleanedData[k] = validateSensorValue(k, numVal)
			} else {
				cleanedData[k] = v // Keep original if not numeric
			}
		default:
			cleanedData[k] = v
		}
	}

	// Add processing timestamp
	cleanedData["processed_at"] = time.Now().UTC().Format(time.RFC3339)

	// Add data quality score (basic implementation)
	cleanedData["quality_score"] = calculateQualityScore(cleanedData)

	return cleanedData
}

// normalizeTimestamp ensures timestamp is in RFC3339 format
func normalizeTimestamp(timestamp interface{}) string {
	switch v := timestamp.(type) {
	case string:
		// Try to parse as RFC3339
		if _, err := time.Parse(time.RFC3339, v); err == nil {
			return v
		}
		// If parse fails, return current time
		return time.Now().UTC().Format(time.RFC3339)
	case float64:
		// Unix timestamp
		return time.Unix(int64(v), 0).UTC().Format(time.RFC3339)
	default:
		return time.Now().UTC().Format(time.RFC3339)
	}
}

// validateSensorValue applies basic validation to sensor readings
func validateSensorValue(sensorType string, value float64) float64 {
	switch sensorType {
	case "temperature":
		// Temperature should be reasonable (-50 to 60 Celsius)
		if value < -50 || value > 60 {
			log.Printf("Warning: Unusual temperature value: %f", value)
		}
	case "humidity":
		// Humidity should be 0-100%
		if value < 0 || value > 100 {
			log.Printf("Warning: Invalid humidity value: %f, clamping to valid range", value)
			if value < 0 {
				return 0
			}
			if value > 100 {
				return 100
			}
		}
	}
	return value
}

// calculateQualityScore provides a basic data quality assessment
func calculateQualityScore(data map[string]interface{}) float64 {
	score := 1.0

	// Reduce score for missing common fields
	commonFields := []string{"device_id", "sensor_id", "timestamp"}
	for _, field := range commonFields {
		if _, exists := data[field]; !exists {
			score -= 0.1
		}
	}

	// Reduce score for unusual values (already logged in validateSensorValue)
	if temp, exists := data["temperature"]; exists {
		if tempVal, ok := temp.(float64); ok && (tempVal < -50 || tempVal > 60) {
			score -= 0.2
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}
