package scale

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

func record(t *testing.T, instrument string, sequence int64, partition int32, runID, phase string) Record {
	t.Helper()
	envelope := event.Envelope{
		EventID:       fmt.Sprintf("scale:%s:%s:%09d", runID, strings.ToLower(instrument), sequence),
		EventType:     event.MarketPriceObservedType,
		SchemaVersion: event.MarketPriceSchemaVersion,
		Source:        "scale." + runID,
		TenantID:      "load-test",
		OccurredAt:    "2026-07-30T00:00:00Z",
		IngestedAt:    "2026-07-30T00:00:00.001Z",
		PartitionKey:  instrument,
		TraceID:       "11111111111111111111111111111111",
		Payload: event.PriceObservedPayload{
			Instrument:       instrument,
			Currency:         "USD",
			Price:            "100.0000",
			ProviderSequence: sequence,
		},
	}
	value, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return Record{
		Partition: partition,
		Key:       []byte(instrument),
		Value:     value,
		Headers:   map[string]string{"run_id": runID, "phase": phase},
	}
}

func TestValidateRunReportsAggregateEvidence(t *testing.T) {
	records := []Record{
		record(t, "LOAD-A", 1, 0, "proof", "original"),
		record(t, "LOAD-A", 2, 0, "proof", "original"),
		record(t, "LOAD-B", 1, 2, "proof", "original"),
		record(t, "LOAD-A", 1, 0, "other", "original"),
		record(t, "LOAD-A", 1, 0, "proof", "replay"),
	}
	report, err := ValidateRun(records, "proof", "original", 3)
	if err != nil {
		t.Fatalf("ValidateRun() error = %v", err)
	}
	if report.ObservedRecords != 3 || report.InstrumentCount != 2 || report.PartitionCount != 2 ||
		report.RecordsByPartition[0] != 2 || report.RecordsByPartition[2] != 1 || !report.OrderingValid ||
		len(report.ValueDigestSHA256) != 64 {
		t.Fatalf("report = %+v", report)
	}
	replay := []Record{
		record(t, "LOAD-A", 1, 0, "proof", "replay"),
		record(t, "LOAD-A", 2, 0, "proof", "replay"),
		record(t, "LOAD-B", 1, 2, "proof", "replay"),
	}
	replayReport, err := ValidateRun(replay, "proof", "replay", 3)
	if err != nil || replayReport.ValueDigestSHA256 != report.ValueDigestSHA256 {
		t.Fatalf("replay report = %+v, %v", replayReport, err)
	}
}

func TestValidateRunRejectsInvalidRequestAndEvidence(t *testing.T) {
	if _, err := ValidateRun(nil, "", "invalid", -1); err == nil {
		t.Fatal("ValidateRun() accepted invalid request")
	}
	base := record(t, "LOAD-A", 1, 0, "proof", "original")
	tests := []struct {
		name   string
		mutate func(*Record)
		want   string
	}{
		{name: "headers", mutate: func(r *Record) { delete(r.Headers, "phase") }, want: "missing"},
		{name: "encoding", mutate: func(r *Record) { r.Value = []byte("{") }, want: "decode"},
		{name: "key", mutate: func(r *Record) { r.Key = []byte("LOAD-B") }, want: "Kafka key"},
		{name: "identity", mutate: func(r *Record) {
			var envelope event.Envelope
			if err := json.Unmarshal(r.Value, &envelope); err != nil {
				t.Fatal(err)
			}
			envelope.Source = "scale.other"
			r.Value, _ = json.Marshal(envelope)
		}, want: "identity"},
		{name: "sequence", mutate: func(r *Record) {
			var envelope event.Envelope
			if err := json.Unmarshal(r.Value, &envelope); err != nil {
				t.Fatal(err)
			}
			envelope.Payload.ProviderSequence = 2
			r.Value, _ = json.Marshal(envelope)
		}, want: "sequence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := base
			candidate.Headers = map[string]string{"run_id": "proof", "phase": "original"}
			tt.mutate(&candidate)
			if _, err := ValidateRun([]Record{candidate}, "proof", "original", 1); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateRun() error = %v", err)
			}
		})
	}

	t.Run("partition stability", func(t *testing.T) {
		records := []Record{
			record(t, "LOAD-A", 1, 0, "proof", "original"),
			record(t, "LOAD-A", 2, 1, "proof", "original"),
		}
		if _, err := ValidateRun(records, "proof", "original", 2); err == nil || !strings.Contains(err.Error(), "multiple partitions") {
			t.Fatalf("ValidateRun() error = %v", err)
		}
	})
	t.Run("count", func(t *testing.T) {
		if _, err := ValidateRun([]Record{base}, "proof", "original", 2); err == nil || !strings.Contains(err.Error(), "want 2") {
			t.Fatalf("ValidateRun() error = %v", err)
		}
	})
	t.Run("duplicate event ID", func(t *testing.T) {
		if _, err := ValidateRun([]Record{base, base}, "proof", "original", 2); err == nil || !strings.Contains(err.Error(), "duplicate event ID") {
			t.Fatalf("ValidateRun() error = %v", err)
		}
	})
	t.Run("different values produce different digest", func(t *testing.T) {
		first, err := ValidateRun([]Record{base}, "proof", "original", 1)
		if err != nil {
			t.Fatal(err)
		}
		changed := base
		var envelope event.Envelope
		if err := json.Unmarshal(changed.Value, &envelope); err != nil {
			t.Fatal(err)
		}
		envelope.Payload.Price = "101.0000"
		changed.Value, _ = json.Marshal(envelope)
		second, err := ValidateRun([]Record{changed}, "proof", "original", 1)
		if err != nil || first.ValueDigestSHA256 == second.ValueDigestSHA256 {
			t.Fatalf("changed report = %+v, %v", second, err)
		}
	})
}

