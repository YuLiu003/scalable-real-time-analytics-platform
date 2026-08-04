package benchmark

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/scale"
)

func TestDecodeProducerSummary(t *testing.T) {
	data := append([]byte("not json\n"), producerLog(t, 600, 1000)...)
	summary, err := DecodeProducerSummary(data)
	if err != nil || summary.Messages != 600 || summary.TargetRateEventsPerSecond != 1000 ||
		summary.AcknowledgementP50Milliseconds != 1 || summary.AcknowledgementP99Milliseconds != 3 {
		t.Fatalf("DecodeProducerSummary() = %+v, %v", summary, err)
	}
	if _, err := DecodeProducerSummary(append(producerLog(t, 600, 0), producerLog(t, 600, 0)...)); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("multiple summaries error = %v", err)
	}
	if _, err := DecodeProducerSummary([]byte("{}\n")); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing summary error = %v", err)
	}
	invalid := map[string]any{
		"msg": "load scenario completed", "messages": 0, "target_rate_events_per_second": -1,
		"duration_milliseconds": -1, "throughput_events_per_second": 0,
		"ack_p50_milliseconds": 3, "ack_p95_milliseconds": 2, "ack_p99_milliseconds": 1,
	}
	encoded, _ := json.Marshal(invalid)
	if _, err := DecodeProducerSummary(encoded); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("invalid summary error = %v", err)
	}
}

func TestDecodePrometheusVectorAndRange(t *testing.T) {
	vector := vectorJSON(t, "outcome", map[string]float64{"created": 4, "error": 0})
	values, err := DecodeLabeledVector(vector, "outcome")
	if err != nil || values["created"] != 4 || values["error"] != 0 {
		t.Fatalf("DecodeLabeledVector() = %v, %v", values, err)
	}
	vectorCases := []struct {
		name  string
		data  []byte
		label string
		want  string
	}{
		{name: "decode", data: []byte("{"), label: "le", want: "decode"},
		{name: "unavailable", data: []byte(`{"status":"error"}`), label: "le", want: "unavailable"},
		{name: "label", data: []byte(`{"status":"success","data":{"result":[{"metric":{},"value":[1,"1"]}]}}`), label: "le", want: "missing label"},
		{name: "duplicate", data: []byte(`{"status":"success","data":{"result":[{"metric":{"le":"1"},"value":[1,"1"]},{"metric":{"le":"1"},"value":[1,"2"]}]}}`), label: "le", want: "repeats"},
		{name: "value", data: []byte(`{"status":"success","data":{"result":[{"metric":{"le":"1"},"value":[1,"-1"]}]}}`), label: "le", want: "invalid"},
	}
	for _, tt := range vectorCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeLabeledVector(tt.data, tt.label); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeLabeledVector() error = %v", err)
			}
		})
	}

	peak, available, err := DecodeRangePeak(rangeJSON(t, 1, 3, 2))
	if err != nil || !available || peak != 3 {
		t.Fatalf("DecodeRangePeak() = %v, %v, %v", peak, available, err)
	}
	peak, available, err = DecodeRangePeak([]byte(`{"status":"success","data":{"result":[]}}`))
	if err != nil || available || peak != 0 {
		t.Fatalf("empty DecodeRangePeak() = %v, %v, %v", peak, available, err)
	}
	rangeCases := []struct {
		name string
		data []byte
		want string
	}{
		{name: "decode", data: []byte("{"), want: "decode"},
		{name: "failed", data: []byte(`{"status":"error"}`), want: "failed"},
		{name: "series", data: []byte(`{"status":"success","data":{"result":[{"values":[]},{"values":[]}]}}`), want: "one aggregate"},
		{name: "sample", data: []byte(`{"status":"success","data":{"result":[{"values":[[1,"-1"]]}]}}`), want: "invalid sample"},
	}
	for _, tt := range rangeCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := DecodeRangePeak(tt.data); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeRangePeak() error = %v", err)
			}
		})
	}
}

