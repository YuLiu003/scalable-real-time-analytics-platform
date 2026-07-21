package event

import (
	"strings"
	"testing"
)

func validEvent() Envelope {
	return Envelope{
		EventID:       "synthetic:price:aapl:20260721t000000z",
		EventType:     MarketPriceObservedType,
		SchemaVersion: MarketPriceSchemaVersion,
		Source:        "synthetic",
		TenantID:      "demo",
		OccurredAt:    "2026-07-21T00:00:00Z",
		IngestedAt:    "2026-07-21T00:00:01Z",
		PartitionKey:  "AAPL",
		TraceID:       "11111111111111111111111111111111",
		Payload: PriceObservedPayload{
			Instrument:       "AAPL",
			Currency:         "USD",
			Price:            "214.1250",
			ProviderSequence: 1,
		},
	}
}

func TestEnvelopeRoundTripAndArchiveKey(t *testing.T) {
	e := validEvent()
	encoded, err := e.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	decoded, err := DecodeStrict(encoded)
	if err != nil {
		t.Fatalf("DecodeStrict() error = %v", err)
	}
	if decoded.EventID != e.EventID {
		t.Fatalf("event ID = %q, want %q", decoded.EventID, e.EventID)
	}
	key, err := decoded.ArchiveKey()
	if err != nil {
		t.Fatalf("ArchiveKey() error = %v", err)
	}
	want := "bronze/market.price.observed/v1/date=2026-07-21/source=synthetic/instrument=AAPL/synthetic:price:aapl:20260721t000000z.json"
	if key != want {
		t.Fatalf("ArchiveKey() = %q, want %q", key, want)
	}
}

func TestDecodeStrictRejectsUnknownFields(t *testing.T) {
	encoded, err := validEvent().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := strings.Replace(string(encoded), `"payload":`, `"unexpected":true,"payload":`, 1)
	if _, err := DecodeStrict([]byte(withUnknown)); err == nil {
		t.Fatal("DecodeStrict() accepted an unknown envelope field")
	}
}

func TestValidateRejectsEventIdentityCollisionInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{name: "floating point style exponent", mutate: func(e *Envelope) { e.Payload.Price = "2.14e2" }},
		{name: "instrument mismatch", mutate: func(e *Envelope) { e.Payload.Instrument = "MSFT" }},
		{name: "zero trace", mutate: func(e *Envelope) { e.TraceID = strings.Repeat("0", 32) }},
		{name: "source too long", mutate: func(e *Envelope) { e.Source = strings.Repeat("s", 33) }},
		{name: "tenant too long", mutate: func(e *Envelope) { e.TenantID = strings.Repeat("t", 65) }},
		{name: "unknown schema", mutate: func(e *Envelope) { e.SchemaVersion = 2 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEvent()
			tt.mutate(&e)
			if err := e.Validate(); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
}
