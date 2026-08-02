package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/archive"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/archivemetrics"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("archiver stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	kafkaSettings, err := kafkaclient.FromEnvironment()
	if err != nil {
		return err
	}
	consumerConfig, err := kafkaSettings.Consumer("raw-event-archiver")
	if err != nil {
		return err
	}
	producerConfig, err := kafkaSettings.Producer("raw-event-archiver-quarantine")
	if err != nil {
		return err
	}
	consumer, err := sarama.NewConsumerGroup(kafkaSettings.Brokers, envOrDefault("KAFKA_CONSUMER_GROUP", "raw-event-archiver-v1"), consumerConfig)
	if err != nil {
		return fmt.Errorf("create Kafka consumer group: %w", err)
	}
	defer consumer.Close()
	quarantineProducer, err := sarama.NewSyncProducer(kafkaSettings.Brokers, producerConfig)
	if err != nil {
		return fmt.Errorf("create quarantine producer: %w", err)
	}
	defer quarantineProducer.Close()

	store, err := archive.New(ctx, archive.Settings{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		Region:    os.Getenv("AWS_REGION"),
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	})
	if err != nil {
		return err
	}
	metrics, err := archivemetrics.New(envOrDefault("ARCHIVER_METRICS_SCOPE", "baseline"))
	if err != nil {
		return err
	}
	logEvery, err := strconv.ParseUint(envOrDefault("ARCHIVER_LOG_EVERY", "1"), 10, 64)
	if err != nil || logEvery == 0 {
		return errors.New("ARCHIVER_LOG_EVERY must be a positive integer")
	}
	metricsListener, err := net.Listen("tcp", envOrDefault("METRICS_ADDR", ":8081"))
	if err != nil {
		return fmt.Errorf("listen for metrics: %w", err)
	}
	metricsServer := &http.Server{
		Handler:           metrics.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	metricsErrors := make(chan error, 1)
	go func() {
		if err := metricsServer.Serve(metricsListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			metricsErrors <- fmt.Errorf("serve metrics: %w", err)
			cancel()
		}
	}()
	defer metricsServer.Shutdown(context.Background())

	delay, err := time.ParseDuration(envOrDefault("ARCHIVER_POST_WRITE_DELAY", "0s"))
	if err != nil {
		return fmt.Errorf("parse ARCHIVER_POST_WRITE_DELAY: %w", err)
	}
	handler := &consumerHandler{
		store:           store,
		quarantine:      quarantineProducer,
		quarantineTopic: envOrDefault("KAFKA_QUARANTINE_TOPIC", "ingestion.quarantine"),
		postWriteDelay:  delay,
		logger:          logger,
		metrics:         metrics,
		logEvery:        logEvery,
	}

	go func() {
		for consumerError := range consumer.Errors() {
			logger.Error("Kafka consumer error", "error", consumerError)
		}
	}()

	topic := envOrDefault("KAFKA_TOPIC", "market.prices")
	logger.Info("archiver started", "topic", topic, "consumer_group", envOrDefault("KAFKA_CONSUMER_GROUP", "raw-event-archiver-v1"))
	for runContext.Err() == nil {
		if err := consumer.Consume(runContext, []string{topic}, handler); err != nil {
			if runContext.Err() != nil {
				break
			}
			logger.Error("consumer session failed", "error", err)
			time.Sleep(time.Second)
		}
	}
	select {
	case err := <-metricsErrors:
		return err
	default:
		return runContext.Err()
	}
}

type consumerHandler struct {
	store           *archive.Store
	quarantine      sarama.SyncProducer
	quarantineTopic string
	postWriteDelay  time.Duration
	logger          *slog.Logger
	metrics         *archivemetrics.Recorder
	logEvery        uint64
	processed       atomic.Uint64
}

func (*consumerHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (*consumerHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return session.Context().Err()
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			startedAt := time.Now()
			outcome, err := h.process(session.Context(), message)
			processing := time.Since(startedAt)
			durable := processing
			if !message.Timestamp.IsZero() {
				durable = time.Since(message.Timestamp)
				if durable < 0 {
					durable = processing
				}
			}
			if err != nil {
				_ = h.metrics.Observe("error", processing, durable)
				return err
			}
			_ = h.metrics.Observe(outcome, processing, durable)
			session.MarkMessage(message, "archive effect durable")
		}
	}
}

func (h *consumerHandler) process(ctx context.Context, message *sarama.ConsumerMessage) (string, error) {
	envelope, err := event.DecodeStrict(message.Value)
	if err != nil {
		return "quarantined", h.sendToQuarantine(message, "schema_validation_failed", err.Error())
	}
	result, _, err := h.store.PutEvent(ctx, envelope, message.Value)
	if errors.Is(err, archive.ErrEventIDCollision) {
		return "quarantined", h.sendToQuarantine(message, "event_id_collision", err.Error())
	}
	if err != nil {
		return "", err
	}
	if h.processed.Add(1)%h.logEvery == 0 {
		h.logger.Info("archive effect durable", "event_ref", event.IDReference(envelope.EventID), "result", result, "partition", message.Partition, "offset", message.Offset)
	}
	if result == archive.Created && h.postWriteDelay > 0 {
		h.logger.Info("post-write failure window open", "event_ref", event.IDReference(envelope.EventID), "delay", h.postWriteDelay.String())
		timer := time.NewTimer(h.postWriteDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return string(result), nil
}

type quarantineRecord struct {
	SourceTopic   string `json:"source_topic"`
	Partition     int32  `json:"partition"`
	Offset        int64  `json:"offset"`
	Reason        string `json:"reason"`
	Detail        string `json:"detail"`
	PayloadBase64 string `json:"payload_base64"`
}

func (h *consumerHandler) sendToQuarantine(message *sarama.ConsumerMessage, reason, detail string) error {
	record := quarantineRecord{
		SourceTopic:   message.Topic,
		Partition:     message.Partition,
		Offset:        message.Offset,
		Reason:        reason,
		Detail:        detail,
		PayloadBase64: base64.StdEncoding.EncodeToString(message.Value),
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	partition, offset, err := h.quarantine.SendMessage(&sarama.ProducerMessage{
		Topic: h.quarantineTopic,
		Key:   sarama.StringEncoder(fmt.Sprintf("%s:%d:%d", message.Topic, message.Partition, message.Offset)),
		Value: sarama.ByteEncoder(encoded),
	})
	if err != nil {
		return fmt.Errorf("write quarantine record: %w", err)
	}
	h.logger.Warn("record quarantined", "reason", reason, "source_topic", message.Topic, "source_partition", message.Partition, "source_offset", message.Offset, "quarantine_partition", partition, "quarantine_offset", offset)
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
