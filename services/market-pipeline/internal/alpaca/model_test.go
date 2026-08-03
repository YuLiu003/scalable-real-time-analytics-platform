package alpaca

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func validTestBar(instrument string, timestamp time.Time) Bar {
	return Bar{Instrument: instrument, Timestamp: timestamp, Close: "101.2500", TradeCount: 7}
}

func TestBarValidateBoundaries(t *testing.T) {
	timestamp := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		bar  Bar
	}{
		{name: "instrument", bar: Bar{Instrument: "private", Timestamp: timestamp, Close: "1", TradeCount: 1}},
		{name: "timestamp", bar: Bar{Instrument: "LOAD-A", Close: "1", TradeCount: 1}},
		{name: "negative syntax", bar: Bar{Instrument: "LOAD-A", Timestamp: timestamp, Close: "-1", TradeCount: 1}},
		{name: "zero", bar: Bar{Instrument: "LOAD-A", Timestamp: timestamp, Close: "0.000", TradeCount: 1}},
		{name: "overflow", bar: Bar{Instrument: "LOAD-A", Timestamp: timestamp, Close: strings.Repeat("9", 400), TradeCount: 1}},
		{name: "negative trades", bar: Bar{Instrument: "LOAD-A", Timestamp: timestamp, Close: "1", TradeCount: -1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.bar.Validate(); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
	for _, close := range []string{"1", "1.2", "999999.12345678"} {
		bar := validTestBar("LOAD-A", timestamp)
		bar.Close = close
		if err := bar.Validate(); err != nil {
			t.Errorf("Validate() close %q error = %v", close, err)
		}
	}
}

func TestBarEnvelopeIsCanonicalAndDeterministic(t *testing.T) {
	bar := validTestBar("LOAD-A", time.Date(2026, 8, 3, 12, 34, 0, 123, time.FixedZone("private", -7*60*60)))
	first, err := bar.Envelope("fakepaca-iex", "fakepaca")
	if err != nil {
		t.Fatalf("Envelope() error = %v", err)
	}
	second, err := bar.Envelope("fakepaca-iex", "fakepaca")
	if err != nil {
		t.Fatalf("second Envelope() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Envelope() is non-deterministic: %#v != %#v", first, second)
	}
	decimalVariant := bar
	decimalVariant.Close = "101.25000000"
	canonicalVariant, err := decimalVariant.Envelope("fakepaca-iex", "fakepaca")
	if err != nil || !reflect.DeepEqual(first, canonicalVariant) {
		t.Fatalf("Envelope() did not canonicalize equivalent provider decimals: %#v, %v", canonicalVariant, err)
	}
	otherTenant, err := bar.Envelope("fakepaca-iex", "fakepaca-alt")
	if err != nil || first.EventID == otherTenant.EventID || first.TraceID == otherTenant.TraceID {
		t.Fatalf("Envelope() identity is not tenant-scoped: %#v, %v", otherTenant, err)
	}
	if first.OccurredAt != "2026-08-03T19:35:00.000000123Z" || first.IngestedAt != first.OccurredAt ||
		first.PartitionKey != "LOAD-A" || first.Payload.Instrument != "LOAD-A" || first.Payload.Currency != "USD" ||
		first.Payload.Price != "101.25" || first.Payload.ProviderSequence != bar.TradeCount ||
		!strings.HasPrefix(first.EventID, "alpaca:bar:") || len(first.TraceID) != 32 {
		t.Fatalf("Envelope() = %#v", first)
	}

	invalid := bar
	invalid.Close = "0"
	if _, err := invalid.Envelope("fakepaca-iex", "fakepaca"); err == nil {
		t.Fatal("Envelope() accepted an invalid bar")
	}
	if _, err := bar.Envelope("INVALID", "fakepaca"); err == nil || !strings.Contains(err.Error(), "canonical provider event") {
		t.Fatalf("Envelope() invalid source error = %v", err)
	}
}

func TestCanonicalPrice(t *testing.T) {
	for value, want := range map[string]string{
		"1": "1", "1.00000000": "1", "1.23000000": "1.23", "0.00000001": "0.00000001",
	} {
		if got := canonicalPrice(value); got != want {
			t.Errorf("canonicalPrice(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestDecodeWireMessages(t *testing.T) {
	messages, err := decodeWireMessages([]byte(`[{"T":"success","msg":"connected"}]`))
	if err != nil || len(messages) != 1 || messages[0].Message != "connected" {
		t.Fatalf("decodeWireMessages() = %#v, %v", messages, err)
	}
	for _, data := range [][]byte{[]byte(``), []byte(`{}`), []byte(`[]`), []byte(`private`)} {
		if _, err := decodeWireMessages(data); err == nil {
			t.Errorf("decodeWireMessages(%q) succeeded", data)
		}
	}
}

func TestWireMessageBar(t *testing.T) {
	valid := wireMessage{Type: "b", Symbol: "LOAD-A", Close: "12.5", TradeCount: 2, Timestamp: "2026-08-03T12:00:00Z"}
	bar, err := valid.bar()
	if err != nil || bar.Instrument != "LOAD-A" || bar.Close != "12.5" {
		t.Fatalf("bar() = %#v, %v", bar, err)
	}
	valid.Type = "u"
	if _, err := valid.bar(); err != nil {
		t.Fatalf("updated bar error = %v", err)
	}
	invalid := []wireMessage{
		{Type: "success"},
		{Type: "b", Symbol: "LOAD-A", Close: "1", Timestamp: "private"},
		{Type: "b", Symbol: "private", Close: "1", Timestamp: "2026-08-03T12:00:00Z"},
	}
	for _, message := range invalid {
		if _, err := message.bar(); err == nil {
			t.Errorf("bar() accepted %#v", message)
		}
	}
}

func TestSortBarsOrdersTimestampInstrumentAndTradeCount(t *testing.T) {
	first := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	bars := []Bar{
		{Instrument: "LOAD-B", Timestamp: first, TradeCount: 1},
		{Instrument: "LOAD-A", Timestamp: second, TradeCount: 1},
		{Instrument: "LOAD-A", Timestamp: first, TradeCount: 3},
		{Instrument: "LOAD-A", Timestamp: first, TradeCount: 2},
	}
	sortBars(bars)
	got := []string{
		bars[0].Instrument + ":" + string(rune(bars[0].TradeCount)),
		bars[1].Instrument + ":" + string(rune(bars[1].TradeCount)),
		bars[2].Instrument + ":" + string(rune(bars[2].TradeCount)),
		bars[3].Instrument + ":" + string(rune(bars[3].TradeCount)),
	}
	want := []string{"LOAD-A:\x02", "LOAD-A:\x03", "LOAD-B:\x01", "LOAD-A:\x01"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortBars() = %#v, want %#v", got, want)
	}
}
