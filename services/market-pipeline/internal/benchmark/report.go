package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/scale"
)

var requiredOutcomes = []string{"created", "duplicate", "quarantined", "error"}

var requiredResourceComponents = []string{"archiver", "kafka", "object_store"}

type histogramBucket struct {
	upper float64
	count float64
}

type ProducerSummary struct {
	Messages                       int64   `json:"messages"`
	TargetRateEventsPerSecond      int64   `json:"target_rate_events_per_second"`
	DurationMilliseconds           float64 `json:"duration_milliseconds"`
	ThroughputEventsPerSecond      float64 `json:"throughput_events_per_second"`
	AcknowledgementP50Milliseconds float64 `json:"ack_p50_milliseconds"`
	AcknowledgementP95Milliseconds float64 `json:"ack_p95_milliseconds"`
	AcknowledgementP99Milliseconds float64 `json:"ack_p99_milliseconds"`
}

type LatencyQuantiles struct {
	Observations int64   `json:"observations"`
	P50Seconds   float64 `json:"p50_seconds"`
	P95Seconds   float64 `json:"p95_seconds"`
	P99Seconds   float64 `json:"p99_seconds"`
}

type LagSample struct {
	ElapsedMilliseconds int64           `json:"elapsed_milliseconds"`
	Lag                 scale.LagReport `json:"lag"`
	AvailableReplicas   int             `json:"available_replicas"`
	ProducerComplete    bool            `json:"producer_complete"`
}

type LagSummary struct {
	SampleCount                   int             `json:"sample_count"`
	MaximumTotalLag               int64           `json:"maximum_total_lag"`
	MaximumLagByPartition         map[int32]int64 `json:"maximum_lag_by_partition"`
	LagObserved                   bool            `json:"lag_observed"`
	ObservedLagDrainMilliseconds  int64           `json:"observed_lag_drain_milliseconds"`
	PostProducerDrainMilliseconds int64           `json:"post_producer_drain_milliseconds"`
}

type ResourcePeak struct {
	CPUAvailable    bool    `json:"cpu_available"`
	PeakCPUCores    float64 `json:"peak_cpu_cores,omitempty"`
	MemoryAvailable bool    `json:"memory_available"`
	PeakMemoryBytes float64 `json:"peak_memory_bytes,omitempty"`
}

type RunConfig struct {
	Events                       int64 `json:"events"`
	Repetition                   int   `json:"repetition"`
	TargetRate                   int64 `json:"target_rate_events_per_second"`
	TopicPartitions              int   `json:"topic_partitions"`
	ConsumerReplicas             int   `json:"consumer_replicas"`
	ArchiveDelayMillis           int   `json:"archive_delay_milliseconds"`
	ResourceMeasurementsRequired bool  `json:"resource_measurements_required"`
}

type RunMeasurements struct {
	Producer                         ProducerSummary         `json:"producer"`
	DurableCompletionMilliseconds    int64                   `json:"durable_completion_milliseconds"`
	DurableThroughputEventsPerSecond float64                 `json:"durable_throughput_events_per_second"`
	DurableLatency                   LatencyQuantiles        `json:"durable_latency"`
	Lag                              LagSummary              `json:"lag"`
	Resources                        map[string]ResourcePeak `json:"resources,omitempty"`
}

type RunAssertions struct {
	BrokerAcknowledgedEvents int64            `json:"broker_acknowledged_events"`
	TopicObservedEvents      int64            `json:"topic_observed_events"`
	ArchiveCreatedEvents     int64            `json:"archive_created_events"`
	UniqueArchiveObjects     int64            `json:"unique_archive_objects"`
	OutcomeDeltas            map[string]int64 `json:"outcome_deltas"`
	OrderingValid            bool             `json:"ordering_valid"`
	NoLoss                   bool             `json:"no_loss"`
	NoUnexpectedDuplicates   bool             `json:"no_unexpected_duplicates"`
	NoQuarantineOrErrors     bool             `json:"no_quarantine_or_errors"`
}

