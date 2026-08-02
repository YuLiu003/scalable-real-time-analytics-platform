package synthetic

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

func TestBaselineMessagesEncodeRequestedFundsAndFailureFixtures(t *testing.T) {
	messages, err := Messages("baseline")
	if err != nil {
		t.Fatalf("Messages() error = %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("message count = %d, want 4", len(messages))
	}
	if messages[0].Key != "DEMO-ASSET-A" || messages[1].Key != "DEMO-ASSET-B" || messages[2].EventID != messages[0].EventID {
		t.Fatalf("unexpected baseline identity: %+v", messages)
	}
	var first event.Envelope
	if err := json.Unmarshal(messages[0].Value, &first); err != nil {
		t.Fatalf("decode first fixture: %v", err)
	}
	if first.Payload.Price != "100.0000" || messages[3].Key != "INVALID" {
		t.Fatalf("unexpected baseline fixtures: %+v %+v", first, messages[3])
	}
}

func TestBaselineMessagesPropagatesEitherEncodingError(t *testing.T) {
	valid := fixture("DEMO-ASSET-A", "100.0000", 1, strings.Repeat("1", 32), "2026-07-21T00:00:00Z")
	invalid := valid
	invalid.Payload.Price = "invalid"
	if _, err := baselineMessages(invalid, valid); err == nil {
		t.Fatal("baselineMessages() accepted an invalid first fixture")
	}
	if _, err := baselineMessages(valid, invalid); err == nil {
		t.Fatal("baselineMessages() accepted an invalid second fixture")
	}
}

func TestPortfolioCompleteMessagesSupplyAnalyticsInputs(t *testing.T) {
	messages, err := Messages("portfolio-complete")
	if err != nil {
		t.Fatalf("Messages(portfolio-complete) error = %v", err)
	}
	if len(messages) != 2 || messages[0].Key != "DEMO-ASSET-C" || messages[1].Key != "DEMO-BENCH-D" {
		t.Fatalf("portfolio-complete messages = %+v", messages)
	}
}

func TestSingleMessageDefaultsAndOverrides(t *testing.T) {
	message, err := Messages("single")
	if err != nil {
		t.Fatalf("Messages(default single) error = %v", err)
	}
	if len(message) != 1 || message[0].Key != "DEMO-ASSET-C" {
		t.Fatalf("default single = %+v", message)
	}

	t.Setenv("EVENT_INSTRUMENT", "DEMO-BENCH-D")
	t.Setenv("EVENT_PRICE", "1000.0000")
	t.Setenv("EVENT_SEQUENCE", "101")
	t.Setenv("EVENT_TRACE_ID", strings.Repeat("4", 32))
	t.Setenv("EVENT_OCCURRED_AT", "2026-07-21T00:01:30Z")
	t.Setenv("EVENT_ID", "synthetic:price:demo-bench-d:override")
	message, err = Messages("single")
	if err != nil {
		t.Fatalf("Messages(overridden single) error = %v", err)
	}
	if message[0].Key != "DEMO-BENCH-D" || message[0].EventID != "synthetic:price:demo-bench-d:override" {
		t.Fatalf("overridden single = %+v", message[0])
	}
}

func TestMessagesRejectsInvalidConfiguration(t *testing.T) {
	t.Run("sequence", func(t *testing.T) {
		t.Setenv("EVENT_SEQUENCE", "not-an-integer")
		if _, err := Messages("single"); err == nil || !strings.Contains(err.Error(), "parse EVENT_SEQUENCE") {
			t.Fatalf("Messages() error = %v", err)
		}
	})
	t.Run("envelope", func(t *testing.T) {
		t.Setenv("EVENT_PRICE", "invalid")
		if _, err := Messages("single"); err == nil {
			t.Fatal("Messages() accepted an invalid price")
		}
	})
	t.Run("scenario", func(t *testing.T) {
		if _, err := Messages("unknown"); err == nil || !strings.Contains(err.Error(), "unknown PRODUCER_SCENARIO") {
			t.Fatalf("Messages() error = %v", err)
		}
	})
}

func TestEnvOrDefault(t *testing.T) {
	if got := envOrDefault("UNSET_SYNTHETIC_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault() = %q", got)
	}
	t.Setenv("SET_SYNTHETIC_TEST_VALUE", "configured")
	if got := envOrDefault("SET_SYNTHETIC_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("envOrDefault() = %q", got)
	}
}

func TestLoadConfigFromEnvironmentDefaultsAndOverrides(t *testing.T) {
	for _, name := range []string{
		"LOAD_RUN_ID", "LOAD_PHASE", "LOAD_EVENT_COUNT", "LOAD_INSTRUMENTS",
		"LOAD_BASE_TIME", "LOAD_TARGET_RATE",
	} {
		t.Setenv(name, "")
	}
	config, err := LoadConfigFromEnvironment()
	if err != nil {
		t.Fatalf("LoadConfigFromEnvironment() error = %v", err)
	}
	if config.RunID != "local-scale" || config.Phase != "original" || config.EventCount != 1000 ||
		config.TargetRate != 0 || !reflect.DeepEqual(config.Instruments, []string{"LOAD-A", "LOAD-B", "LOAD-C"}) {
		t.Fatalf("default load config = %+v", config)
	}

	t.Setenv("LOAD_RUN_ID", "load-42")
	t.Setenv("LOAD_PHASE", "replay")
	t.Setenv("LOAD_EVENT_COUNT", "12")
	t.Setenv("LOAD_INSTRUMENTS", " SCALE-A, ,SCALE-B ")
	t.Setenv("LOAD_BASE_TIME", "2026-08-01T01:02:03.004Z")
	t.Setenv("LOAD_TARGET_RATE", "500")
	config, err = LoadConfigFromEnvironment()
	if err != nil {
		t.Fatalf("LoadConfigFromEnvironment(overrides) error = %v", err)
	}
	if config.RunID != "load-42" || config.Phase != "replay" || config.EventCount != 12 ||
		config.TargetRate != 500 || !reflect.DeepEqual(config.Instruments, []string{"SCALE-A", "SCALE-B"}) ||
		config.BaseTime.Format(time.RFC3339Nano) != "2026-08-01T01:02:03.004Z" {
		t.Fatalf("overridden load config = %+v", config)
	}
}

