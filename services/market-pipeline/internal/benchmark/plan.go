package benchmark

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	SchemaVersion  = 1
	EvidenceScope  = "local_kind_synthetic_capacity"
	minEventCount  = 600
	maxEventCount  = 1_000_000
	maxRepetitions = 9
	maxTargetRate  = 100_000
)

var suiteIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,9}$`)

type Plan struct {
	SchemaVersion int       `json:"schema_version"`
	EvidenceScope string    `json:"evidence_scope"`
	SuiteID       string    `json:"suite_id"`
	EventCounts   []int64   `json:"event_counts"`
	Repetitions   int       `json:"repetitions"`
	TargetRate    int64     `json:"target_rate_events_per_second"`
	Runs          []RunSpec `json:"runs"`
}

type RunSpec struct {
	SuiteID    string `json:"suite_id"`
	RunID      string `json:"run_id"`
	EventCount int64  `json:"event_count"`
	Repetition int    `json:"repetition"`
	TargetRate int64  `json:"target_rate_events_per_second"`
}

func NewPlan(suiteID, eventCounts string, repetitions int, targetRate int64) (Plan, error) {
	if !suiteIDPattern.MatchString(suiteID) {
		return Plan{}, errors.New("suite ID must contain 1-10 lowercase letters, digits, or hyphens")
	}
	if repetitions < 1 || repetitions > maxRepetitions {
		return Plan{}, fmt.Errorf("repetitions must be between 1 and %d", maxRepetitions)
	}
	if targetRate < 0 || targetRate > maxTargetRate {
		return Plan{}, fmt.Errorf("target rate must be between 0 and %d", maxTargetRate)
	}
	if strings.TrimSpace(eventCounts) == "" {
		return Plan{}, errors.New("at least one event count is required")
	}
	parts := strings.Split(eventCounts, ",")
	counts := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || value < minEventCount || value > maxEventCount {
			return Plan{}, fmt.Errorf("event counts must be comma-separated integers between %d and %d", minEventCount, maxEventCount)
		}
		if _, exists := seen[value]; exists {
			return Plan{}, fmt.Errorf("event count %d is duplicated", value)
		}
		seen[value] = struct{}{}
		counts = append(counts, value)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i] < counts[j] })

	plan := Plan{
		SchemaVersion: SchemaVersion,
		EvidenceScope: EvidenceScope,
		SuiteID:       suiteID,
		EventCounts:   counts,
		Repetitions:   repetitions,
		TargetRate:    targetRate,
		Runs:          make([]RunSpec, 0, len(counts)*repetitions),
	}
	for _, count := range counts {
		for repetition := 1; repetition <= repetitions; repetition++ {
			runID := fmt.Sprintf("%s-e%d-r%d", suiteID, count, repetition)
			plan.Runs = append(plan.Runs, RunSpec{
				SuiteID:    suiteID,
				RunID:      runID,
				EventCount: count,
				Repetition: repetition,
				TargetRate: targetRate,
			})
		}
	}
	return plan, nil
}

func (p Plan) Run(runID string) (RunSpec, error) {
	for _, run := range p.Runs {
		if run.RunID == runID {
			return run, nil
		}
	}
	return RunSpec{}, fmt.Errorf("run %q is not in the benchmark plan", runID)
}
