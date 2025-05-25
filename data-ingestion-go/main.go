package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
)

// SensorData defines the structure for incoming sensor readings
// Using the same approach as in gin-api for consistency
type SensorData map[string]interface{}

var (
	kafkaWriter  *kafka.Writer
	kafkaTopic   string
	validAPIKeys []string
)

func main() {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting data ingestion service...")

	// Initialize API keys from environment variables
	initAPIKeys()

	// Initialize Kafka producer
	if err := initKafkaProducer(); err != nil {
		log.Printf("Warning: Failed to initialize Kafka producer: %v", err)
	}
	defer closeKafkaProducer()

	// Set up the Gin router
	r := gin.Default()

	// API endpoints
	r.POST("/api/data", requireAPIKey(), ingestData)
	r.GET("/health", healthCheck)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Server running on port %s", port)
	log.Printf("Kafka broker: %s, Topic: %s", os.Getenv("KAFKA_BROKER"), kafkaTopic)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initAPIKeys loads API keys from environment variables
func initAPIKeys() {
	apiKey1 := os.Getenv("API_KEY_1")
	apiKey2 := os.Getenv("API_KEY_2")

	// In test mode, we allow missing API keys and will use defaults
	// Check if we're in test mode by looking at gin mode
	isTestMode := gin.Mode() == gin.TestMode

	// Required API keys from environment variables (unless in test mode)
	if apiKey1 == "" {
		if isTestMode {
			apiKey1 = "test-key-1" // Default test key
		} else {
			log.Fatal("API_KEY_1 environment variable is required")
		}
	}
	if apiKey2 == "" {
		if isTestMode {
			apiKey2 = "test-key-2" // Default test key
		} else {
			log.Fatal("API_KEY_2 environment variable is required")
		}
	}

	// Create the validAPIKeys slice
	validAPIKeys = []string{apiKey1, apiKey2}

	// Also check for comma-separated API_KEYS variable
	if apiKeys := os.Getenv("API_KEYS"); apiKeys != "" {
		keys := strings.Split(apiKeys, ",")
		for _, key := range keys {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey != "" && !contains(validAPIKeys, trimmedKey) {
				validAPIKeys = append(validAPIKeys, trimmedKey)
			}
		}
	}

	log.Printf("Loaded %d API keys", len(validAPIKeys))
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// initKafkaProducer initializes the Kafka writer based on environment variables
func initKafkaProducer() error {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	kafkaTopic = os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		kafkaTopic = "sensor-data"
	}

	log.Printf("Initializing Kafka producer for broker %s, topic %s", kafkaBroker, kafkaTopic)

	// Configure the writer
	kafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Async:        false,
		MaxAttempts:  3,
		BatchSize:    1, // Send messages immediately
		BatchTimeout: 0, // No batching
		RequiredAcks: kafka.RequireAll,
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				log.Printf("Kafka async write failed: %v", err)
			}
		},
	}

	return nil
}

// produceMessage sends the data to the configured Kafka topic
func produceMessage(ctx context.Context, data SensorData) error {
	if kafkaWriter == nil {
		log.Println("Kafka producer not initialized, skipping message send.")
		return nil
	}

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling data to JSON for Kafka: %v", err)
		return err
	}

	// Log message size
	log.Printf("Message size: %d bytes", len(jsonData))

	// Use device_id as the message key if available, for partitioning
	var messageKey []byte
	if deviceID, ok := data["device_id"].(string); ok {
		messageKey = []byte(deviceID)
	}

	// Write message
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = kafkaWriter.WriteMessages(writeCtx, kafka.Message{
		Key:   messageKey,
		Value: jsonData,
	})

	if err != nil {
		if err == context.DeadlineExceeded {
			log.Printf("Kafka write timed out for key %s", string(messageKey))
		} else {
			log.Printf("Error writing message to Kafka for key %s: %v", string(messageKey), err)
		}
		return err
	}

	log.Printf("Message sent to Kafka topic '%s' with key '%s'", kafkaTopic, string(messageKey))
	return nil
}

// closeKafkaProducer closes the Kafka writer connection
func closeKafkaProducer() {
	if kafkaWriter != nil {
		log.Println("Closing Kafka producer...")
		if err := kafkaWriter.Close(); err != nil {
			log.Printf("Error closing Kafka writer: %v", err)
		}
	}
}

// requireAPIKey is a middleware that checks for a valid API key
func requireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if auth is bypassed
		bypassAuth := os.Getenv("BYPASS_AUTH")
		if bypassAuth == "true" {
			c.Next()
			return
		}

		// Check for API key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Missing API key",
			})
			c.Abort()
			return
		}

		// Validate API key
		valid := false
		for _, key := range validAPIKeys {
			if apiKey == key {
				valid = true
				break
			}
		}

		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid API key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateSensorData ensures the required fields are present
func validateSensorData(data SensorData) error {
	// Check if device_id is present
	deviceID, ok := data["device_id"]
	if !ok || deviceID == "" {
		return errors.New("device_id is required")
	}

	// Additional validation can be added here
	return nil
}

// ingestData handles the data ingestion endpoint
func ingestData(c *gin.Context) {
	var data SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid JSON data",
		})
		return
	}

	// Validate required fields
	if err := validateSensorData(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Add timestamp if not present
	if _, exists := data["timestamp"]; !exists {
		data["timestamp"] = time.Now().Format(time.RFC3339)
	}

	// Send to Kafka
	var kafkaStatus string
	err := produceMessage(c.Request.Context(), data)
	if err != nil {
		kafkaStatus = "error: " + err.Error()
	} else {
		kafkaStatus = "sent"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      "Data received successfully",
		"data":         data,
		"kafka_status": kafkaStatus,
	})
}

// healthCheck provides a simple health check endpoint
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
