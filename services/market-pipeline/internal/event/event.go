package event

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	MarketPriceObservedType  = "market.price.observed"
	MarketPriceSchemaVersion = 1
)

var (
	eventIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)
	sourcePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)
	tenantIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	instrumentPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.-]{0,14}$`)
	currencyPattern   = regexp.MustCompile(`^[A-Z]{3}$`)
	pricePattern      = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,8})?$`)
	traceIDPattern    = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Envelope is the canonical v1 market.price.observed event. String decimal
// prices avoid binary floating-point ambiguity at the contract boundary.
type Envelope struct {
	EventID       string               `json:"event_id"`
	EventType     string               `json:"event_type"`
	SchemaVersion int                  `json:"schema_version"`
	Source        string               `json:"source"`
	TenantID      string               `json:"tenant_id"`
	OccurredAt    string               `json:"occurred_at"`
	IngestedAt    string               `json:"ingested_at"`
	PartitionKey  string               `json:"partition_key"`
	TraceID       string               `json:"trace_id"`
	Payload       PriceObservedPayload `json:"payload"`
}

type PriceObservedPayload struct {
	Instrument       string `json:"instrument"`
	Currency         string `json:"currency"`
	Price            string `json:"price"`
	ProviderSequence int64  `json:"provider_sequence"`
}

func DecodeStrict(data []byte) (Envelope, error) {
	var envelope Envelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode canonical event: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Envelope{}, err
	}
	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode canonical event: multiple JSON values")
		}
		return fmt.Errorf("decode canonical event trailer: %w", err)
	}
	return nil
}

func (e Envelope) Validate() error {
	if !eventIDPattern.MatchString(e.EventID) {
		return errors.New("event_id does not match the canonical identifier format")
	}
	if e.EventType != MarketPriceObservedType {
		return fmt.Errorf("event_type must be %q", MarketPriceObservedType)
	}
	if e.SchemaVersion != MarketPriceSchemaVersion {
		return fmt.Errorf("schema_version must be %d", MarketPriceSchemaVersion)
	}
	if !sourcePattern.MatchString(e.Source) {
		return errors.New("source does not match the canonical slug format")
	}
	if !tenantIDPattern.MatchString(e.TenantID) {
		return errors.New("tenant_id does not match the canonical slug format")
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, e.OccurredAt)
	if err != nil {
		return fmt.Errorf("occurred_at must be RFC3339: %w", err)
	}
	ingestedAt, err := time.Parse(time.RFC3339Nano, e.IngestedAt)
	if err != nil {
		return fmt.Errorf("ingested_at must be RFC3339: %w", err)
	}
	if ingestedAt.Before(occurredAt) {
		return errors.New("ingested_at cannot precede occurred_at")
	}
	if !instrumentPattern.MatchString(e.PartitionKey) {
		return errors.New("partition_key must be a canonical instrument identifier")
	}
	if !traceIDPattern.MatchString(e.TraceID) || e.TraceID == strings.Repeat("0", 32) {
		return errors.New("trace_id must be a non-zero 32-character lowercase hex value")
	}
	if e.Payload.Instrument != e.PartitionKey {
		return errors.New("payload.instrument must match partition_key")
	}
	if !currencyPattern.MatchString(e.Payload.Currency) {
		return errors.New("payload.currency must be an ISO-style three-letter code")
	}
	if !pricePattern.MatchString(e.Payload.Price) {
		return errors.New("payload.price must be a non-negative decimal string with at most eight fractional digits")
	}
	if e.Payload.ProviderSequence < 0 {
		return errors.New("payload.provider_sequence cannot be negative")
	}
	return nil
}

func (e Envelope) Marshal() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}

func (e Envelope) ArchiveKey() (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}
	occurredAt, _ := time.Parse(time.RFC3339Nano, e.OccurredAt)
	return fmt.Sprintf(
		"bronze/%s/v%d/date=%s/source=%s/instrument=%s/%s.json",
		e.EventType,
		e.SchemaVersion,
		occurredAt.UTC().Format(time.DateOnly),
		e.Source,
		e.Payload.Instrument,
		e.EventID,
	), nil
}
