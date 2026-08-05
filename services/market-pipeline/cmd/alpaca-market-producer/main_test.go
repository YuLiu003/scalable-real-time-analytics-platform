package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/IBM/sarama"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

type fakeMessageProducer struct {
	message *sarama.ProducerMessage
	err     error
}

func (producer *fakeMessageProducer) SendMessage(message *sarama.ProducerMessage) (int32, int64, error) {
	producer.message = message
	return 1, 2, producer.err
}

func TestKafkaPublisherWritesCanonicalMessage(t *testing.T) {
	producer := &fakeMessageProducer{}
	envelope := validEnvelope()
	publisher := kafkaPublisher{producer: producer, topic: "market.prices"}
	if err := publisher.Publish(context.Background(), envelope); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if producer.message == nil || producer.message.Topic != "market.prices" {
		t.Fatalf("message = %#v", producer.message)
	}
	key, err := producer.message.Key.Encode()
	if err != nil || string(key) != envelope.PartitionKey {
		t.Fatalf("key = %q, %v", key, err)
	}
	value, err := producer.message.Value.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := event.DecodeStrict(value)
	if err != nil || !reflect.DeepEqual(decoded, envelope) {
		t.Fatalf("decoded = %#v, %v", decoded, err)
	}
	wantHeaders := map[string]string{"event_id": envelope.EventID, "schema_version": "1"}
	gotHeaders := make(map[string]string, len(producer.message.Headers))
	for _, header := range producer.message.Headers {
		gotHeaders[string(header.Key)] = string(header.Value)
	}
	if !reflect.DeepEqual(gotHeaders, wantHeaders) {
		t.Fatalf("headers = %#v, want %#v", gotHeaders, wantHeaders)
	}
}

func TestKafkaPublisherRejectsCanceledInvalidAndFailedWrites(t *testing.T) {
	t.Run("canceled", func(t *testing.T) {
		producer := &fakeMessageProducer{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := (kafkaPublisher{producer: producer, topic: "market.prices"}).Publish(ctx, validEnvelope())
		if !errors.Is(err, context.Canceled) || producer.message != nil {
			t.Fatalf("Publish() = %v, message %#v", err, producer.message)
		}
	})

	t.Run("invalid envelope", func(t *testing.T) {
		producer := &fakeMessageProducer{}
		envelope := validEnvelope()
		envelope.EventID = "INVALID"
		err := (kafkaPublisher{producer: producer, topic: "market.prices"}).Publish(context.Background(), envelope)
		if err == nil || producer.message != nil {
			t.Fatalf("Publish() = %v, message %#v", err, producer.message)
		}
	})

	t.Run("Kafka failure", func(t *testing.T) {
		want := errors.New("broker unavailable")
		producer := &fakeMessageProducer{err: want}
		err := (kafkaPublisher{producer: producer, topic: "market.prices"}).Publish(context.Background(), validEnvelope())
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "publish canonical market event") {
			t.Fatalf("Publish() error = %v", err)
		}
	})
}

func validEnvelope() event.Envelope {
	return event.Envelope{
		EventID:       "alpaca-iex:private:load-a:20260803t120000z:1:100",
		EventType:     event.MarketPriceObservedType,
		SchemaVersion: event.MarketPriceSchemaVersion,
		Source:        "alpaca-iex",
		TenantID:      "private",
		OccurredAt:    "2026-08-03T12:01:00Z",
		IngestedAt:    "2026-08-03T12:01:00Z",
		PartitionKey:  "LOAD-A",
		TraceID:       "11111111111111111111111111111111",
		Payload: event.PriceObservedPayload{
			Instrument:       "LOAD-A",
			Currency:         "USD",
			Price:            "100.00000000",
			ProviderSequence: 1,
		},
	}
}
