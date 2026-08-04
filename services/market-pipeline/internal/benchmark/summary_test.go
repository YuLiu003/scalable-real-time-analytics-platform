package benchmark

import (
	"strings"
	"testing"
)

func TestBuildSummaryAggregatesEveryRunWithoutBestRunSelection(t *testing.T) {
	plan, err := NewPlan("cap", "1000,600", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	reports := []RunReport{
		summaryReport(plan.Runs[3], 4),
		summaryReport(plan.Runs[0], 1),
		summaryReport(plan.Runs[2], 3),
		summaryReport(plan.Runs[1], 2),
	}
	summary, err := BuildSummary(plan, validEnvironment(), reports)
	if err != nil {
		t.Fatal(err)
	}
	if summary.SchemaVersion != SchemaVersion || summary.EvidenceScope != EvidenceScope ||
		len(summary.Scenarios) != 2 || len(summary.Limitations) != 4 ||
		summary.Limitations[3] != "Unbounded production measures burst completion, not a sustained provider feed." {
		t.Fatalf("BuildSummary() = %+v", summary)
	}
	first := summary.Scenarios[0]
	if first.EventCount != 600 || first.Repetitions != 2 || !first.AllAssertionsPassed ||
		first.ProducerThroughputEventsPerSecond.Minimum != 10 ||
		first.ProducerThroughputEventsPerSecond.Median != 15 ||
		first.ProducerThroughputEventsPerSecond.P95 != 20 ||
		first.Resources["kafka"].PeakMemoryBytes.Maximum != 1024 {
		t.Fatalf("first scenario = %+v", first)
	}
	odd := distribution([]float64{9, 1, 5})
	if odd.Minimum != 1 || odd.Median != 5 || odd.P95 != 9 || odd.Maximum != 9 {
		t.Fatalf("distribution() = %+v", odd)
	}
	pacedPlan, err := NewPlan("paced", "600", 1, 250)
	if err != nil {
		t.Fatal(err)
	}
	paced, err := BuildSummary(pacedPlan, validEnvironment(), []RunReport{summaryReport(pacedPlan.Runs[0], 1)})
	if err != nil || paced.Limitations[3] != "Rate-controlled production validates the configured arrival rate, not burst capacity or a sustained provider feed." {
		t.Fatalf("paced BuildSummary() = %+v, %v", paced, err)
	}
}

func TestBuildSummaryRejectsInvalidEvidence(t *testing.T) {
	plan, err := NewPlan("cap", "600", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	validReports := []RunReport{summaryReport(plan.Runs[0], 1), summaryReport(plan.Runs[1], 2)}
	tests := []struct {
		name        string
		plan        Plan
		environment Environment
		reports     []RunReport
		want        string
	}{
		{name: "plan", plan: Plan{}, environment: validEnvironment(), want: "plan"},
		{name: "environment", plan: plan, environment: Environment{}, reports: validReports, want: "environment"},
		{name: "count", plan: plan, environment: validEnvironment(), reports: validReports[:1], want: "received"},
		{name: "duplicate", plan: plan, environment: validEnvironment(), reports: []RunReport{validReports[0], validReports[0]}, want: "duplicated"},
		{name: "missing", plan: plan, environment: validEnvironment(), reports: []RunReport{validReports[0], summaryReport(RunSpec{SuiteID: "cap", RunID: "cap-e700-r2", EventCount: 700, Repetition: 2}, 2)}, want: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildSummary(tt.plan, tt.environment, tt.reports)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildSummary() error = %v", err)
			}
		})
	}

	badReport := summaryReport(plan.Runs[0], 1)
	badReport.Assertions.NoLoss = false
	if err := validateRunForSummary(badReport, plan, 600, 1); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("validateRunForSummary(assertions) error = %v", err)
	}
	if _, err := BuildSummary(plan, validEnvironment(), []RunReport{badReport, validReports[1]}); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("BuildSummary(invalid run) error = %v", err)
	}
	badResources := summaryReport(plan.Runs[0], 1)
	delete(badResources.Measurements.Resources, "archiver")
	if err := validateRunForSummary(badResources, plan, 600, 1); err == nil || !strings.Contains(err.Error(), "archiver") {
		t.Fatalf("validateRunForSummary(resources) error = %v", err)
	}
}

