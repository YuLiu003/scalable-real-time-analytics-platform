package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/benchmark"
	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/scale"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("capacity-report requires plan, run, or summary")
	}
	switch arguments[0] {
	case "plan":
		return writePlan(arguments[1:])
	case "run":
		return writeRun(arguments[1:])
	case "summary":
		return writeSummary(arguments[1:])
	default:
		return fmt.Errorf("unknown capacity-report command %q", arguments[0])
	}
}

func writePlan(arguments []string) error {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	suiteID := flags.String("suite-id", "", "privacy-safe suite identifier")
	eventCounts := flags.String("event-counts", "10000,50000,100000", "comma-separated event counts")
	repetitions := flags.Int("repetitions", 5, "repetitions per event count")
	targetRate := flags.Int64("target-rate", 0, "producer target events per second; zero is unbounded")
	output := flags.String("output", "", "plan JSON path")
	runsOutput := flags.String("runs-output", "", "run matrix TSV path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *output == "" || *runsOutput == "" {
		return errors.New("--output and --runs-output are required")
	}
	plan, err := benchmark.NewPlan(*suiteID, *eventCounts, *repetitions, *targetRate)
	if err != nil {
		return err
	}
	if err := writeJSONAtomic(*output, plan); err != nil {
		return err
	}
	var rows bytes.Buffer
	for _, run := range plan.Runs {
		fmt.Fprintf(&rows, "%s\t%d\t%d\t%d\n", run.RunID, run.EventCount, run.Repetition, run.TargetRate)
	}
	return writeAtomic(*runsOutput, rows.Bytes())
}

func writeRun(arguments []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	planPath := flags.String("plan", "", "benchmark plan JSON path")
	runID := flags.String("run-id", "", "run identifier from the plan")
	rawDir := flags.String("raw-dir", "", "raw run artifact directory")
	duration := flags.Int64("duration-ms", 0, "submission-to-durable-completion milliseconds")
	partitions := flags.Int("partitions", 3, "topic partition count")
	workers := flags.Int("workers", 3, "fixed consumer replica count")
	output := flags.String("output", "", "run report JSON path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *planPath == "" || *runID == "" || *rawDir == "" || *output == "" {
		return errors.New("--plan, --run-id, --raw-dir, and --output are required")
	}
	var plan benchmark.Plan
	if err := readJSON(*planPath, &plan); err != nil {
		return err
	}
	spec, err := plan.Run(*runID)
	if err != nil {
		return err
	}
	producerData, err := os.ReadFile(filepath.Join(*rawDir, "producer.jsonl"))
	if err != nil {
		return err
	}
	producer, err := benchmark.DecodeProducerSummary(producerData)
	if err != nil {
		return err
	}
	latencyBefore, err := readVector(filepath.Join(*rawDir, "latency-before.json"), "le")
	if err != nil {
		return err
	}
	latencyAfter, err := readVector(filepath.Join(*rawDir, "latency-after.json"), "le")
	if err != nil {
		return err
	}
	outcomesBefore, err := readVector(filepath.Join(*rawDir, "outcomes-before.json"), "outcome")
	if err != nil {
		return err
	}
	outcomesAfter, err := readVector(filepath.Join(*rawDir, "outcomes-after.json"), "outcome")
	if err != nil {
		return err
	}
	sampleData, err := os.ReadFile(filepath.Join(*rawDir, "samples.jsonl"))
	if err != nil {
		return err
	}
	samples, err := benchmark.DecodeLagSamples(sampleData)
	if err != nil {
		return err
	}
	var ordering scale.Report
	if err := readJSON(filepath.Join(*rawDir, "ordering.json"), &ordering); err != nil {
		return err
	}
	var archive struct {
		Prefix string `json:"prefix"`
		Count  int64  `json:"count"`
	}
	if err := readJSON(filepath.Join(*rawDir, "archive.json"), &archive); err != nil {
		return err
	}
	resources := make(map[string]benchmark.ResourcePeak, 3)
	for _, component := range []string{"archiver", "kafka", "object_store"} {
		cpu, cpuAvailable, err := readRangePeak(filepath.Join(*rawDir, "resource-"+component+"-cpu.json"))
		if err != nil {
			return err
		}
		memory, memoryAvailable, err := readRangePeak(filepath.Join(*rawDir, "resource-"+component+"-memory.json"))
		if err != nil {
			return err
		}
		resources[component] = benchmark.ResourcePeak{
			CPUAvailable:    cpuAvailable,
			PeakCPUCores:    cpu,
			MemoryAvailable: memoryAvailable,
			PeakMemoryBytes: memory,
		}
	}
	report, err := benchmark.BuildRunReport(benchmark.RunInput{
		Spec:                          spec,
		Partitions:                    *partitions,
		Workers:                       *workers,
		DurableCompletionMilliseconds: *duration,
		Producer:                      producer,
		LatencyBefore:                 latencyBefore,
		LatencyAfter:                  latencyAfter,
		OutcomesBefore:                outcomesBefore,
		OutcomesAfter:                 outcomesAfter,
		Samples:                       samples,
		Ordering:                      ordering,
		ArchiveCount:                  archive.Count,
		Resources:                     resources,
	})
	if err != nil {
		return err
	}
	return writeJSONAtomic(*output, report)
}

func writeSummary(arguments []string) error {
	flags := flag.NewFlagSet("summary", flag.ContinueOnError)
	planPath := flags.String("plan", "", "benchmark plan JSON path")
	environmentPath := flags.String("environment", "", "environment JSON path")
	runsDir := flags.String("runs-dir", "", "directory containing per-run reports")
	output := flags.String("output", "", "summary JSON path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *planPath == "" || *environmentPath == "" || *runsDir == "" || *output == "" {
		return errors.New("--plan, --environment, --runs-dir, and --output are required")
	}
	var plan benchmark.Plan
	if err := readJSON(*planPath, &plan); err != nil {
		return err
	}
	var environment benchmark.Environment
	if err := readJSON(*environmentPath, &environment); err != nil {
		return err
	}
	reports := make([]benchmark.RunReport, 0, len(plan.Runs))
	for _, run := range plan.Runs {
		var report benchmark.RunReport
		if err := readJSON(filepath.Join(*runsDir, run.RunID, "report.json"), &report); err != nil {
			return err
		}
		reports = append(reports, report)
	}
	summary, err := benchmark.BuildSummary(plan, environment, reports)
	if err != nil {
		return err
	}
	return writeJSONAtomic(*output, summary)
}

func readVector(path, label string) (map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return benchmark.DecodeLabeledVector(data, label)
}

func readRangePeak(path string) (float64, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false, err
	}
	return benchmark.DecodeRangePeak(data)
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode %s: trailing JSON data", path)
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	return writeAtomic(path, encoded.Bytes())
}

func writeAtomic(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".capacity-report-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
