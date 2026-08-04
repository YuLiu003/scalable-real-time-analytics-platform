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
		`/api/v1/config`,
		`id="access-token-form"`,
		`type="password"`,
		`Authorization: `,
		`encodeURIComponent(configuration.portfolio_id)`,
		`projectionForm.elements.initial_investment.value = Number(result.total_market_value).toFixed(2)`,
		`stock: 'Stock'`,
		`etf: 'ETF'`,
		`index: 'Index'`,
		`Private portfolio data`,
		`Synthetic demonstration data`,
		`Inflation-adjusted`,
		`Estimated fee drag`,
		`Not financial advice`,
	} {
		if !strings.Contains(page, required) {
			t.Fatalf("dashboard does not contain %q", required)
		}
	}
	for _, forbidden := range []string{
		`/api/v1/portfolios/demo/allocation`,
		`localStorage`,
		`sessionStorage`,
		`document.cookie`,
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("dashboard unexpectedly contains %q", forbidden)
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
