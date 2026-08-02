package scale

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

// Record is the Kafka evidence needed to validate one deterministic load run.
type Record struct {
	Partition int32
	Key       []byte
	Value     []byte
	Headers   map[string]string
}

// Report is emitted by the topic inspector and embedded in the scale evidence.
type Report struct {
	RunID              string          `json:"run_id"`
	Phase              string          `json:"phase"`
	ExpectedRecords    int64           `json:"expected_records"`
	ObservedRecords    int64           `json:"observed_records"`
	InstrumentCount    int             `json:"instrument_count"`
	PartitionCount     int             `json:"partition_count"`
	RecordsByPartition map[int32]int64 `json:"records_by_partition"`
	OrderingValid      bool            `json:"ordering_valid"`
	ValueDigestSHA256  string          `json:"value_digest_sha256"`
}

// GroupMember is the privacy-safe Kafka membership evidence needed to prove
// that a replacement consumer joined the group and received an assignment.
type GroupMember struct {
	ClientHost string
	Assignment []byte
}

// GroupReport contains aggregate group evidence without member IDs or hosts.
type GroupReport struct {
	State                     string `json:"state"`
	MemberCount               int    `json:"member_count"`
	AssignedPartitionCount    int    `json:"assigned_partition_count"`
	RequiredClientHostPresent bool   `json:"required_client_host_present"`
}

// ValidateRun proves key/envelope agreement, per-instrument ordering, and the
// exact record count for one run phase without exposing instrument names.
func ValidateRun(records []Record, runID, phase string, expected int64) (Report, error) {
	if runID == "" || (phase != "original" && phase != "replay") || expected < 0 {
		return Report{}, errors.New("run ID, valid phase, and non-negative expected count are required")
	}
	report := Report{
		RunID:              runID,
		Phase:              phase,
		ExpectedRecords:    expected,
		RecordsByPartition: map[int32]int64{},
		OrderingValid:      true,
	}
	lastSequence := map[string]int64{}
	instrumentPartitions := map[string]int32{}
	valuesByEventID := map[string][]byte{}
	prefix := "scale:" + runID + ":"
	source := "scale." + runID
	for _, record := range records {
		recordRunID, hasRunID := record.Headers["run_id"]
		recordPhase, hasPhase := record.Headers["phase"]
		if !hasRunID || !hasPhase {
			return Report{}, errors.New("scale record is missing run_id or phase header")
		}
		if recordRunID != runID || recordPhase != phase {
			continue
		}
		envelope, err := event.DecodeStrict(record.Value)
		if err != nil {
			return Report{}, fmt.Errorf("decode scale record: %w", err)
		}
		if string(record.Key) != envelope.PartitionKey {
			return Report{}, errors.New("Kafka key does not match the canonical partition key")
		}
		if envelope.Source != source || !strings.HasPrefix(envelope.EventID, prefix) {
			return Report{}, errors.New("scale record identity does not match the requested run")
		}
		if _, exists := valuesByEventID[envelope.EventID]; exists {
			return Report{}, errors.New("scale phase contains a duplicate event ID")
		}
		valuesByEventID[envelope.EventID] = record.Value
		instrument := envelope.Payload.Instrument
		if partition, exists := instrumentPartitions[instrument]; exists && partition != record.Partition {
			return Report{}, errors.New("one instrument was observed in multiple partitions")
		}
		instrumentPartitions[instrument] = record.Partition
		wantSequence := lastSequence[instrument] + 1
		if envelope.Payload.ProviderSequence != wantSequence {
			return Report{}, fmt.Errorf("provider sequence is not contiguous for a scale instrument: got %d, want %d", envelope.Payload.ProviderSequence, wantSequence)
		}
		lastSequence[instrument] = envelope.Payload.ProviderSequence
		report.ObservedRecords++
		report.RecordsByPartition[record.Partition]++
	}
	if report.ObservedRecords != expected {
		return Report{}, fmt.Errorf("observed %d scale records, want %d", report.ObservedRecords, expected)
	}
	report.InstrumentCount = len(lastSequence)
	report.PartitionCount = len(report.RecordsByPartition)
	report.ValueDigestSHA256 = digestValues(valuesByEventID)
	return report, nil
}

