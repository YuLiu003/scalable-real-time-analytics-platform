package main

import (
	"context"
	"encoding/json"
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

	// Just log the data for now
	log.Printf("Data: %+v", data)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data logged successfully",
		"data":    data,
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

// cleanData applies data cleaning and validation rules
func cleanData(data map[string]interface{}) map[string]interface{} {
	// Add your data cleaning logic here
	// For now, we'll just pass through the data with a timestamp
	cleanedData := make(map[string]interface{})
	for k, v := range data {
		cleanedData[k] = v
	}

	// Add processing timestamp
	cleanedData["processed_at"] = time.Now().UTC().Format(time.RFC3339)

	return cleanedData
}
