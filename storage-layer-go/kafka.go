package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/IBM/sarama"
)

// KafkaConsumer handles consuming messages from Kafka
type KafkaConsumer struct {
	consumer sarama.Consumer
	topic    string
	db       *DB
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(broker, topic string, db *DB) (*KafkaConsumer, error) {
	log.Printf("[DEBUG] Entered NewKafkaConsumer with broker=%s, topic=%s", broker, topic)
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	log.Printf("[DEBUG] Creating Sarama consumer for broker=%s", broker)
	consumer, err := sarama.NewConsumer([]string{broker}, config)
	if err != nil {
		log.Printf("[ERROR] Failed to create Sarama consumer: %v", err)
		return nil, fmt.Errorf("failed to create consumer: %v", err)
	}

	log.Printf("[DEBUG] Sarama consumer created successfully")
	return &KafkaConsumer{
		consumer: consumer,
		topic:    topic,
		db:       db,
		done:     make(chan struct{}),
	}, nil
}

// Start begins consuming messages from Kafka
func (k *KafkaConsumer) Start() error {
	log.Printf("[DEBUG] Entered KafkaConsumer.Start for topic: %s", k.topic)

	// Log available partitions for the topic
	partitions, err := k.consumer.Partitions(k.topic)
	if err != nil {
		log.Printf("[ERROR] Failed to get partitions for topic %s: %v", k.topic, err)
		return fmt.Errorf("failed to get partitions for topic %s: %v", k.topic, err)
	}
	log.Printf("[DEBUG] Available partitions for topic %s: %v", k.topic, partitions)

	// Start a consumer for each partition
	log.Printf("[DEBUG] Starting loop to create consumers for %d partitions", len(partitions))
	for i, partition := range partitions {
		log.Printf("[DEBUG] Loop iteration %d: Attempting to start partition consumer for topic=%s, partition=%d", i, k.topic, partition)
		partitionConsumer, err := k.consumer.ConsumePartition(k.topic, partition, sarama.OffsetOldest)
		if err != nil {
			log.Printf("[ERROR] Failed to start partition consumer for partition %d: %v", partition, err)
			return fmt.Errorf("failed to start consumer for partition %d: %v", partition, err)
		}

		log.Printf("[DEBUG] Partition consumer started successfully for topic=%s, partition=%d", k.topic, partition)

		k.wg.Add(1)
		go func(pc sarama.PartitionConsumer, partitionID int32) {
			defer k.wg.Done()
			defer pc.Close()
			log.Printf("[DEBUG] Entered partition consumer goroutine for topic=%s, partition=%d", k.topic, partitionID)
			defer log.Printf("[DEBUG] Exiting partition consumer goroutine for topic=%s, partition=%d", k.topic, partitionID)

			for {
				select {
				case <-k.done:
					log.Printf("[KafkaConsumer] Received shutdown signal, stopping partition consumer for partition %d...", partitionID)
					return
				case msg := <-pc.Messages():
					log.Printf("[KafkaConsumer] Received message: partition=%d, offset=%d, key=%s", msg.Partition, msg.Offset, string(msg.Key))
					if err := k.processMessage(msg); err != nil {
						log.Printf("[KafkaConsumer] Error processing message: %v", err)
					}
				case err := <-pc.Errors():
					log.Printf("[KafkaConsumer] Error from consumer partition %d: %v", partitionID, err)
				}
			}
		}(partitionConsumer, partition)
	}

	log.Printf("[DEBUG] KafkaConsumer.Start completed for topic=%s", k.topic)
	return nil
}

// Stop stops the consumer
func (k *KafkaConsumer) Stop() {
	close(k.done)
	k.wg.Wait()
	k.consumer.Close()
}

// processMessage processes a single Kafka message
func (k *KafkaConsumer) processMessage(msg *sarama.ConsumerMessage) error {
	log.Printf("[KafkaConsumer] Processing message: offset=%d, key=%s", msg.Offset, string(msg.Key))
	var data SensorDataRequest
	if err := json.Unmarshal(msg.Value, &data); err != nil {
		log.Printf("[KafkaConsumer] Error unmarshaling message: %v", err)
		return fmt.Errorf("error unmarshaling message: %v", err)
	}

	// Store the data
	if err := k.db.StoreSensorData(&data); err != nil {
		log.Printf("[KafkaConsumer] Error storing data: %v", err)
		return fmt.Errorf("error storing data: %v", err)
	}

	log.Printf("[KafkaConsumer] Successfully stored data for device_id=%s, tenant_id=%s", data.DeviceID, data.TenantID)
	return nil
}

// StartKafkaConsumer starts the Kafka consumer (deprecated - use NewKafkaConsumer + Start instead)
func StartKafkaConsumer(broker, topic string, db *DB) error {
	log.Printf("[DEBUG] (kafka.go) Entered StartKafkaConsumer with broker=%s, topic=%s", broker, topic)
	consumer, err := NewKafkaConsumer(broker, topic, db)
	if err != nil {
		log.Printf("[ERROR] (kafka.go) Failed to create Kafka consumer: %v", err)
		return fmt.Errorf("failed to create Kafka consumer: %v", err)
	}

	log.Printf("[DEBUG] (kafka.go) Starting KafkaConsumer.Start for topic=%s", topic)
	if err := consumer.Start(); err != nil {
		log.Printf("[ERROR] (kafka.go) Failed to start Kafka consumer: %v", err)
		return fmt.Errorf("failed to start Kafka consumer: %v", err)
	}

	log.Printf("[DEBUG] (kafka.go) KafkaConsumer.Start returned successfully for topic=%s", topic)

	// Handle graceful shutdown
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigterm
		log.Println("[KafkaConsumer] Received shutdown signal, stopping Kafka consumer...")
		consumer.Stop()
		log.Println("[KafkaConsumer] Kafka consumer stopped.")
	}()

	log.Printf("[DEBUG] (kafka.go) Exiting StartKafkaConsumer for topic=%s", topic)
	return nil
}
