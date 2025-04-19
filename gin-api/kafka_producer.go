package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

var (
	kafkaWriter *kafka.Writer
	kafkaTopic  string
)

// initKafkaProducer initializes the Kafka writer based on environment variables.
func initKafkaProducer() error {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	kafkaTopic = os.Getenv("KAFKA_TOPIC")

	if kafkaBroker == "" {
		log.Println("Warning: KAFKA_BROKER not set. Kafka producer disabled.")
		return nil // Not necessarily an error, just disabled
	}
	if kafkaTopic == "" {
		log.Println("Warning: KAFKA_TOPIC not set. Kafka producer disabled.")
		return nil // Not necessarily an error, just disabled
	}

	log.Printf("Initializing Kafka producer for broker %s, topic %s", kafkaBroker, kafkaTopic)

	// Configure the writer
	// Use Balancer: &kafka.LeastBytes{} for better distribution if multiple partitions
	kafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second, // Add timeouts
		ReadTimeout:  10 * time.Second,
		Async:        false, // Set to false for synchronous writes initially for simplicity
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				log.Printf("Kafka async write failed: %v", err)
			}
		},
	}

	// Optional: Test connection (can block startup)
	// conn, err := kafka.DialLeader(context.Background(), "tcp", kafkaBroker, kafkaTopic, 0) // Check partition 0
	// if err != nil {
	// 	 log.Printf("Failed to connect to Kafka leader: %v", err)
	// 	 return err
	// }
	// conn.Close()
	// log.Println("Kafka connection successful.")

	return nil
}

// produceMessage sends the data to the configured Kafka topic.
func produceMessage(ctx context.Context, data SensorData) error {
	if kafkaWriter == nil {
		log.Println("Kafka producer not initialized, skipping message send.")
		// In a real app, you might buffer or return an error
		return nil
	}

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling data to JSON for Kafka: %v", err)
		return err
	}

	// Use device_id as the message key if available, for partitioning
	var messageKey []byte
	if deviceID, ok := data["device_id"].(string); ok {
		messageKey = []byte(deviceID)
	}

	// Write message
	// Use context with timeout for the write operation
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
	// Increment Prometheus counter (defined in metrics.go)
	kafkaMessagesProduced.WithLabelValues(kafkaTopic).Inc()
	return nil
}

// closeKafkaProducer closes the Kafka writer connection.
func closeKafkaProducer() {
	if kafkaWriter != nil {
		log.Println("Closing Kafka producer...")
		if err := kafkaWriter.Close(); err != nil {
			log.Printf("Error closing Kafka writer: %v", err)
		}
	}
}