func TestLoadConfigFromEnvironmentRejectsParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    string
		want     string
	}{
		{name: "count", variable: "LOAD_EVENT_COUNT", value: "many", want: "parse LOAD_EVENT_COUNT"},
		{name: "rate", variable: "LOAD_TARGET_RATE", value: "fast", want: "parse LOAD_TARGET_RATE"},
		{name: "base time", variable: "LOAD_BASE_TIME", value: "soon", want: "parse LOAD_BASE_TIME"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.variable, tt.value)
			if _, err := LoadConfigFromEnvironment(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadConfigFromEnvironment() error = %v", err)
			}
		})
	}

	t.Setenv("LOAD_PHASE", "invalid")
	if _, err := LoadConfigFromEnvironment(); err == nil || !strings.Contains(err.Error(), "LOAD_PHASE") {
		t.Fatalf("LoadConfigFromEnvironment() validation error = %v", err)
	}
}

func TestLoadConfigValidateRejectsEveryInvalidClass(t *testing.T) {
	valid := LoadConfig{
		RunID:       "load-1",
		Phase:       "original",
		EventCount:  3,
		Instruments: []string{"LOAD-A", "LOAD-B"},
		BaseTime:    time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name   string
		mutate func(*LoadConfig)
		want   string
	}{
		{name: "run ID", mutate: func(c *LoadConfig) { c.RunID = "PRIVATE_VALUE" }, want: "LOAD_RUN_ID"},
		{name: "phase", mutate: func(c *LoadConfig) { c.Phase = "third" }, want: "LOAD_PHASE"},
		{name: "count", mutate: func(c *LoadConfig) { c.EventCount = MaxLoadEvents + 1 }, want: "LOAD_EVENT_COUNT"},
		{name: "rate", mutate: func(c *LoadConfig) { c.TargetRate = -1 }, want: "LOAD_TARGET_RATE"},
		{name: "time", mutate: func(c *LoadConfig) { c.BaseTime = time.Time{} }, want: "LOAD_BASE_TIME"},
		{name: "instrument count", mutate: func(c *LoadConfig) { c.Instruments = nil }, want: "LOAD_INSTRUMENTS"},
		{name: "duplicate instrument", mutate: func(c *LoadConfig) { c.Instruments = []string{"LOAD-A", "LOAD-A"} }, want: "duplicate"},
		{name: "invalid instrument", mutate: func(c *LoadConfig) { c.Instruments = []string{"private-value"} }, want: "invalid LOAD_INSTRUMENTS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := valid
			tt.mutate(&config)
			if err := config.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
}

func TestLoadGeneratorStreamsDeterministicReplay(t *testing.T) {
	config := LoadConfig{
		RunID:       "scale-proof",
		Phase:       "original",
		EventCount:  4,
		Instruments: []string{"LOAD-A", "LOAD-B"},
		BaseTime:    time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	}
	original, err := NewLoadGenerator(config)
	if err != nil {
		t.Fatalf("NewLoadGenerator() error = %v", err)
	}
	config.Phase = "replay"
	replay, err := NewLoadGenerator(config)
	if err != nil {
		t.Fatalf("NewLoadGenerator(replay) error = %v", err)
	}
	for index := int64(0); index < config.EventCount; index++ {
		first, ok, err := original.Next()
		if err != nil || !ok {
			t.Fatalf("original.Next() = %+v, %v, %v", first, ok, err)
		}
		second, ok, err := replay.Next()
		if err != nil || !ok {
			t.Fatalf("replay.Next() = %+v, %v, %v", second, ok, err)
		}
		if first.Key != config.Instruments[index%2] || first.EventID != second.EventID || !bytes.Equal(first.Value, second.Value) {
			t.Fatalf("messages differ at %d: %+v %+v", index, first, second)
		}
		var envelope event.Envelope
		if err := json.Unmarshal(first.Value, &envelope); err != nil {
			t.Fatalf("decode load event: %v", err)
		}
		if envelope.Payload.ProviderSequence != index/2+1 || envelope.Source != "scale.scale-proof" {
			t.Fatalf("load envelope = %+v", envelope)
		}
	}
	if message, ok, err := original.Next(); err != nil || ok || !reflect.DeepEqual(message, Message{}) {
		t.Fatalf("exhausted Next() = %+v, %v, %v", message, ok, err)
	}
}

func TestLoadGeneratorRejectsInvalidConfigAndEncodingFailure(t *testing.T) {
	if _, err := NewLoadGenerator(LoadConfig{}); err == nil {
		t.Fatal("NewLoadGenerator() accepted invalid config")
	}
	config := LoadConfig{
		RunID:       "load-1",
		Phase:       "original",
		EventCount:  1,
		Instruments: []string{"LOAD-A"},
		BaseTime:    time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	}
	generator, err := NewLoadGenerator(config)
	if err != nil {
		t.Fatal(err)
	}
	generator.config.Instruments[0] = "invalid"
	if _, ok, err := generator.Next(); err == nil || ok {
		t.Fatalf("Next() = ok %v, error %v", ok, err)
	}
}
