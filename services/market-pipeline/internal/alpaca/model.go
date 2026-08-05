package alpaca

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

var providerPrice = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,8})?$`)

// Bar is the provider-neutral subset of an Alpaca one-minute bar needed by the
// canonical market-price event.
type Bar struct {
	Instrument string
	Timestamp  time.Time
	Close      string
	TradeCount int64
}

func (b Bar) Validate() error {
	if !event.ValidInstrument(b.Instrument) {
		return errors.New("provider bar contains an invalid instrument")
	}
	if b.Timestamp.IsZero() {
		return errors.New("provider bar contains an invalid timestamp")
	}
	if !providerPrice.MatchString(b.Close) || strings.TrimRight(b.Close, "0.") == "" {
		return errors.New("provider bar contains an invalid positive close price")
	}
	price, err := strconv.ParseFloat(b.Close, 64)
	if err != nil || price <= 0 {
		return errors.New("provider bar contains an invalid positive close price")
	}
	if b.TradeCount < 0 {
		return errors.New("provider bar contains a negative trade count")
	}
	return nil
}

// Envelope converts a one-minute close into deterministic canonical bytes.
// Alpaca timestamps bars at interval start, so the canonical observation time
// is the interval end. Using that deterministic boundary for both timestamps
// keeps retries and historical backfills byte-identical.
func (b Bar) Envelope(source, tenant string) (event.Envelope, error) {
	if err := b.Validate(); err != nil {
		return event.Envelope{}, err
	}
	closePrice := canonicalPrice(b.Close)
	identity := strings.Join([]string{
		source,
		tenant,
		b.Instrument,
		b.Timestamp.UTC().Format(time.RFC3339Nano),
		strconv.FormatInt(b.TradeCount, 10),
		closePrice,
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	eventID := "alpaca:bar:" + hex.EncodeToString(digest[:])
	observedAt := b.Timestamp.UTC().Add(time.Minute).Format(time.RFC3339Nano)
	envelope := event.Envelope{
		EventID:       eventID,
		EventType:     event.MarketPriceObservedType,
		SchemaVersion: event.MarketPriceSchemaVersion,
		Source:        source,
		TenantID:      tenant,
		OccurredAt:    observedAt,
		IngestedAt:    observedAt,
		PartitionKey:  b.Instrument,
		TraceID:       hex.EncodeToString(digest[:16]),
		Payload: event.PriceObservedPayload{
			Instrument:       b.Instrument,
			Currency:         "USD",
			Price:            closePrice,
			ProviderSequence: b.TradeCount,
		},
	}
	if err := envelope.Validate(); err != nil {
		return event.Envelope{}, fmt.Errorf("build canonical provider event: %w", err)
	}
	return envelope, nil
}

func canonicalPrice(value string) string {
	whole, fraction, decimal := strings.Cut(value, ".")
	if !decimal {
		return whole
	}
	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return whole
	}
	return whole + "." + fraction
}

type wireMessage struct {
	Type       string      `json:"T"`
	Message    string      `json:"msg"`
	Code       int         `json:"code"`
	Symbol     string      `json:"S"`
	Close      json.Number `json:"c"`
	TradeCount int64       `json:"n"`
	Timestamp  string      `json:"t"`
	Bars       []string    `json:"bars"`
	Updated    []string    `json:"updatedBars"`
}

func decodeWireMessages(data []byte) ([]wireMessage, error) {
	var messages []wireMessage
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&messages); err != nil || len(messages) == 0 {
		return nil, errors.New("provider returned an invalid message batch")
	}
	return messages, nil
}

func (message wireMessage) bar() (Bar, error) {
	if message.Type != "b" && message.Type != "u" {
		return Bar{}, errors.New("provider message is not a minute bar")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, message.Timestamp)
	if err != nil {
		return Bar{}, errors.New("provider bar contains an invalid timestamp")
	}
	bar := Bar{
		Instrument: message.Symbol,
		Timestamp:  timestamp,
		Close:      message.Close.String(),
		TradeCount: message.TradeCount,
	}
	if err := bar.Validate(); err != nil {
		return Bar{}, err
	}
	return bar, nil
}

func sortBars(bars []Bar) {
	sort.SliceStable(bars, func(left, right int) bool {
		if bars[left].Timestamp.Equal(bars[right].Timestamp) {
			if bars[left].Instrument == bars[right].Instrument {
				return bars[left].TradeCount < bars[right].TradeCount
			}
			return bars[left].Instrument < bars[right].Instrument
		}
		return bars[left].Timestamp.Before(bars[right].Timestamp)
	})
}