type RunReport struct {
	SchemaVersion int             `json:"schema_version"`
	EvidenceScope string          `json:"evidence_scope"`
	SuiteID       string          `json:"suite_id"`
	RunID         string          `json:"run_id"`
	Config        RunConfig       `json:"config"`
	Measurements  RunMeasurements `json:"measurements"`
	Assertions    RunAssertions   `json:"assertions"`
	Limitations   []string        `json:"limitations"`
}

type RunInput struct {
	Spec                          RunSpec
	Partitions                    int
	Workers                       int
	DurableCompletionMilliseconds int64
	Producer                      ProducerSummary
	LatencyBefore                 map[string]float64
	LatencyAfter                  map[string]float64
	OutcomesBefore                map[string]float64
	OutcomesAfter                 map[string]float64
	Samples                       []LagSample
	Ordering                      scale.Report
	ArchiveCount                  int64
	Resources                     map[string]ResourcePeak
}

func DecodeProducerSummary(data []byte) (ProducerSummary, error) {
	var found *ProducerSummary
	for _, line := range strings.Split(string(data), "\n") {
		var record struct {
			Message string `json:"msg"`
			ProducerSummary
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.Message != "load scenario completed" {
			continue
		}
		if found != nil {
			return ProducerSummary{}, errors.New("producer log contains multiple completion summaries")
		}
		candidate := record.ProducerSummary
		found = &candidate
	}
	if found == nil {
		return ProducerSummary{}, errors.New("producer completion summary is missing")
	}
	if found.Messages < 1 || found.TargetRateEventsPerSecond < 0 || found.DurationMilliseconds <= 0 ||
		!finitePositive(found.ThroughputEventsPerSecond) || !finiteNonNegative(found.AcknowledgementP50Milliseconds) ||
		!finiteNonNegative(found.AcknowledgementP95Milliseconds) || !finiteNonNegative(found.AcknowledgementP99Milliseconds) ||
		found.AcknowledgementP50Milliseconds > found.AcknowledgementP95Milliseconds ||
		found.AcknowledgementP95Milliseconds > found.AcknowledgementP99Milliseconds {
		return ProducerSummary{}, errors.New("producer completion summary contains invalid measurements")
	}
	return *found, nil
}

func DecodeLabeledVector(data []byte, label string) (map[string]float64, error) {
	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode Prometheus vector: %w", err)
	}
	if response.Status != "success" || len(response.Data.Result) == 0 {
		return nil, errors.New("Prometheus vector is unavailable")
	}
	values := make(map[string]float64, len(response.Data.Result))
	for _, result := range response.Data.Result {
		key := result.Metric[label]
		if key == "" {
			return nil, fmt.Errorf("Prometheus vector is missing label %q", label)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("Prometheus vector repeats label %q", key)
		}
		value, err := prometheusValue(result.Value)
		if err != nil || !finiteNonNegative(value) {
			return nil, fmt.Errorf("Prometheus vector value for %q is invalid", key)
		}
		values[key] = value
	}
	return values, nil
}

