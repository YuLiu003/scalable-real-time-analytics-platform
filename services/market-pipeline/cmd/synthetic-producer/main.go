package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/scale"
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
	topic := envOrDefault("KAFKA_TOPIC", "market.prices")
	scenario := envOrDefault("PRODUCER_SCENARIO", "baseline")
	producerConfig, err := client.Producer("synthetic-market-producer")
	if err != nil {
		return err
	}
	if scenario == "load" {
		return runLoad(logger, client, producerConfig, topic)
	}
	producer, err := sarama.NewSyncProducer(client.Brokers, producerConfig)
	if err != nil {
		return fmt.Errorf("create Kafka producer: %w", err)
	}
	defer producer.Close()

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

type loadAcknowledgements struct {
	latencies []time.Duration
	firstErr  error
}

func runLoad(logger *slog.Logger, client kafkaclient.Config, producerConfig *sarama.Config, topic string) error {
	config, err := synthetic.LoadConfigFromEnvironment()
	if err != nil {
		return err
	}
	generator, err := synthetic.NewLoadGenerator(config)
	if err != nil {
		return err
	}
	producerConfig.Producer.Flush.Messages = 500
	producerConfig.Producer.Flush.Frequency = 50 * time.Millisecond
	producer, err := sarama.NewAsyncProducer(client.Brokers, producerConfig)
	if err != nil {
		return fmt.Errorf("create asynchronous Kafka producer: %w", err)
	}

	var acknowledgements loadAcknowledgements
	var collector sync.WaitGroup
	collector.Add(1)
	go func() {
		defer collector.Done()
		successes, failures := producer.Successes(), producer.Errors()
		for successes != nil || failures != nil {
			select {
			case message, ok := <-successes:
				if !ok {
					successes = nil
					continue
				}
				if sentAt, ok := message.Metadata.(time.Time); ok {
					acknowledgements.latencies = append(acknowledgements.latencies, time.Since(sentAt))
				} else if acknowledgements.firstErr == nil {
					acknowledgements.firstErr = errors.New("Kafka acknowledgement is missing producer timing metadata")
				}
			case failure, ok := <-failures:
				if !ok {
					failures = nil
					continue
				}
				if acknowledgements.firstErr == nil {
					acknowledgements.firstErr = fmt.Errorf("send scale event: %w", failure.Err)
				}
			}
		}
	}()

	var ticker *time.Ticker
	if config.TargetRate > 0 {
		ticker = time.NewTicker(time.Second / time.Duration(config.TargetRate))
		defer ticker.Stop()
	}
	startedAt := time.Now()
	var sent int64
	for {
		message, ok, err := generator.Next()
		if err != nil {
			producer.AsyncClose()
			collector.Wait()
			return err
		}
		if !ok {
			break
		}
		if ticker != nil && sent > 0 {
			<-ticker.C
		}
		sentAt := time.Now()
		producer.Input() <- &sarama.ProducerMessage{
			Topic:     topic,
			Key:       sarama.StringEncoder(message.Key),
			Value:     sarama.ByteEncoder(message.Value),
			Timestamp: sentAt,
			Metadata:  sentAt,
			Headers: []sarama.RecordHeader{
				{Key: []byte("event_id"), Value: []byte(message.EventID)},
				{Key: []byte("schema_version"), Value: []byte("1")},
				{Key: []byte("run_id"), Value: []byte(config.RunID)},
				{Key: []byte("phase"), Value: []byte(config.Phase)},
			},
		}
		sent++
	}
	producer.AsyncClose()
	collector.Wait()
	if acknowledgements.firstErr != nil {
		return acknowledgements.firstErr
	}
	if int64(len(acknowledgements.latencies)) != sent {
		return fmt.Errorf("Kafka acknowledged %d of %d scale events", len(acknowledgements.latencies), sent)
	}
	duration := time.Since(startedAt)
	p95, err := scale.PercentileMilliseconds(acknowledgements.latencies, 0.95)
	if err != nil {
		return err
	}
	logger.Info(
		"load scenario completed",
		"run_id", config.RunID,
		"phase", config.Phase,
		"messages", sent,
		"duration_milliseconds", float64(duration)/float64(time.Millisecond),
		"throughput_events_per_second", float64(sent)/duration.Seconds(),
		"ack_p95_milliseconds", p95,
	)
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
