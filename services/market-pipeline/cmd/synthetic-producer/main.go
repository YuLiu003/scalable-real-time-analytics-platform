package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/kafkaclient"
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
	var messages []outboundMessage
	switch scenario {
	case "baseline":
		first := fixture("QQQ", "600.0000", 1, "11111111111111111111111111111111", "2026-07-21T00:00:00Z")
		second := fixture("QQQM", "250.0000", 2, "22222222222222222222222222222222", "2026-07-21T00:00:02Z")
		firstBytes, err := first.Marshal()
		if err != nil {
			return err
		}
		secondBytes, err := second.Marshal()
		if err != nil {
			return err
		}
		messages = []outboundMessage{
			{key: first.PartitionKey, eventID: first.EventID, value: firstBytes},
			{key: second.PartitionKey, eventID: second.EventID, value: secondBytes},
			{key: first.PartitionKey, eventID: first.EventID, value: firstBytes},
			{
				key:     "INVALID",
				eventID: "synthetic:malformed:001",
				value:   []byte(`{"event_id":"synthetic:malformed:001","event_type":"market.price.observed","schema_version":1,"unexpected":true}`),
			},
		}
	case "single":
		instrument := envOrDefault("EVENT_INSTRUMENT", "FSELX")
		price := envOrDefault("EVENT_PRICE", "60.0000")
		sequence, err := strconv.ParseInt(envOrDefault("EVENT_SEQUENCE", "100"), 10, 64)
		if err != nil {
			return fmt.Errorf("parse EVENT_SEQUENCE: %w", err)
		}
		envelope := fixture(
			instrument,
			price,
			sequence,
			envOrDefault("EVENT_TRACE_ID", "33333333333333333333333333333333"),
			envOrDefault("EVENT_OCCURRED_AT", "2026-07-21T00:01:00Z"),
		)
		if configuredID := os.Getenv("EVENT_ID"); configuredID != "" {
			envelope.EventID = configuredID
		}
		encoded, err := envelope.Marshal()
		if err != nil {
			return err
		}
		messages = []outboundMessage{{key: envelope.PartitionKey, eventID: envelope.EventID, value: encoded}}
	default:
		return fmt.Errorf("unknown PRODUCER_SCENARIO %q", scenario)
	}

	for index, message := range messages {
		partition, offset, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(message.key),
			Value: sarama.ByteEncoder(message.value),
			Headers: []sarama.RecordHeader{
				{Key: []byte("event_id"), Value: []byte(message.eventID)},
				{Key: []byte("schema_version"), Value: []byte("1")},
			},
		})
		if err != nil {
			return fmt.Errorf("send event %s: %w", message.eventID, err)
		}
		logger.Info("broker acknowledged event", "event_id", message.eventID, "topic", topic, "partition", partition, "offset", offset)
		if index == 0 && envOrDefault("FAIL_AFTER_FIRST_ACK", "false") == "true" {
			return errors.New("injected producer failure after broker acknowledgement")
		}
	}
	logger.Info("producer scenario completed", "scenario", scenario, "messages", len(messages))
	return nil
}

type outboundMessage struct {
	key     string
	eventID string
	value   []byte
}

func fixture(instrument, price string, sequence int64, traceID, occurredAt string) event.Envelope {
	parsedTime, _ := time.Parse(time.RFC3339Nano, occurredAt)
	compactTime := strings.ToLower(parsedTime.UTC().Format("20060102t150405z"))
	return event.Envelope{
		EventID:       fmt.Sprintf("synthetic:price:%s:%s", strings.ToLower(instrument), compactTime),
		EventType:     event.MarketPriceObservedType,
		SchemaVersion: event.MarketPriceSchemaVersion,
		Source:        "synthetic",
		TenantID:      "demo",
		OccurredAt:    occurredAt,
		IngestedAt:    parsedTime.Add(time.Second).UTC().Format(time.RFC3339Nano),
		PartitionKey:  instrument,
		TraceID:       traceID,
		Payload: event.PriceObservedPayload{
			Instrument:       instrument,
			Currency:         "USD",
			Price:            price,
			ProviderSequence: sequence,
		},
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