func TestValidateConsumerGroupRequiresStableAssignedReplacement(t *testing.T) {
	members := []GroupMember{
		{ClientHost: "/10.0.0.1", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {0}})},
		{ClientHost: "/10.0.0.2", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {1}})},
		{ClientHost: "/10.0.0.3", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {2}})},
	}
	report, err := ValidateConsumerGroup("Stable", members, "market.prices.scale", "10.0.0.3", 3, 3)
	if err != nil || report.MemberCount != 3 || report.AssignedPartitionCount != 3 ||
		!report.RequiredClientHostPresent || report.State != "Stable" {
		t.Fatalf("ValidateConsumerGroup() = %+v, %v", report, err)
	}

	tests := []struct {
		name       string
		state      string
		members    []GroupMember
		topic      string
		host       string
		memberWant int
		partWant   int
		want       string
	}{
		{name: "request", state: "Stable", members: members, partWant: 3, want: "required"},
		{name: "state", state: "PreparingRebalance", members: members, topic: "market.prices.scale", host: "10.0.0.3", memberWant: 3, partWant: 3, want: "state"},
		{name: "members", state: "Stable", members: members[:2], topic: "market.prices.scale", host: "10.0.0.2", memberWant: 3, partWant: 3, want: "members"},
		{name: "decode", state: "Stable", members: []GroupMember{{ClientHost: "10.0.0.1", Assignment: []byte{0}}}, topic: "market.prices.scale", host: "10.0.0.1", memberWant: 1, partWant: 1, want: "decode"},
		{name: "missing topic", state: "Stable", members: []GroupMember{{ClientHost: "10.0.0.1", Assignment: assignment(t, map[string][]int32{"other": {0}})}}, topic: "market.prices.scale", host: "10.0.0.1", memberWant: 1, partWant: 1, want: "no assignment"},
		{name: "negative partition", state: "Stable", members: []GroupMember{{ClientHost: "10.0.0.1", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {-1}})}}, topic: "market.prices.scale", host: "10.0.0.1", memberWant: 1, partWant: 1, want: "negative"},
		{name: "out of range partition", state: "Stable", members: []GroupMember{{ClientHost: "10.0.0.1", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {1}})}}, topic: "market.prices.scale", host: "10.0.0.1", memberWant: 1, partWant: 1, want: "expected range"},
		{name: "duplicate partition", state: "Stable", members: []GroupMember{{ClientHost: "10.0.0.1", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {0}})}, {ClientHost: "10.0.0.2", Assignment: assignment(t, map[string][]int32{"market.prices.scale": {0}})}}, topic: "market.prices.scale", host: "10.0.0.2", memberWant: 2, partWant: 1, want: "multiple"},
		{name: "partition count", state: "Stable", members: members, topic: "market.prices.scale", host: "10.0.0.3", memberWant: 3, partWant: 4, want: "partitions"},
		{name: "replacement", state: "Stable", members: members, topic: "market.prices.scale", host: "10.0.0.4", memberWant: 3, partWant: 3, want: "replacement"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateConsumerGroup(tt.state, tt.members, tt.topic, tt.host, tt.memberWant, tt.partWant); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateConsumerGroup() error = %v", err)
			}
		})
	}
}

