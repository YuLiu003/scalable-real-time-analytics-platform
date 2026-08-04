package synthetic

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
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

const MaxLoadEvents int64 = 1_000_000

var loadRunIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,25}$`)

// LoadConfig defines a deterministic, non-portfolio traffic run. Reusing the
// same configuration produces byte-identical event values for replay tests.
type LoadConfig struct {
	RunID       string
	Phase       string
	EventCount  int64
	Instruments []string
	BaseTime    time.Time
	TargetRate  int64
}

// LoadGenerator emits one message at a time so large runs do not accumulate
// every encoded event in memory.
type LoadGenerator struct {
	config LoadConfig
	index  int64
}

// Messages builds a supported synthetic producer scenario from its documented
// environment variables. Keeping fixture construction separate from Kafka I/O
// makes the source contract fully unit-testable.
func Messages(scenario string) ([]Message, error) {
	switch scenario {
	case "baseline":
		first := fixture("DEMO-ASSET-A", "100.0000", 1, "11111111111111111111111111111111", "2026-07-21T00:00:00Z")
		second := fixture("DEMO-ASSET-B", "100.0000", 2, "22222222222222222222222222222222", "2026-07-21T00:00:02Z")
		return baselineMessages(first, second)
	case "portfolio-complete":
		return validMessages(
			fixture("DEMO-ASSET-C", "100.0000", 100, "33333333333333333333333333333333", "2026-07-21T00:01:00Z"),
			fixture("DEMO-BENCH-D", "1000.0000", 101, "44444444444444444444444444444444", "2026-07-21T00:01:30Z"),
		)
	case "private-portfolio-acceptance":
		return privatePortfolioMessages()
	case "single":
		instrument := envOrDefault("EVENT_INSTRUMENT", "DEMO-ASSET-C")
		price := envOrDefault("EVENT_PRICE", "100.0000")
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

func privatePortfolioMessages() ([]Message, error) {
	fixtures := []event.Envelope{
		fixture("DEMO-LIVE-A", "100.0000", 201, "55555555555555555555555555555555", "2026-08-03T16:00:00Z"),
		fixture("DEMO-LIVE-B", "200.0000", 202, "66666666666666666666666666666666", "2026-08-03T16:00:01Z"),
		fixture("DEMO-LIVE-C", "500.0000", 203, "77777777777777777777777777777777", "2026-08-03T16:00:02Z"),
	}
	for index := range fixtures {
		fixtures[index].Source = "private-acceptance"
		fixtures[index].TenantID = "private"
		fixtures[index].EventID = fmt.Sprintf("acceptance:private:%s", strings.ToLower(fixtures[index].PartitionKey))
	}
	return validMessages(fixtures...)
}

// LoadConfigFromEnvironment reads the bounded public scale controls. Instrument
// values are runtime input and must remain synthetic in committed manifests.
func LoadConfigFromEnvironment() (LoadConfig, error) {
	count, err := strconv.ParseInt(envOrDefault("LOAD_EVENT_COUNT", "1000"), 10, 64)
	if err != nil {
		return LoadConfig{}, fmt.Errorf("parse LOAD_EVENT_COUNT: %w", err)
	}
	rate, err := strconv.ParseInt(envOrDefault("LOAD_TARGET_RATE", "0"), 10, 64)
	if err != nil {
		return LoadConfig{}, fmt.Errorf("parse LOAD_TARGET_RATE: %w", err)
	}
	baseTime, err := time.Parse(time.RFC3339Nano, envOrDefault("LOAD_BASE_TIME", "2026-07-30T00:00:00Z"))
	if err != nil {
		return LoadConfig{}, fmt.Errorf("parse LOAD_BASE_TIME: %w", err)
	}
	config := LoadConfig{
		RunID:       envOrDefault("LOAD_RUN_ID", "local-scale"),
		Phase:       envOrDefault("LOAD_PHASE", "original"),
		EventCount:  count,
		Instruments: splitNonEmpty(envOrDefault("LOAD_INSTRUMENTS", "LOAD-A,LOAD-B,LOAD-C")),
		BaseTime:    baseTime.UTC(),
		TargetRate:  rate,
	}
	if err := config.Validate(); err != nil {
		return LoadConfig{}, err
	}
	return config, nil
}

// Validate protects local resources and prevents accidental use of a personal
// watchlist in the source-controlled scale path.
func (c LoadConfig) Validate() error {
	if !loadRunIDPattern.MatchString(c.RunID) {
		return errors.New("LOAD_RUN_ID must be 1-26 lowercase letters, digits, or hyphens")
	}
	if c.Phase != "original" && c.Phase != "replay" {
		return errors.New(`LOAD_PHASE must be "original" or "replay"`)
	}
	if c.EventCount < 1 || c.EventCount > MaxLoadEvents {
		return fmt.Errorf("LOAD_EVENT_COUNT must be between 1 and %d", MaxLoadEvents)
	}
	if c.TargetRate < 0 || c.TargetRate > 100_000 {
		return errors.New("LOAD_TARGET_RATE must be between 0 and 100000")
	}
	if c.BaseTime.IsZero() {
		return errors.New("LOAD_BASE_TIME must not be zero")
	}
	if len(c.Instruments) < 1 || len(c.Instruments) > 100 {
		return errors.New("LOAD_INSTRUMENTS must contain between 1 and 100 values")
	}
	seen := make(map[string]struct{}, len(c.Instruments))
	for _, instrument := range c.Instruments {
		if _, exists := seen[instrument]; exists {
			return fmt.Errorf("LOAD_INSTRUMENTS contains duplicate %q", instrument)
		}
		seen[instrument] = struct{}{}
		if err := loadEnvelope(c, 0, instrument, 1).Validate(); err != nil {
			return fmt.Errorf("invalid LOAD_INSTRUMENTS value %q: %w", instrument, err)
		}
	}
	return nil
}

// NewLoadGenerator validates config once before any Kafka writes occur.
func NewLoadGenerator(config LoadConfig) (*LoadGenerator, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &LoadGenerator{config: config}, nil
}

// Next returns the next deterministic message and false after EventCount.
func (g *LoadGenerator) Next() (Message, bool, error) {
	if g.index >= g.config.EventCount {
		return Message{}, false, nil
	}
	instrumentIndex := g.index % int64(len(g.config.Instruments))
	instrument := g.config.Instruments[instrumentIndex]
	sequence := g.index/int64(len(g.config.Instruments)) + 1
	envelope := loadEnvelope(g.config, g.index, instrument, sequence)
	encoded, err := envelope.Marshal()
	if err != nil {
		return Message{}, false, err
	}
	g.index++
	return Message{Key: instrument, EventID: envelope.EventID, Value: encoded}, true, nil
}

func loadEnvelope(config LoadConfig, index int64, instrument string, sequence int64) event.Envelope {
	occurredAt := config.BaseTime.Add(time.Duration(index) * time.Millisecond)
	traceSeed := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%d", config.RunID, instrument, sequence)))
	return event.Envelope{
		EventID:       fmt.Sprintf("scale:%s:%s:%09d", config.RunID, strings.ToLower(instrument), sequence),
		EventType:     event.MarketPriceObservedType,
		SchemaVersion: event.MarketPriceSchemaVersion,
		Source:        "scale." + config.RunID,
		TenantID:      "load-test",
		OccurredAt:    occurredAt.Format(time.RFC3339Nano),
		IngestedAt:    occurredAt.Add(time.Millisecond).Format(time.RFC3339Nano),
		PartitionKey:  instrument,
		TraceID:       hex.EncodeToString(traceSeed[:16]),
		Payload: event.PriceObservedPayload{
			Instrument:       instrument,
			Currency:         "USD",
			Price:            "100.0000",
			ProviderSequence: sequence,
		},
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

func splitNonEmpty(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