func DecodeRangePeak(data []byte) (float64, bool, error) {
	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Values [][]json.RawMessage `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return 0, false, fmt.Errorf("decode Prometheus range: %w", err)
	}
	if response.Status != "success" {
		return 0, false, errors.New("Prometheus range query failed")
	}
	if len(response.Data.Result) == 0 {
		return 0, false, nil
	}
	if len(response.Data.Result) != 1 || len(response.Data.Result[0].Values) == 0 {
		return 0, false, errors.New("Prometheus range must contain one aggregate series")
	}
	var peak float64
	for _, sample := range response.Data.Result[0].Values {
		value, err := prometheusValue(sample)
		if err != nil || !finiteNonNegative(value) {
			return 0, false, errors.New("Prometheus range contains an invalid sample")
		}
		if value > peak {
			peak = value
		}
	}
	return peak, true, nil
}

func DecodeLagSamples(data []byte) ([]LagSample, error) {
	var samples []LagSample
	for lineNumber, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var sample LagSample
		if err := json.Unmarshal([]byte(line), &sample); err != nil {
			return nil, fmt.Errorf("decode lag sample %d: %w", lineNumber+1, err)
		}
		samples = append(samples, sample)
	}
	if len(samples) == 0 {
		return nil, errors.New("lag samples are missing")
	}
	return samples, nil
}

func HistogramQuantiles(before, after map[string]float64) (LatencyQuantiles, error) {
	if len(before) == 0 || len(before) != len(after) {
		return LatencyQuantiles{}, errors.New("histogram snapshots must contain the same buckets")
	}
	var buckets []histogramBucket
	for label, afterValue := range after {
		beforeValue, exists := before[label]
		if !exists || afterValue < beforeValue {
			return LatencyQuantiles{}, errors.New("histogram counters are missing or decreased")
		}
		upper := math.Inf(1)
		if label != "+Inf" {
			parsed, err := strconv.ParseFloat(label, 64)
			if err != nil || !finiteNonNegative(parsed) {
				return LatencyQuantiles{}, fmt.Errorf("histogram bucket %q is invalid", label)
			}
			upper = parsed
		}
		buckets = append(buckets, histogramBucket{upper: upper, count: afterValue - beforeValue})
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].upper < buckets[j].upper })
	if len(buckets) < 2 || !math.IsInf(buckets[len(buckets)-1].upper, 1) {
		return LatencyQuantiles{}, errors.New("histogram +Inf bucket is missing")
	}
	for index := 1; index < len(buckets); index++ {
		if buckets[index].count < buckets[index-1].count {
			return LatencyQuantiles{}, errors.New("histogram cumulative counts decreased")
		}
	}
	total := buckets[len(buckets)-1].count
	if total < 1 || math.Abs(total-math.Round(total)) > 0.000001 {
		return LatencyQuantiles{}, errors.New("histogram observation count is invalid")
	}
	return LatencyQuantiles{
		Observations: int64(math.Round(total)),
		P50Seconds:   interpolatedQuantile(buckets, total, 0.50),
		P95Seconds:   interpolatedQuantile(buckets, total, 0.95),
		P99Seconds:   interpolatedQuantile(buckets, total, 0.99),
	}, nil
}

func interpolatedQuantile(buckets []histogramBucket, total, fraction float64) float64 {
	rank := fraction * total
	previousCount := float64(0)
	lower := float64(0)
	for index, current := range buckets {
		if current.count < rank {
			previousCount = current.count
			if !math.IsInf(current.upper, 1) {
				lower = current.upper
			}
			continue
		}
		if math.IsInf(current.upper, 1) {
			return buckets[index-1].upper
		}
		observations := current.count - previousCount
		return lower + (current.upper-lower)*(rank-previousCount)/observations
	}
	return math.NaN()
}

func OutcomeDelta(before, after map[string]float64) (map[string]int64, error) {
	delta := make(map[string]int64, len(requiredOutcomes))
	for _, outcome := range requiredOutcomes {
		beforeValue, beforeExists := before[outcome]
		afterValue, afterExists := after[outcome]
		difference := afterValue - beforeValue
		if !beforeExists || !afterExists || difference < 0 || math.Abs(difference-math.Round(difference)) > 0.000001 {
			return nil, fmt.Errorf("outcome %q has invalid counter snapshots", outcome)
		}
		delta[outcome] = int64(math.Round(difference))
	}
	return delta, nil
}

func SummarizeLag(samples []LagSample, partitions, workers int) (LagSummary, error) {
	if len(samples) == 0 || partitions < 1 || workers < 1 {
		return LagSummary{}, errors.New("lag samples, partitions, and workers are required")
	}
	summary := LagSummary{SampleCount: len(samples), MaximumLagByPartition: make(map[int32]int64, partitions)}
	previousElapsed := int64(-1)
	firstPositive := int64(-1)
	firstProducerComplete := int64(-1)
	drainedAt := int64(-1)
	for _, sample := range samples {
		if sample.ElapsedMilliseconds <= previousElapsed || sample.AvailableReplicas != workers ||
			sample.Lag.PartitionCount != partitions || len(sample.Lag.LagByPartition) != partitions {
			return LagSummary{}, errors.New("lag sample topology or timestamp is invalid")
		}
		previousElapsed = sample.ElapsedMilliseconds
		var total, maximum int64
		for partition := range int32(partitions) {
			lag, exists := sample.Lag.LagByPartition[partition]
			if !exists || lag < 0 {
				return LagSummary{}, errors.New("lag sample has an invalid partition set")
			}
			total += lag
			if lag > maximum {
				maximum = lag
			}
			if lag > summary.MaximumLagByPartition[partition] {
				summary.MaximumLagByPartition[partition] = lag
			}
		}
		if total != sample.Lag.TotalLag || maximum != sample.Lag.MaxPartitionLag {
			return LagSummary{}, errors.New("lag sample aggregate does not match its partitions")
		}
		if total > summary.MaximumTotalLag {
			summary.MaximumTotalLag = total
		}
		if total > 0 && firstPositive == -1 {
			firstPositive = sample.ElapsedMilliseconds
		}
		if sample.ProducerComplete && firstProducerComplete == -1 {
			firstProducerComplete = sample.ElapsedMilliseconds
		}
		if firstProducerComplete >= 0 && total == 0 {
			drainedAt = sample.ElapsedMilliseconds
			break
		}
	}
	if firstProducerComplete == -1 || drainedAt == -1 {
		return LagSummary{}, errors.New("lag samples do not prove producer completion and drain")
	}
	if firstPositive >= 0 {
		summary.LagObserved = true
		summary.ObservedLagDrainMilliseconds = drainedAt - firstPositive
	}
	summary.PostProducerDrainMilliseconds = drainedAt - firstProducerComplete
	return summary, nil
}

func BuildRunReport(input RunInput) (RunReport, error) {
	if input.Partitions < 1 || input.Workers < 1 || input.DurableCompletionMilliseconds < 1 {
		return RunReport{}, errors.New("positive topology and durable completion duration are required")
	}
	if !suiteIDPattern.MatchString(input.Spec.SuiteID) ||
		input.Spec.RunID != fmt.Sprintf("%s-e%d-r%d", input.Spec.SuiteID, input.Spec.EventCount, input.Spec.Repetition) {
		return RunReport{}, errors.New("run identity does not match the benchmark plan")
	}
	if input.Spec.ResourceMeasurementsRequired != resourceMeasurementsRequired(input.Spec.EventCount, input.Spec.TargetRate) {
		return RunReport{}, errors.New("run resource policy does not match the benchmark plan")
	}
	if input.Producer.Messages != input.Spec.EventCount || input.Producer.TargetRateEventsPerSecond != input.Spec.TargetRate {
		return RunReport{}, errors.New("producer summary does not match the run plan")
	}
	latency, err := HistogramQuantiles(input.LatencyBefore, input.LatencyAfter)
	if err != nil {
		return RunReport{}, err
	}
	if latency.Observations != input.Spec.EventCount {
		return RunReport{}, fmt.Errorf("durable latency observed %d events, want %d", latency.Observations, input.Spec.EventCount)
	}
	outcomes, err := OutcomeDelta(input.OutcomesBefore, input.OutcomesAfter)
	if err != nil {
		return RunReport{}, err
	}
	if outcomes["created"] != input.Spec.EventCount || outcomes["duplicate"] != 0 ||
		outcomes["quarantined"] != 0 || outcomes["error"] != 0 {
		return RunReport{}, fmt.Errorf("archive outcomes do not match a lossless original run: %v", outcomes)
	}
	lag, err := SummarizeLag(input.Samples, input.Partitions, input.Workers)
	if err != nil {
		return RunReport{}, err
	}
	if input.Ordering.RunID != input.Spec.RunID || input.Ordering.Phase != "original" ||
		input.Ordering.ExpectedRecords != input.Spec.EventCount || input.Ordering.ObservedRecords != input.Spec.EventCount ||
		input.Ordering.PartitionCount != input.Partitions || !input.Ordering.OrderingValid {
		return RunReport{}, errors.New("topic ordering evidence does not match the run plan")
	}
	if input.ArchiveCount != input.Spec.EventCount {
		return RunReport{}, fmt.Errorf("archive contains %d objects, want %d", input.ArchiveCount, input.Spec.EventCount)
	}
	if input.Spec.ResourceMeasurementsRequired {
		for _, component := range requiredResourceComponents {
			resource, exists := input.Resources[component]
			if !exists || !resource.CPUAvailable || !resource.MemoryAvailable ||
				!finiteNonNegative(resource.PeakCPUCores) || !finiteNonNegative(resource.PeakMemoryBytes) {
				return RunReport{}, fmt.Errorf("resource measurements for %s are unavailable", component)
			}
		}
		if len(input.Resources) != len(requiredResourceComponents) {
			return RunReport{}, errors.New("resource measurements do not match the required component set")
		}
	} else if len(input.Resources) != 0 {
		return RunReport{}, errors.New("resource measurements must be omitted when the run plan does not require them")
	}
	durableThroughput := float64(input.Spec.EventCount) / (float64(input.DurableCompletionMilliseconds) / 1000)
	limitations := []string{
		"Synthetic local load is not production or AWS capacity evidence.",
		"The single-replica Kafka broker and object store do not provide availability evidence.",
		producerScopeLimitation(input.Spec.TargetRate),
	}
	if !input.Spec.ResourceMeasurementsRequired {
		limitations = append(limitations, resourceOmissionLimitation())
	}
	return RunReport{
		SchemaVersion: SchemaVersion,
		EvidenceScope: EvidenceScope,
		SuiteID:       input.Spec.SuiteID,
		RunID:         input.Spec.RunID,
		Config: RunConfig{
			Events:                       input.Spec.EventCount,
			Repetition:                   input.Spec.Repetition,
			TargetRate:                   input.Spec.TargetRate,
			TopicPartitions:              input.Partitions,
			ConsumerReplicas:             input.Workers,
			ArchiveDelayMillis:           0,
			ResourceMeasurementsRequired: input.Spec.ResourceMeasurementsRequired,
		},
		Measurements: RunMeasurements{
			Producer:                         input.Producer,
			DurableCompletionMilliseconds:    input.DurableCompletionMilliseconds,
			DurableThroughputEventsPerSecond: durableThroughput,
			DurableLatency:                   latency,
			Lag:                              lag,
			Resources:                        input.Resources,
		},
		Assertions: RunAssertions{
			BrokerAcknowledgedEvents: input.Producer.Messages,
			TopicObservedEvents:      input.Ordering.ObservedRecords,
			ArchiveCreatedEvents:     outcomes["created"],
			UniqueArchiveObjects:     input.ArchiveCount,
			OutcomeDeltas:            outcomes,
			OrderingValid:            true,
			NoLoss:                   true,
			NoUnexpectedDuplicates:   true,
			NoQuarantineOrErrors:     true,
		},
		Limitations: limitations,
	}, nil
}

func producerScopeLimitation(targetRate int64) string {
	if targetRate > 0 {
		return "Rate-controlled production validates the configured arrival rate, not burst capacity or a sustained provider feed."
	}
	return "Unbounded production measures burst completion, not a sustained provider feed."
}

func resourceOmissionLimitation() string {
	return "CPU and memory are intentionally not collected for the short unbounded 10,000-event scenario because its durable boundary may not contain the required 25-second CPU window."
}

func prometheusValue(sample []json.RawMessage) (float64, error) {
	if len(sample) != 2 {
		return 0, errors.New("Prometheus sample must contain timestamp and value")
	}
	var encoded string
	if err := json.Unmarshal(sample[1], &encoded); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(encoded, 64)
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func finitePositive(value float64) bool {
	return finiteNonNegative(value) && value > 0
}
