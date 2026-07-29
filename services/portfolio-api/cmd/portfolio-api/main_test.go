package main

import (
	"strings"
	"testing"
)

func TestEmbeddedDashboardContainsProjectionWorkflow(t *testing.T) {
	page := string(dashboard)
	for _, required := range []string{
		`id="projection-form"`,
		`/api/v1/projections/contributions`,
		`Inflation-adjusted`,
		`Estimated fee drag`,
		`Not financial advice`,
	} {
		if !strings.Contains(page, required) {
			t.Fatalf("dashboard does not contain %q", required)
		}
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("PORTFOLIO_TEST_SETTING", "")
	if got := envOrDefault("PORTFOLIO_TEST_SETTING", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault(empty) = %q", got)
	}
	t.Setenv("PORTFOLIO_TEST_SETTING", "configured")
	if got := envOrDefault("PORTFOLIO_TEST_SETTING", "fallback"); got != "configured" {
		t.Fatalf("envOrDefault(configured) = %q", got)
	}
}
