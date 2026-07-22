package synthetic

import (
	"encoding/json"
	"strings"
	"testing"

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
	if messages[0].Key != "QQQ" || messages[1].Key != "QQQM" || messages[2].EventID != messages[0].EventID {
		t.Fatalf("unexpected baseline identity: %+v", messages)
	}
	var first event.Envelope
	if err := json.Unmarshal(messages[0].Value, &first); err != nil {
		t.Fatalf("decode first fixture: %v", err)
	}
	if first.Payload.Price != "600.0000" || messages[3].Key != "INVALID" {
		t.Fatalf("unexpected baseline fixtures: %+v %+v", first, messages[3])
	}
}

func TestBaselineMessagesPropagatesEitherEncodingError(t *testing.T) {
	valid := fixture("QQQ", "600.0000", 1, strings.Repeat("1", 32), "2026-07-21T00:00:00Z")
	invalid := valid
	invalid.Payload.Price = "invalid"
	if _, err := baselineMessages(invalid, valid); err == nil {
		t.Fatal("baselineMessages() accepted an invalid first fixture")
	}
	if _, err := baselineMessages(valid, invalid); err == nil {
		t.Fatal("baselineMessages() accepted an invalid second fixture")
	}
}

func TestSingleMessageDefaultsAndOverrides(t *testing.T) {
	message, err := Messages("single")
	if err != nil {
		t.Fatalf("Messages(default single) error = %v", err)
	}
	if len(message) != 1 || message[0].Key != "FSELX" {
		t.Fatalf("default single = %+v", message)
	}

	t.Setenv("EVENT_INSTRUMENT", "SP500")
	t.Setenv("EVENT_PRICE", "6500.0000")
	t.Setenv("EVENT_SEQUENCE", "101")
	t.Setenv("EVENT_TRACE_ID", strings.Repeat("4", 32))
	t.Setenv("EVENT_OCCURRED_AT", "2026-07-21T00:01:30Z")
	t.Setenv("EVENT_ID", "synthetic:price:sp500:override")
	message, err = Messages("single")
	if err != nil {
		t.Fatalf("Messages(overridden single) error = %v", err)
	}
	if message[0].Key != "SP500" || message[0].EventID != "synthetic:price:sp500:override" {
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
