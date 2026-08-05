package benchmark

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
)

var (
	gitRevisionPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	architecturePattern = regexp.MustCompile(`^(amd64|arm64|x86_64|aarch64)$`)
	versionPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+:-]*$`)
)

type ComponentVersions struct {
	Go         string `json:"go"`
	Kubernetes string `json:"kubernetes"`
	Kind       string `json:"kind"`
	Kafka      string `json:"kafka"`
	Strimzi    string `json:"strimzi"`
	KEDA       string `json:"keda"`
	Garage     string `json:"garage"`
}

type Environment struct {
	GitRevision        string            `json:"git_revision"`
	TrackedTreeClean   bool              `json:"tracked_tree_clean"`
	Architecture       string            `json:"architecture"`
	AllocatedCPUs      int               `json:"allocated_cpus"`
	AllocatedMemoryGiB int               `json:"allocated_memory_gib"`
	AllocatedDiskGiB   int               `json:"allocated_disk_gib"`
	KubernetesNodes    int               `json:"kubernetes_nodes"`
	Versions           ComponentVersions `json:"versions"`
}

type Distribution struct {
	Minimum float64 `json:"minimum"`
	Median  float64 `json:"median"`
	P95     float64 `json:"p95"`
	Maximum float64 `json:"maximum"`
}

type ResourceDistribution struct {
	PeakCPUCores    Distribution `json:"peak_cpu_cores"`
	PeakMemoryBytes Distribution `json:"peak_memory_bytes"`
}

type ScenarioSummary struct {
	EventCount                        int64                           `json:"event_count"`
	TargetRateEventsPerSecond         int64                           `json:"target_rate_events_per_second"`
	Repetitions                       int                             `json:"repetitions"`
	ProducerThroughputEventsPerSecond Distribution                    `json:"producer_throughput_events_per_second"`
	DurableThroughputEventsPerSecond  Distribution                    `json:"durable_throughput_events_per_second"`
	ProducerAcknowledgementP50Millis  Distribution                    `json:"producer_acknowledgement_p50_milliseconds"`
	ProducerAcknowledgementP95Millis  Distribution                    `json:"producer_acknowledgement_p95_milliseconds"`
	ProducerAcknowledgementP99Millis  Distribution                    `json:"producer_acknowledgement_p99_milliseconds"`
	DurableLatencyP50Seconds          Distribution                    `json:"durable_latency_p50_seconds"`
	DurableLatencyP95Seconds          Distribution                    `json:"durable_latency_p95_seconds"`
	DurableLatencyP99Seconds          Distribution                    `json:"durable_latency_p99_seconds"`
	DurableCompletionMilliseconds     Distribution                    `json:"durable_completion_milliseconds"`
	MaximumTotalLag                   Distribution                    `json:"maximum_total_lag"`
	ObservedLagDrainMilliseconds      Distribution                    `json:"observed_lag_drain_milliseconds"`
	PostProducerDrainMilliseconds     Distribution                    `json:"post_producer_drain_milliseconds"`
	ResourceMeasurementsRequired      bool                            `json:"resource_measurements_required"`
	Resources                         map[string]ResourceDistribution `json:"resources,omitempty"`
	AllAssertionsPassed               bool                            `json:"all_assertions_passed"`
}

type Summary struct {
	SchemaVersion int               `json:"schema_version"`
	EvidenceScope string            `json:"evidence_scope"`
	SuiteID       string            `json:"suite_id"`
	Environment   Environment       `json:"environment"`
	Scenarios     []ScenarioSummary `json:"scenarios"`
	Limitations   []string          `json:"limitations"`
}

func BuildSummary(plan Plan, environment Environment, reports []RunReport) (Summary, error) {
	if err := validatePlan(plan); err != nil {
		return Summary{}, err
	}
	if err := validateEnvironment(environment); err != nil {
		return Summary{}, err
	}
	if len(reports) != len(plan.Runs) {
		return Summary{}, fmt.Errorf("received %d run reports, want %d", len(reports), len(plan.Runs))
	}
	byRunID := make(map[string]RunReport, len(reports))
	for _, report := range reports {
		if _, exists := byRunID[report.RunID]; exists {
			return Summary{}, fmt.Errorf("run report %q is duplicated", report.RunID)
		}
		byRunID[report.RunID] = report
	}

	summary := Summary{
		SchemaVersion: SchemaVersion,
		EvidenceScope: EvidenceScope,
		SuiteID:       plan.SuiteID,
		Environment:   environment,
		Scenarios:     make([]ScenarioSummary, 0, len(plan.EventCounts)),
		Limitations: []string{
			"Results describe one disposable local kind environment, not AWS or production capacity.",
			"The Kafka broker and S3-compatible object store each use one replica.",
			"Host contention can affect local results; compare distributions from equivalent environments.",
			producerScopeLimitation(plan.TargetRate),
		},
	}
	for _, eventCount := range plan.EventCounts {
		if !resourceMeasurementsRequired(eventCount, plan.TargetRate) {
			summary.Limitations = append(summary.Limitations, resourceOmissionLimitation())
			break
		}
	}
	for _, eventCount := range plan.EventCounts {
		group := make([]RunReport, 0, plan.Repetitions)
		for repetition := 1; repetition <= plan.Repetitions; repetition++ {
			runID := fmt.Sprintf("%s-e%d-r%d", plan.SuiteID, eventCount, repetition)
			report, exists := byRunID[runID]
			if !exists {
				return Summary{}, fmt.Errorf("run report %q is missing", runID)
			}
			if err := validateRunForSummary(report, plan, eventCount, repetition); err != nil {
				return Summary{}, err
			}
			group = append(group, report)
		}
		summary.Scenarios = append(summary.Scenarios, summarizeScenario(eventCount, plan.TargetRate, group))
	}
	return summary, nil
}

func distribution(values []float64) Distribution {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	median := sorted[middle]
	if len(sorted)%2 == 0 {
		median = (sorted[middle-1] + sorted[middle]) / 2
	}
	p95Index := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	return Distribution{Minimum: sorted[0], Median: median, P95: sorted[p95Index], Maximum: sorted[len(sorted)-1]}
}

func summarizeScenario(eventCount, targetRate int64, reports []RunReport) ScenarioSummary {
	values := func(selectValue func(RunReport) float64) []float64 {
		result := make([]float64, len(reports))
		for index, report := range reports {
			result[index] = selectValue(report)
		}
		return result
	}
	scenario := ScenarioSummary{
		EventCount:                        eventCount,
		TargetRateEventsPerSecond:         targetRate,
		Repetitions:                       len(reports),
		ProducerThroughputEventsPerSecond: distribution(values(func(r RunReport) float64 { return r.Measurements.Producer.ThroughputEventsPerSecond })),
		DurableThroughputEventsPerSecond:  distribution(values(func(r RunReport) float64 { return r.Measurements.DurableThroughputEventsPerSecond })),
		ProducerAcknowledgementP50Millis:  distribution(values(func(r RunReport) float64 { return r.Measurements.Producer.AcknowledgementP50Milliseconds })),
		ProducerAcknowledgementP95Millis:  distribution(values(func(r RunReport) float64 { return r.Measurements.Producer.AcknowledgementP95Milliseconds })),
		ProducerAcknowledgementP99Millis:  distribution(values(func(r RunReport) float64 { return r.Measurements.Producer.AcknowledgementP99Milliseconds })),
		DurableLatencyP50Seconds:          distribution(values(func(r RunReport) float64 { return r.Measurements.DurableLatency.P50Seconds })),
		DurableLatencyP95Seconds:          distribution(values(func(r RunReport) float64 { return r.Measurements.DurableLatency.P95Seconds })),
		DurableLatencyP99Seconds:          distribution(values(func(r RunReport) float64 { return r.Measurements.DurableLatency.P99Seconds })),
		DurableCompletionMilliseconds:     distribution(values(func(r RunReport) float64 { return float64(r.Measurements.DurableCompletionMilliseconds) })),
		MaximumTotalLag:                   distribution(values(func(r RunReport) float64 { return float64(r.Measurements.Lag.MaximumTotalLag) })),
		ObservedLagDrainMilliseconds:      distribution(values(func(r RunReport) float64 { return float64(r.Measurements.Lag.ObservedLagDrainMilliseconds) })),
		PostProducerDrainMilliseconds:     distribution(values(func(r RunReport) float64 { return float64(r.Measurements.Lag.PostProducerDrainMilliseconds) })),
		ResourceMeasurementsRequired:      resourceMeasurementsRequired(eventCount, targetRate),
		AllAssertionsPassed:               true,
	}
	if scenario.ResourceMeasurementsRequired {
		scenario.Resources = make(map[string]ResourceDistribution, len(requiredResourceComponents))
		for _, component := range requiredResourceComponents {
			scenario.Resources[component] = ResourceDistribution{
				PeakCPUCores:    distribution(values(func(r RunReport) float64 { return r.Measurements.Resources[component].PeakCPUCores })),
				PeakMemoryBytes: distribution(values(func(r RunReport) float64 { return r.Measurements.Resources[component].PeakMemoryBytes })),
			}
		}
	}
	return scenario
}

func validateEnvironment(environment Environment) error {
	versions := []string{
		environment.Versions.Go,
		environment.Versions.Kubernetes,
		environment.Versions.Kind,
		environment.Versions.Kafka,
		environment.Versions.Strimzi,
		environment.Versions.KEDA,
		environment.Versions.Garage,
	}
	if !gitRevisionPattern.MatchString(environment.GitRevision) ||
		!architecturePattern.MatchString(environment.Architecture) || environment.AllocatedCPUs < 1 ||
		environment.AllocatedMemoryGiB < 1 || environment.AllocatedDiskGiB < 1 || environment.KubernetesNodes < 1 {
		return errors.New("benchmark environment is invalid")
	}
	for _, version := range versions {
		if !versionPattern.MatchString(version) {
			return errors.New("benchmark component version is invalid")
		}
	}
	return nil
}

func validateRunForSummary(report RunReport, plan Plan, eventCount int64, repetition int) error {
	expectedRunID := fmt.Sprintf("%s-e%d-r%d", plan.SuiteID, eventCount, repetition)
	resourcesRequired := resourceMeasurementsRequired(eventCount, plan.TargetRate)
	if report.SchemaVersion != SchemaVersion || report.EvidenceScope != EvidenceScope || report.SuiteID != plan.SuiteID || report.RunID != expectedRunID ||
		report.Config.Events != eventCount || report.Config.Repetition != repetition || report.Config.TargetRate != plan.TargetRate ||
		report.Config.ResourceMeasurementsRequired != resourcesRequired ||
		report.Config.TopicPartitions < 1 || report.Config.ConsumerReplicas < 1 || report.Config.ArchiveDelayMillis != 0 ||
		report.Measurements.Producer.Messages != eventCount || report.Measurements.Producer.TargetRateEventsPerSecond != plan.TargetRate ||
		report.Measurements.DurableLatency.Observations != eventCount || !finitePositive(report.Measurements.Producer.ThroughputEventsPerSecond) ||
		!finitePositive(report.Measurements.DurableThroughputEventsPerSecond) ||
		report.Assertions.BrokerAcknowledgedEvents != eventCount || report.Assertions.TopicObservedEvents != eventCount ||
		report.Assertions.ArchiveCreatedEvents != eventCount || report.Assertions.UniqueArchiveObjects != eventCount ||
		report.Assertions.OutcomeDeltas["created"] != eventCount || report.Assertions.OutcomeDeltas["duplicate"] != 0 ||
		report.Assertions.OutcomeDeltas["quarantined"] != 0 || report.Assertions.OutcomeDeltas["error"] != 0 ||
		!report.Assertions.OrderingValid || !report.Assertions.NoLoss || !report.Assertions.NoUnexpectedDuplicates ||
		!report.Assertions.NoQuarantineOrErrors {
		return fmt.Errorf("run report %q is incompatible or failed its assertions", report.RunID)
	}
	if resourcesRequired {
		for _, component := range requiredResourceComponents {
			resource, exists := report.Measurements.Resources[component]
			if !exists || !resource.CPUAvailable || !resource.MemoryAvailable ||
				!finiteNonNegative(resource.PeakCPUCores) || !finiteNonNegative(resource.PeakMemoryBytes) {
				return fmt.Errorf("run report %q has invalid %s resources", report.RunID, component)
			}
		}
		if len(report.Measurements.Resources) != len(requiredResourceComponents) {
			return fmt.Errorf("run report %q has an invalid resource component set", report.RunID)
		}
	} else if len(report.Measurements.Resources) != 0 {
		return fmt.Errorf("run report %q must omit resources", report.RunID)
	}
	return nil
}
