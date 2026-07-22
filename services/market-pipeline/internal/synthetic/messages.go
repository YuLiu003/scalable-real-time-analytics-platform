package synthetic

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

// Message is one deterministic producer fixture ready for Kafka encoding.
type Message struct {
	Key     string
	EventID string
	Value   []byte
}

// Messages builds a supported synthetic producer scenario from its documented
// environment variables. Keeping fixture construction separate from Kafka I/O
// makes the source contract fully unit-testable.
func Messages(scenario string) ([]Message, error) {
	switch scenario {
	case "baseline":
		first := fixture("QQQ", "600.0000", 1, "11111111111111111111111111111111", "2026-07-21T00:00:00Z")
		second := fixture("QQQM", "250.0000", 2, "22222222222222222222222222222222", "2026-07-21T00:00:02Z")
		return baselineMessages(first, second)
	case "portfolio-complete":
		return validMessages(
			fixture("FSELX", "60.0000", 100, "33333333333333333333333333333333", "2026-07-21T00:01:00Z"),
			fixture("SP500", "6500.0000", 101, "44444444444444444444444444444444", "2026-07-21T00:01:30Z"),
		)
	case "single":
		instrument := envOrDefault("EVENT_INSTRUMENT", "FSELX")
		price := envOrDefault("EVENT_PRICE", "60.0000")
		sequence, err := strconv.ParseInt(envOrDefault("EVENT_SEQUENCE", "100"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse EVENT_SEQUENCE: %w", err)
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
			return nil, err
		}
		return []Message{{Key: envelope.PartitionKey, EventID: envelope.EventID, Value: encoded}}, nil
	default:
		return nil, fmt.Errorf("unknown PRODUCER_SCENARIO %q", scenario)
	}
}

func baselineMessages(first, second event.Envelope) ([]Message, error) {
	messages, err := validMessages(first, second)
	if err != nil {
		return nil, err
	}
	return append(messages, []Message{
		{Key: first.PartitionKey, EventID: first.EventID, Value: messages[0].Value},
		{
			Key:     "INVALID",
			EventID: "synthetic:malformed:001",
			Value:   []byte(`{"event_id":"synthetic:malformed:001","event_type":"market.price.observed","schema_version":1,"unexpected":true}`),
		},
	}...), nil
}

func validMessages(envelopes ...event.Envelope) ([]Message, error) {
	messages := make([]Message, 0, len(envelopes))
	for _, envelope := range envelopes {
		encoded, err := envelope.Marshal()
		if err != nil {
			return nil, err
		}
		messages = append(messages, Message{Key: envelope.PartitionKey, EventID: envelope.EventID, Value: encoded})
	}
	return messages, nil
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
