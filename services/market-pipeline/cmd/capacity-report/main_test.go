package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/benchmark"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/scale"
)

func TestCommandsBuildRunAndSuiteReports(t *testing.T) {
	directory := t.TempDir()
	planPath := filepath.Join(directory, "plan.json")
	matrixPath := filepath.Join(directory, "runs.tsv")
	if err := run([]string{
		"plan", "--suite-id", "cap", "--event-counts", "600", "--repetitions", "1",
		"--output", planPath, "--runs-output", matrixPath,
	}); err != nil {
		t.Fatal(err)
	}
	if matrix := readText(t, matrixPath); matrix != "cap-e600-r1\t600\t1\t0\ttrue\n" {
		t.Fatalf("matrix = %q", matrix)
	}

	rawDir := writeRawRun(t, directory, "cap-e600-r1", 600, 0, true)
	reportPath := filepath.Join(directory, "runs", "cap-e600-r1", "report.json")
	if err := run([]string{
		"run", "--plan", planPath, "--run-id", "cap-e600-r1", "--raw-dir", rawDir,
		"--duration-ms", "3000", "--output", reportPath,
	}); err != nil {
		t.Fatal(err)
	}
	var report benchmark.RunReport
	readJSONFile(t, reportPath, &report)
	if report.Assertions.UniqueArchiveObjects != 600 || report.Measurements.DurableThroughputEventsPerSecond != 200 {
		t.Fatalf("run report = %+v", report)
	}

	environmentPath := filepath.Join(directory, "environment.json")
	writeJSON(t, environmentPath, benchmark.Environment{
		GitRevision: strings.Repeat("a", 40), TrackedTreeClean: true, Architecture: "arm64",
		AllocatedCPUs: 4, AllocatedMemoryGiB: 8, AllocatedDiskGiB: 30, KubernetesNodes: 3,
		Versions: benchmark.ComponentVersions{
			Go: "1.25.12", Kubernetes: "v1.33.7", Kind: "v0.31.0", Kafka: "4.2.1",
			Strimzi: "1.1.0", KEDA: "2.20.0", Garage: "v2.3.0",
		},
	})
	summaryPath := filepath.Join(directory, "summary.json")
	if err := run([]string{
		"summary", "--plan", planPath, "--environment", environmentPath,
		"--runs-dir", filepath.Join(directory, "runs"), "--output", summaryPath,
	}); err != nil {
		t.Fatal(err)
	}
	var summary benchmark.Summary
	readJSONFile(t, summaryPath, &summary)
	if len(summary.Scenarios) != 1 || summary.Scenarios[0].DurableThroughputEventsPerSecond.Median != 200 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestCommandOmitsResourcesForShortUnboundedRun(t *testing.T) {
	directory := t.TempDir()
	planPath := filepath.Join(directory, "plan.json")
	matrixPath := filepath.Join(directory, "runs.tsv")
	if err := run([]string{
		"plan", "--suite-id", "burst", "--event-counts", "10000", "--repetitions", "1",
		"--output", planPath, "--runs-output", matrixPath,
	}); err != nil {
		t.Fatal(err)
	}
	if matrix := readText(t, matrixPath); matrix != "burst-e10000-r1\t10000\t1\t0\tfalse\n" {
		t.Fatalf("matrix = %q", matrix)
	}
	rawDir := writeRawRun(t, directory, "burst-e10000-r1", 10_000, 0, false)
	reportPath := filepath.Join(directory, "runs", "burst-e10000-r1", "report.json")
	if err := run([]string{
		"run", "--plan", planPath, "--run-id", "burst-e10000-r1", "--raw-dir", rawDir,
		"--duration-ms", "3000", "--output", reportPath,
	}); err != nil {
		t.Fatal(err)
	}
	var report benchmark.RunReport
	readJSONFile(t, reportPath, &report)
	if report.Config.ResourceMeasurementsRequired || report.Measurements.Resources != nil ||
		strings.Contains(readText(t, reportPath), `"resources"`) {
		t.Fatalf("short unbounded report = %+v", report)
	}
}

func TestCommandRequiresResourcesForPacedTenThousand(t *testing.T) {
	directory := t.TempDir()
	planPath := filepath.Join(directory, "plan.json")
	if err := run([]string{
		"plan", "--suite-id", "paced", "--event-counts", "10000", "--repetitions", "1", "--target-rate", "250",
		"--output", planPath, "--runs-output", filepath.Join(directory, "runs.tsv"),
	}); err != nil {
		t.Fatal(err)
	}
	rawDir := writeRawRun(t, directory, "paced-e10000-r1", 10_000, 250, false)
	err := run([]string{
		"run", "--plan", planPath, "--run-id", "paced-e10000-r1", "--raw-dir", rawDir,
		"--duration-ms", "30000", "--output", filepath.Join(directory, "report.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "resource-archiver-cpu.json") {
		t.Fatalf("paced run missing resources error = %v", err)
	}
}

func TestCommandRejectsMissingOrUnknownOperation(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("run() accepted no command")
	}
	if err := run([]string{"unknown"}); err == nil {
		t.Fatal("run() accepted an unknown command")
	}
}

func vector(label string, values map[string]float64) map[string]any {
	result := make([]map[string]any, 0, len(values))
	for key, value := range values {
		result = append(result, map[string]any{
			"metric": map[string]string{label: key}, "value": []any{1, number(value)},
		})
	}
	return map[string]any{"status": "success", "data": map[string]any{"result": result}}
}

func rangeVector(values ...float64) map[string]any {
	samples := make([][]any, len(values))
	for index, value := range values {
		samples[index] = []any{index + 1, number(value)}
	}
	return map[string]any{"status": "success", "data": map[string]any{"result": []any{map[string]any{"values": samples}}}}
}

func writeRawRun(t *testing.T, directory, runID string, events, targetRate int64, includeResources bool) string {
	t.Helper()
	rawDir := filepath.Join(directory, "runs", runID, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(rawDir, "producer.jsonl"), fmt.Sprintf(
		`{"msg":"load scenario completed","messages":%d,"target_rate_events_per_second":%d,"duration_milliseconds":1000,"throughput_events_per_second":600,"ack_p50_milliseconds":1,"ack_p95_milliseconds":2,"ack_p99_milliseconds":3}`+"\n",
		events, targetRate,
	))
	writeJSON(t, filepath.Join(rawDir, "latency-before.json"), vector("le", map[string]float64{"1": 10, "2": 20, "+Inf": 20}))
	writeJSON(t, filepath.Join(rawDir, "latency-after.json"), vector("le", map[string]float64{
		"1": 10 + float64(events)/2, "2": 20 + float64(events), "+Inf": 20 + float64(events),
	}))
	writeJSON(t, filepath.Join(rawDir, "outcomes-before.json"), vector("outcome", map[string]float64{"created": 5, "duplicate": 2, "quarantined": 1, "error": 1}))
	writeJSON(t, filepath.Join(rawDir, "outcomes-after.json"), vector("outcome", map[string]float64{"created": 5 + float64(events), "duplicate": 2, "quarantined": 1, "error": 1}))
	samples := []benchmark.LagSample{
		{ElapsedMilliseconds: 1, AvailableReplicas: 3, Lag: scale.LagReport{PartitionCount: 3, TotalLag: 3, MaxPartitionLag: 1, LagByPartition: map[int32]int64{0: 1, 1: 1, 2: 1}}},
		{ElapsedMilliseconds: 2, AvailableReplicas: 3, ProducerComplete: true, Lag: scale.LagReport{PartitionCount: 3, LagByPartition: map[int32]int64{0: 0, 1: 0, 2: 0}}},
	}
	var sampleLines strings.Builder
	for _, sample := range samples {
		encoded, _ := json.Marshal(sample)
		sampleLines.Write(encoded)
		sampleLines.WriteByte('\n')
	}
	writeText(t, filepath.Join(rawDir, "samples.jsonl"), sampleLines.String())
	perPartition := events / 3
	writeJSON(t, filepath.Join(rawDir, "ordering.json"), scale.Report{
		RunID: runID, Phase: "original", ExpectedRecords: events, ObservedRecords: events,
		PartitionCount: 3, RecordsByPartition: map[int32]int64{0: perPartition, 1: perPartition, 2: events - 2*perPartition}, OrderingValid: true,
	})
	writeJSON(t, filepath.Join(rawDir, "archive.json"), map[string]any{"prefix": "privacy-safe", "count": events})
	if includeResources {
		for _, component := range []string{"archiver", "kafka", "object_store"} {
			writeJSON(t, filepath.Join(rawDir, "resource-"+component+"-cpu.json"), rangeVector(0.25, 0.5))
			writeJSON(t, filepath.Join(rawDir, "resource-"+component+"-memory.json"), rangeVector(1024, 2048))
		}
	}
	return rawDir
}

func number(value float64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeText(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}
