package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/alpaca"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		logger.Error("private market feed stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	config, err := alpaca.LoadConfigFromEnvironment()
	if err != nil {
		return err
	}
	kafkaSettings, err := kafkaclient.FromEnvironment()
	if err != nil {
		return err
	}
	producerConfig, err := kafkaSettings.Producer("alpaca-market-producer")
	if err != nil {
		return err
	}
	producer, err := sarama.NewSyncProducer(kafkaSettings.Brokers, producerConfig)
	if err != nil {
		return fmt.Errorf("create Kafka producer: %w", err)
	}
	defer producer.Close()

	streamHTTPClient := alpaca.NewProviderHTTPClient(0)
	historyHTTPClient := alpaca.NewProviderHTTPClient(30 * time.Second)
	status := alpaca.NewStatus()
	runner := alpaca.NewRunner(
		config,
		alpaca.StreamClient{URL: config.StreamURL, KeyID: config.KeyID, SecretKey: config.SecretKey, HTTP: streamHTTPClient},
		alpaca.HistoryClient{URL: config.HistoryURL, Feed: config.Feed, KeyID: config.KeyID, SecretKey: config.SecretKey, HTTP: historyHTTPClient},
		kafkaPublisher{producer: producer, topic: config.Topic},
		alpaca.NewFileCheckpoints(config.CheckpointFile, alpaca.CheckpointScope(config)),
		status,
	)
	server := &http.Server{
		Addr:              config.HTTPAddress,
		Handler:           status.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	runnerResult := make(chan error, 1)
	serverResult := make(chan error, 1)
	go func() { runnerResult <- runner.Run(ctx) }()
	go func() { serverResult <- server.ListenAndServe() }()

	var result error
	select {
	case result = <-runnerResult:
	case result = <-serverResult:
		if errors.Is(result, http.ErrServerClosed) {
			result = nil
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil && result == nil {
		result = fmt.Errorf("shut down health server: %w", err)
	}
	return result
}

type kafkaPublisher struct {
	producer messageProducer
	topic    string
}

type messageProducer interface {
	SendMessage(*sarama.ProducerMessage) (partition int32, offset int64, err error)
}

func (publisher kafkaPublisher) Publish(ctx context.Context, envelope event.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := envelope.Marshal()
	if err != nil {
		return err
	}
	_, _, err = publisher.producer.SendMessage(&sarama.ProducerMessage{
		Topic: publisher.topic,
		Key:   sarama.StringEncoder(envelope.PartitionKey),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_id"), Value: []byte(envelope.EventID)},
			{Key: []byte("schema_version"), Value: []byte("1")},
		},
	})
	if err != nil {
		return fmt.Errorf("publish canonical market event: %w", err)
	}
	return nil
}