func TestEnvironmentValidationRejectsUnsafeOrMissingMetadata(t *testing.T) {
	valid := validEnvironment()
	if err := validateEnvironment(valid); err != nil {
		t.Fatal(err)
	}
	invalidBase := valid
	invalidBase.Architecture = "personal-host"
	if err := validateEnvironment(invalidBase); err == nil || !strings.Contains(err.Error(), "environment") {
		t.Fatalf("validateEnvironment(base) error = %v", err)
	}
	invalidVersion := valid
	invalidVersion.Versions.Kafka = "private value"
	if err := validateEnvironment(invalidVersion); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("validateEnvironment(version) error = %v", err)
	}
}

func summaryReport(spec RunSpec, factor float64) RunReport {
	resources := map[string]ResourcePeak{}
	for _, component := range requiredResourceComponents {
		resources[component] = ResourcePeak{
			CPUAvailable: true, PeakCPUCores: factor / 10,
			MemoryAvailable: true, PeakMemoryBytes: factor * 512,
		}
	}
	return RunReport{
		SchemaVersion: SchemaVersion, EvidenceScope: EvidenceScope, SuiteID: spec.SuiteID, RunID: spec.RunID,
		Config: RunConfig{
			Events: spec.EventCount, Repetition: spec.Repetition, TargetRate: spec.TargetRate,
			TopicPartitions: 3, ConsumerReplicas: 3,
		},
		Measurements: RunMeasurements{
			Producer: ProducerSummary{
				Messages: spec.EventCount, TargetRateEventsPerSecond: spec.TargetRate,
				DurationMilliseconds: 1000, ThroughputEventsPerSecond: factor * 10,
				AcknowledgementP50Milliseconds: factor, AcknowledgementP95Milliseconds: factor * 2,
				AcknowledgementP99Milliseconds: factor * 3,
			},
			DurableCompletionMilliseconds:    int64(factor * 1000),
			DurableThroughputEventsPerSecond: factor * 5,
			DurableLatency: LatencyQuantiles{
				Observations: spec.EventCount, P50Seconds: factor / 10, P95Seconds: factor / 5, P99Seconds: factor / 4,
			},
			Lag: LagSummary{
				MaximumTotalLag: int64(factor * 10), ObservedLagDrainMilliseconds: int64(factor * 100),
				PostProducerDrainMilliseconds: int64(factor * 50),
			},
			Resources: resources,
		},
		Assertions: RunAssertions{
			BrokerAcknowledgedEvents: spec.EventCount, TopicObservedEvents: spec.EventCount,
			ArchiveCreatedEvents: spec.EventCount, UniqueArchiveObjects: spec.EventCount,
			OutcomeDeltas: map[string]int64{"created": spec.EventCount, "duplicate": 0, "quarantined": 0, "error": 0},
			OrderingValid: true, NoLoss: true, NoUnexpectedDuplicates: true, NoQuarantineOrErrors: true,
		},
	}
}

func validEnvironment() Environment {
	return Environment{
		GitRevision: strings.Repeat("a", 40), TrackedTreeClean: true, Architecture: "arm64",
		AllocatedCPUs: 4, AllocatedMemoryGiB: 8, AllocatedDiskGiB: 30, KubernetesNodes: 3,
		Versions: ComponentVersions{
			Go: "1.25.12", Kubernetes: "v1.33.7", Kind: "v0.31.0", Kafka: "4.2.1",
			Strimzi: "1.1.0", KEDA: "2.20.0", Garage: "v2.3.0",
		},
	}
}