func TestCalculateLagReportsEveryPartition(t *testing.T) {
	report, err := CalculateLag(map[int32]PartitionOffsets{
		0: {Oldest: 5, Newest: 20, Committed: 12},
		1: {Oldest: 7, Newest: 11, Committed: -1},
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if report.PartitionCount != 2 || report.TotalLag != 12 || report.MaxPartitionLag != 8 ||
		report.LagByPartition[0] != 8 || report.LagByPartition[1] != 4 {
		t.Fatalf("CalculateLag() = %+v", report)
	}
}

func TestCalculateLagRejectsIncompleteOrInvalidOffsets(t *testing.T) {
	tests := []struct {
		name       string
		offsets    map[int32]PartitionOffsets
		partitions int
		want       string
	}{
		{name: "empty", offsets: nil, partitions: 0, want: "complete"},
		{name: "missing", offsets: map[int32]PartitionOffsets{1: {}}, partitions: 1, want: "missing"},
		{name: "broker", offsets: map[int32]PartitionOffsets{0: {Oldest: 2, Newest: 1}}, partitions: 1, want: "broker"},
		{name: "committed before retention", offsets: map[int32]PartitionOffsets{0: {Oldest: 2, Newest: 3, Committed: 1}}, partitions: 1, want: "committed"},
		{name: "committed after end", offsets: map[int32]PartitionOffsets{0: {Oldest: 0, Newest: 3, Committed: 4}}, partitions: 1, want: "committed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := CalculateLag(tt.offsets, tt.partitions); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("CalculateLag() error = %v", err)
			}
		})
	}
}

func TestDecodeAssignmentRejectsMalformedEncodings(t *testing.T) {
	valid := assignment(t, map[string][]int32{"topic": {0}})
	emptyUserData := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(emptyUserData[len(emptyUserData)-4:], 0)
	if _, err := decodeAssignment(emptyUserData); err != nil {
		t.Fatalf("decodeAssignment(empty user data) error = %v", err)
	}
	cases := [][]byte{
		nil,
		encodedNumbers(int16(0), int32(-1)),
		encodedNumbers(int16(0), int32(1), int16(-1)),
		append(encodedNumbers(int16(0), int32(1), int16(5)), []byte("topic")...),
		append(append(encodedNumbers(int16(0), int32(1), int16(5)), []byte("topic")...), encodedNumbers(int32(-1))...),
		append(append(encodedNumbers(int16(0), int32(1), int16(5)), []byte("topic")...), encodedNumbers(int32(1))...),
		append(append(append(encodedNumbers(int16(0), int32(1), int16(5)), []byte("topic")...), encodedNumbers(int32(0))...), encodedNumbers(int32(-2))...),
		append(append(append(encodedNumbers(int16(0), int32(1), int16(5)), []byte("topic")...), encodedNumbers(int32(0))...), encodedNumbers(int32(1))...),
		append(append([]byte(nil), valid...), 1),
	}
	for _, encoded := range cases {
		if _, err := decodeAssignment(encoded); err == nil {
			t.Fatalf("decodeAssignment(%v) succeeded", encoded)
		}
	}
}

func encodedNumbers(values ...any) []byte {
	var encoded bytes.Buffer
	for _, value := range values {
		_ = binary.Write(&encoded, binary.BigEndian, value)
	}
	return encoded.Bytes()
}

func TestPercentileMilliseconds(t *testing.T) {
	values := []time.Duration{4 * time.Millisecond, time.Millisecond, 3 * time.Millisecond, 2 * time.Millisecond}
	got, err := PercentileMilliseconds(values, 0.75)
	if err != nil {
		t.Fatalf("PercentileMilliseconds() error = %v", err)
	}
	if got != 3 || values[0] != 4*time.Millisecond {
		t.Fatalf("PercentileMilliseconds() = %v, values = %v", got, values)
	}
	if got, err := PercentileMilliseconds(values, 1); err != nil || got != 4 {
		t.Fatalf("PercentileMilliseconds(1) = %v, %v", got, err)
	}
	for _, test := range []struct {
		values     []time.Duration
		percentile float64
	}{
		{values: nil, percentile: 0.95},
		{values: values, percentile: 0},
	} {
		if _, err := PercentileMilliseconds(test.values, test.percentile); err == nil {
			t.Fatal("PercentileMilliseconds() accepted invalid input")
		}
	}
}

func assignment(t *testing.T, topics map[string][]int32) []byte {
	t.Helper()
	var encoded bytes.Buffer
	write := func(value any) {
		if err := binary.Write(&encoded, binary.BigEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	write(int16(0))
	write(int32(len(topics)))
	for topic, partitions := range topics {
		write(int16(len(topic)))
		encoded.WriteString(topic)
		write(int32(len(partitions)))
		for _, partition := range partitions {
			write(partition)
		}
	}
	write(int32(-1))
	return encoded.Bytes()
}
