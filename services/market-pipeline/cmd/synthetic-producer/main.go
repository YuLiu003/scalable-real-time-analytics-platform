package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/synthetic"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("producer failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	client, err := kafkaclient.FromEnvironment()
	if err != nil {
		return err
	}
	producerConfig, err := client.Producer("synthetic-market-producer")
	if err != nil {
		return err
	}
	producer, err := sarama.NewSyncProducer(client.Brokers, producerConfig)
	if err != nil {
		return fmt.Errorf("create Kafka producer: %w", err)
	}
	defer producer.Close()

	topic := envOrDefault("KAFKA_TOPIC", "market.prices")
	scenario := envOrDefault("PRODUCER_SCENARIO", "baseline")
	messages, err := synthetic.Messages(scenario)
	if err != nil {
		return err
	}

	for index, message := range messages {
		partition, offset, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(message.Key),
			Value: sarama.ByteEncoder(message.Value),
			Headers: []sarama.RecordHeader{
				{Key: []byte("event_id"), Value: []byte(message.EventID)},
				{Key: []byte("schema_version"), Value: []byte("1")},
			},
		})
		if err != nil {
			return fmt.Errorf("send event %s: %w", message.EventID, err)
		}
		logger.Info("broker acknowledged event", "event_id", message.EventID, "topic", topic, "partition", partition, "offset", offset)
		if index == 0 && envOrDefault("FAIL_AFTER_FIRST_ACK", "false") == "true" {
			return errors.New("injected producer failure after broker acknowledgement")
		}
	}
	logger.Info("producer scenario completed", "scenario", scenario, "messages", len(messages))
	return nil
}
func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