func TestDecodeLagSamples(t *testing.T) {
	samples := validLagSamples()
	var lines []string
	for _, sample := range samples {
		encoded, _ := json.Marshal(sample)
		lines = append(lines, string(encoded))
	}
	decoded, err := DecodeLagSamples([]byte(strings.Join(lines, "\n") + "\n"))
	if err != nil || len(decoded) != len(samples) {
		t.Fatalf("DecodeLagSamples() = %+v, %v", decoded, err)
	}
	if _, err := DecodeLagSamples([]byte("{\n")); err == nil || !strings.Contains(err.Error(), "sample 1") {
		t.Fatalf("malformed samples error = %v", err)
	}
	if _, err := DecodeLagSamples([]byte(" \n")); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("empty samples error = %v", err)
	}
}

func TestHistogramQuantilesUsesSnapshotDelta(t *testing.T) {
	before := map[string]float64{"1": 10, "2": 20, "+Inf": 20}
	after := map[string]float64{"1": 15, "2": 29, "+Inf": 30}
	quantiles, err := HistogramQuantiles(before, after)
	if err != nil || quantiles.Observations != 10 || quantiles.P50Seconds != 1 || quantiles.P95Seconds != 2 || quantiles.P99Seconds != 2 {
		t.Fatalf("HistogramQuantiles() = %+v, %v", quantiles, err)
	}
	if !math.IsNaN(interpolatedQuantile(nil, 1, 0.5)) {
		t.Fatal("interpolatedQuantile() did not expose missing terminal data")
	}
}

func TestHistogramQuantilesRejectsInvalidSnapshots(t *testing.T) {
	tests := []struct {
		name   string
		before map[string]float64
		after  map[string]float64
		want   string
	}{
		{name: "shape", before: nil, after: nil, want: "same buckets"},
		{name: "decrease", before: map[string]float64{"1": 2, "+Inf": 2}, after: map[string]float64{"1": 1, "+Inf": 2}, want: "decreased"},
		{name: "label", before: map[string]float64{"bad": 0, "+Inf": 0}, after: map[string]float64{"bad": 1, "+Inf": 1}, want: "invalid"},
		{name: "infinity", before: map[string]float64{"1": 0, "2": 0}, after: map[string]float64{"1": 1, "2": 1}, want: "+Inf"},
		{name: "cumulative", before: map[string]float64{"1": 0, "2": 0, "+Inf": 0}, after: map[string]float64{"1": 2, "2": 1, "+Inf": 2}, want: "cumulative"},
		{name: "total", before: map[string]float64{"1": 0, "+Inf": 0}, after: map[string]float64{"1": 0.5, "+Inf": 1.5}, want: "observation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := HistogramQuantiles(tt.before, tt.after); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("HistogramQuantiles() error = %v", err)
			}
		})
	}
}

func TestOutcomeDelta(t *testing.T) {
	before := map[string]float64{"created": 2, "duplicate": 1, "quarantined": 0, "error": 0}
	after := map[string]float64{"created": 5, "duplicate": 1, "quarantined": 0, "error": 0}
	delta, err := OutcomeDelta(before, after)
	if err != nil || delta["created"] != 3 || delta["duplicate"] != 0 {
		t.Fatalf("OutcomeDelta() = %v, %v", delta, err)
	}
	if _, err := OutcomeDelta(before, map[string]float64{"created": 1}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("invalid OutcomeDelta() error = %v", err)
	}
}

func TestSummarizeLag(t *testing.T) {
	summary, err := SummarizeLag(validLagSamples(), 3, 3)
	if err != nil || summary.SampleCount != 3 || summary.MaximumTotalLag != 10 ||
		summary.MaximumLagByPartition[0] != 5 || !summary.LagObserved ||
		summary.ObservedLagDrainMilliseconds != 3 || summary.PostProducerDrainMilliseconds != 3 {
		t.Fatalf("SummarizeLag() = %+v, %v", summary, err)
	}
	noLag := validLagSamples()[2:]
	noLag[0].ElapsedMilliseconds = 0
	summary, err = SummarizeLag(noLag, 3, 3)
	if err != nil || summary.LagObserved || summary.ObservedLagDrainMilliseconds != 0 {
		t.Fatalf("no-lag SummarizeLag() = %+v, %v", summary, err)
	}
}

