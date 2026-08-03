package benchmark

import (
	"strings"
	"testing"
)

func TestNewPlanBuildsSortedDeterministicMatrix(t *testing.T) {
	plan, err := NewPlan("c123456789", "50000, 10000", 2, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SchemaVersion || plan.EvidenceScope != EvidenceScope ||
		len(plan.Runs) != 4 || plan.EventCounts[0] != 10000 || plan.EventCounts[1] != 50000 {
		t.Fatalf("NewPlan() = %+v", plan)
	}
	wantIDs := []string{
		"c123456789-e10000-r1",
		"c123456789-e10000-r2",
		"c123456789-e50000-r1",
		"c123456789-e50000-r2",
	}
	for index, want := range wantIDs {
		if plan.Runs[index].SuiteID != plan.SuiteID || plan.Runs[index].RunID != want ||
			plan.Runs[index].TargetRate != 1000 {
			t.Fatalf("run %d = %+v", index, plan.Runs[index])
		}
	}
	run, err := plan.Run(wantIDs[2])
	if err != nil || run.EventCount != 50000 || run.Repetition != 1 {
		t.Fatalf("Run() = %+v, %v", run, err)
	}
	if _, err := plan.Run("missing"); err == nil {
		t.Fatal("Run() accepted an unknown ID")
	}
}

func TestNewPlanRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		suiteID     string
		eventCounts string
		repetitions int
		targetRate  int64
		want        string
	}{
		{name: "suite", suiteID: "PRIVATE", eventCounts: "10000", repetitions: 1, want: "suite ID"},
		{name: "repetitions low", suiteID: "cap", eventCounts: "10000", repetitions: 0, want: "repetitions"},
		{name: "repetitions high", suiteID: "cap", eventCounts: "10000", repetitions: 10, want: "repetitions"},
		{name: "rate low", suiteID: "cap", eventCounts: "10000", repetitions: 1, targetRate: -1, want: "target rate"},
		{name: "rate high", suiteID: "cap", eventCounts: "10000", repetitions: 1, targetRate: 100001, want: "target rate"},
		{name: "empty", suiteID: "cap", eventCounts: " ", repetitions: 1, want: "at least one"},
		{name: "parse", suiteID: "cap", eventCounts: "many", repetitions: 1, want: "comma-separated"},
		{name: "count low", suiteID: "cap", eventCounts: "599", repetitions: 1, want: "comma-separated"},
		{name: "count high", suiteID: "cap", eventCounts: "1000001", repetitions: 1, want: "comma-separated"},
		{name: "duplicate", suiteID: "cap", eventCounts: "10000,10000", repetitions: 1, want: "duplicated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPlan(tt.suiteID, tt.eventCounts, tt.repetitions, tt.targetRate)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewPlan() error = %v", err)
			}
		})
	}
}