// ValidateConsumerGroup proves that the replacement pod is a stable, assigned
// Kafka group member. The report intentionally omits pod IPs and member IDs.
func ValidateConsumerGroup(
	state string,
	members []GroupMember,
	topic, requiredClientHost string,
	expectedMembers, expectedPartitions int,
) (GroupReport, error) {
	if topic == "" || requiredClientHost == "" || expectedMembers < 1 || expectedPartitions < 1 {
		return GroupReport{}, errors.New("topic, client host, and positive expected counts are required")
	}
	if state != "Stable" {
		return GroupReport{}, fmt.Errorf("consumer group state is %q, want Stable", state)
	}
	if len(members) != expectedMembers {
		return GroupReport{}, fmt.Errorf("consumer group has %d members, want %d", len(members), expectedMembers)
	}

	requiredClientHost = strings.TrimPrefix(requiredClientHost, "/")
	partitions := make(map[int32]struct{}, expectedPartitions)
	requiredClientHostPresent := false
	for _, member := range members {
		assignment, err := decodeAssignment(member.Assignment)
		if err != nil {
			return GroupReport{}, fmt.Errorf("decode consumer assignment: %w", err)
		}
		assigned := assignment[topic]
		if len(assigned) == 0 {
			return GroupReport{}, errors.New("consumer group member has no assignment for the scale topic")
		}
		if strings.TrimPrefix(member.ClientHost, "/") == requiredClientHost {
			requiredClientHostPresent = true
		}
		for _, partition := range assigned {
			if partition < 0 {
				return GroupReport{}, errors.New("consumer group assignment contains a negative partition")
			}
			if partition >= int32(expectedPartitions) {
				return GroupReport{}, errors.New("consumer group assignment contains a partition outside the expected range")
			}
			if _, exists := partitions[partition]; exists {
				return GroupReport{}, errors.New("scale topic partition is assigned to multiple consumers")
			}
			partitions[partition] = struct{}{}
		}
	}
	if len(partitions) != expectedPartitions {
		return GroupReport{}, fmt.Errorf("consumer group has %d assigned scale partitions, want %d", len(partitions), expectedPartitions)
	}
	if !requiredClientHostPresent {
		return GroupReport{}, errors.New("replacement pod is not an assigned consumer group member")
	}
	return GroupReport{
		State:                     state,
		MemberCount:               len(members),
		AssignedPartitionCount:    len(partitions),
		RequiredClientHostPresent: true,
	}, nil
}

// PercentileMilliseconds uses the nearest-rank definition on a defensive copy.
func PercentileMilliseconds(values []time.Duration, percentile float64) (float64, error) {
	if len(values) == 0 || percentile <= 0 || percentile > 1 {
		return 0, errors.New("non-empty values and a percentile in (0,1] are required")
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	return float64(sorted[index]) / float64(time.Millisecond), nil
}

func digestValues(valuesByEventID map[string][]byte) string {
	eventIDs := make([]string, 0, len(valuesByEventID))
	for eventID := range valuesByEventID {
		eventIDs = append(eventIDs, eventID)
	}
	sort.Strings(eventIDs)
	digest := sha256.New()
	for _, eventID := range eventIDs {
		digest.Write([]byte(eventID))
		digest.Write([]byte{0})
		digest.Write(valuesByEventID[eventID])
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func decodeAssignment(encoded []byte) (map[string][]int32, error) {
	reader := bytes.NewReader(encoded)
	if _, err := readInt16(reader); err != nil {
		return nil, err
	}
	topicCount, err := readInt32(reader)
	if err != nil || topicCount < 0 || topicCount > 1_000 {
		return nil, errors.New("invalid assignment topic count")
	}
	topics := make(map[string][]int32, topicCount)
	for range topicCount {
		topic, err := readString(reader)
		if err != nil || topic == "" {
			return nil, errors.New("invalid assignment topic")
		}
		partitionCount, err := readInt32(reader)
		if err != nil || partitionCount < 0 || partitionCount > 100_000 {
			return nil, errors.New("invalid assignment partition count")
		}
		partitions := make([]int32, partitionCount)
		for index := range partitions {
			partition, err := readInt32(reader)
			if err != nil {
				return nil, errors.New("invalid assignment partition")
			}
			partitions[index] = partition
		}
		topics[topic] = partitions
	}
	userDataLength, err := readInt32(reader)
	if err != nil || userDataLength < -1 || int64(userDataLength) > int64(reader.Len()) {
		return nil, errors.New("invalid assignment user data")
	}
	if userDataLength >= 0 {
		_, _ = reader.Seek(int64(userDataLength), io.SeekCurrent)
	}
	if reader.Len() != 0 {
		return nil, errors.New("assignment contains trailing data")
	}
	return topics, nil
}

func readString(reader *bytes.Reader) (string, error) {
	length, err := readInt16(reader)
	if err != nil || length < 0 || int(length) > reader.Len() {
		return "", io.ErrUnexpectedEOF
	}
	value := make([]byte, length)
	_, _ = io.ReadFull(reader, value)
	return string(value), nil
}

func readInt16(reader *bytes.Reader) (int16, error) {
	var value int16
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

func readInt32(reader *bytes.Reader) (int32, error) {
	var value int32
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}