func TestSummarizeLagRejectsInvalidSamples(t *testing.T) {
	valid := validLagSamples()
	tests := []struct {
		name       string
		samples    []LagSample
		partitions int
		workers    int
		want       string
	}{
		{name: "required", samples: nil, partitions: 0, workers: 0, want: "required"},
		{name: "topology", samples: []LagSample{{ElapsedMilliseconds: 0, AvailableReplicas: 2}}, partitions: 3, workers: 3, want: "topology"},
		{name: "partition", samples: []LagSample{{ElapsedMilliseconds: 0, AvailableReplicas: 3, Lag: scale.LagReport{PartitionCount: 1, LagByPartition: map[int32]int64{1: 0}}}}, partitions: 1, workers: 3, want: "partition set"},
		{name: "aggregate", samples: []LagSample{{ElapsedMilliseconds: 0, AvailableReplicas: 3, ProducerComplete: true, Lag: scale.LagReport{PartitionCount: 1, TotalLag: 2, MaxPartitionLag: 1, LagByPartition: map[int32]int64{0: 1}}}}, partitions: 1, workers: 3, want: "aggregate"},
		{name: "completion", samples: valid[:2], partitions: 3, workers: 3, want: "completion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SummarizeLag(tt.samples, tt.partitions, tt.workers); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("SummarizeLag() error = %v", err)
			}
		})
	}
}

func TestBuildRunReportProducesLosslessEvidence(t *testing.T) {
	report, err := BuildRunReport(validRunInput())
	if err != nil || report.SchemaVersion != SchemaVersion || report.SuiteID != "cap" ||
		report.Measurements.DurableThroughputEventsPerSecond != 200 ||
		report.Assertions.ArchiveCreatedEvents != 600 || !report.Assertions.NoLoss ||
		report.Config.ArchiveDelayMillis != 0 || len(report.Limitations) != 3 ||
		report.Limitations[2] != "Unbounded production measures burst completion, not a sustained provider feed." {
		t.Fatalf("BuildRunReport() = %+v, %v", report, err)
	}
	paced := validRunInput()
	paced.Spec.TargetRate = 250
	paced.Producer.TargetRateEventsPerSecond = 250
	report, err = BuildRunReport(paced)
	if err != nil || report.Limitations[2] != "Rate-controlled production validates the configured arrival rate, not burst capacity or a sustained provider feed." {
		t.Fatalf("paced BuildRunReport() = %+v, %v", report, err)
	}
}

func TestBuildRunReportRejectsInvalidEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RunInput)
		want   string
	}{
		{name: "topology", mutate: func(input *RunInput) { input.Workers = 0 }, want: "topology"},
		{name: "identity", mutate: func(input *RunInput) { input.Spec.RunID = "wrong" }, want: "identity"},
		{name: "producer", mutate: func(input *RunInput) { input.Producer.Messages = 1 }, want: "producer"},
		{name: "histogram", mutate: func(input *RunInput) { input.LatencyBefore = nil }, want: "histogram"},
		{name: "latency count", mutate: func(input *RunInput) { input.LatencyAfter["+Inf"]-- }, want: "durable latency"},
		{name: "outcome snapshot", mutate: func(input *RunInput) { delete(input.OutcomesAfter, "error") }, want: "counter snapshots"},
		{name: "outcome values", mutate: func(input *RunInput) { input.OutcomesAfter["duplicate"]++ }, want: "lossless"},
		{name: "lag", mutate: func(input *RunInput) { input.Samples = nil }, want: "lag samples"},
		{name: "ordering", mutate: func(input *RunInput) { input.Ordering.OrderingValid = false }, want: "ordering"},
		{name: "archive", mutate: func(input *RunInput) { input.ArchiveCount-- }, want: "archive contains"},
		{name: "resources", mutate: func(input *RunInput) { delete(input.Resources, "kafka") }, want: "kafka"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validRunInput()
			tt.mutate(&input)
			if _, err := BuildRunReport(input); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildRunReport() error = %v", err)
			}
		})
	}
}

func TestPrometheusValueAndFinitePredicates(t *testing.T) {
	if _, err := prometheusValue([]json.RawMessage{[]byte("1")}); err == nil {
		t.Fatal("prometheusValue() accepted a short sample")
	}
	if _, err := prometheusValue([]json.RawMessage{[]byte("1"), []byte("2")}); err == nil {
		t.Fatal("prometheusValue() accepted a non-string value")
	}
	if _, err := prometheusValue([]json.RawMessage{[]byte("1"), []byte(`"bad"`)}); err == nil {
		t.Fatal("prometheusValue() accepted a non-number")
	}
	if !finiteNonNegative(0) || finiteNonNegative(math.Inf(1)) || finiteNonNegative(math.NaN()) || finitePositive(0) || !finitePositive(1) {
		t.Fatal("finite predicates returned an invalid result")
	}
}

func producerLog(t *testing.T, messages, targetRate int64) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"msg": "load scenario completed", "messages": messages,
		"target_rate_events_per_second": targetRate,
		"duration_milliseconds":         1000.0, "throughput_events_per_second": 600.0,
		"ack_p50_milliseconds": 1.0, "ack_p95_milliseconds": 2.0, "ack_p99_milliseconds": 3.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func vectorJSON(t *testing.T, label string, values map[string]float64) []byte {
	t.Helper()
	results := make([]map[string]any, 0, len(values))
	for key, value := range values {
		results = append(results, map[string]any{
			"metric": map[string]string{label: key},
			"value":  []any{1, jsonNumber(value)},
		})
	}
	encoded, err := json.Marshal(map[string]any{"status": "success", "data": map[string]any{"result": results}})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func rangeJSON(t *testing.T, values ...float64) []byte {
	t.Helper()
	samples := make([][]any, len(values))
	for index, value := range values {
		samples[index] = []any{index + 1, jsonNumber(value)}
	}
	encoded, err := json.Marshal(map[string]any{
		"status": "success",
		"data":   map[string]any{"result": []any{map[string]any{"values": samples}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func jsonNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func validLagSamples() []LagSample {
	return []LagSample{
		{ElapsedMilliseconds: 1, AvailableReplicas: 3, Lag: scale.LagReport{PartitionCount: 3, LagByPartition: map[int32]int64{0: 0, 1: 0, 2: 0}}},
		{ElapsedMilliseconds: 2, AvailableReplicas: 3, ProducerComplete: true, Lag: scale.LagReport{PartitionCount: 3, TotalLag: 10, MaxPartitionLag: 5, LagByPartition: map[int32]int64{0: 5, 1: 3, 2: 2}}},
		{ElapsedMilliseconds: 5, AvailableReplicas: 3, ProducerComplete: true, Lag: scale.LagReport{PartitionCount: 3, LagByPartition: map[int32]int64{0: 0, 1: 0, 2: 0}}},
	}
}

func validRunInput() RunInput {
	beforeLatency := map[string]float64{"1": 10, "2": 20, "+Inf": 20}
	afterLatency := map[string]float64{"1": 310, "2": 610, "+Inf": 620}
	beforeOutcomes := map[string]float64{"created": 5, "duplicate": 2, "quarantined": 1, "error": 1}
	afterOutcomes := map[string]float64{"created": 605, "duplicate": 2, "quarantined": 1, "error": 1}
	resources := map[string]ResourcePeak{}
	for _, component := range requiredResourceComponents {
		resources[component] = ResourcePeak{CPUAvailable: true, PeakCPUCores: 0.5, MemoryAvailable: true, PeakMemoryBytes: 1024}
	}
	return RunInput{
		Spec:                          RunSpec{SuiteID: "cap", RunID: "cap-e600-r1", EventCount: 600, Repetition: 1},
		Partitions:                    3,
		Workers:                       3,
		DurableCompletionMilliseconds: 3000,
		Producer: ProducerSummary{
			Messages: 600, DurationMilliseconds: 1000, ThroughputEventsPerSecond: 600,
			AcknowledgementP50Milliseconds: 1, AcknowledgementP95Milliseconds: 2, AcknowledgementP99Milliseconds: 3,
		},
		LatencyBefore:  beforeLatency,
		LatencyAfter:   afterLatency,
		OutcomesBefore: beforeOutcomes,
		OutcomesAfter:  afterOutcomes,
		Samples:        validLagSamples(),
		Ordering: scale.Report{
			RunID: "cap-e600-r1", Phase: "original", ExpectedRecords: 600, ObservedRecords: 600,
			PartitionCount: 3, RecordsByPartition: map[int32]int64{0: 200, 1: 200, 2: 200}, OrderingValid: true,
		},
		ArchiveCount: 600,
		Resources:    resources,
	}
}
