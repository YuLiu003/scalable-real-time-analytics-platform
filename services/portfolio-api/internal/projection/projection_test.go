package projection

import (
	"strings"
	"testing"
)

func validRequest() Request {
	return Request{
		InitialInvestment:     "10000.00",
		ContributionAmount:    "500.00",
		ContributionFrequency: "monthly",
		Years:                 20,
		AnnualReturnPct:       "7.00",
		ReturnVariancePct:     "2.00",
		AnnualInflationPct:    "2.50",
		AnnualExpenseRatioPct: "0.20",
	}
}

func TestDecodeAcceptsStrictCanonicalRequest(t *testing.T) {
	request, err := Decode(strings.NewReader(`{
		"initial_investment":"1000.00",
		"contribution_amount":"100.00",
		"contribution_frequency":"monthly",
		"years":1,
		"annual_return_pct":"0",
		"return_variance_pct":"0",
		"annual_inflation_pct":"0",
		"annual_expense_ratio_pct":"0"
	}`))
	if err != nil || request.Years != 1 {
		t.Fatalf("Decode() = %+v, %v", request, err)
	}
}

func TestDecodeRejectsMalformedUnknownAndTrailingJSON(t *testing.T) {
	for _, body := range []string{
		`{"initial_investment":`,
		`{"unknown":"value"}`,
		`{} {}`,
	} {
		if _, err := Decode(strings.NewReader(body)); err == nil {
			t.Fatalf("Decode(%q) unexpectedly succeeded", body)
		}
	}
}

func TestCalculateZeroReturnMatchesContributions(t *testing.T) {
	request := validRequest()
	request.InitialInvestment = "1000"
	request.ContributionAmount = "100"
	request.Years = 1
	request.AnnualReturnPct = "0"
	request.ReturnVariancePct = "0"
	request.AnnualInflationPct = "0"
	request.AnnualExpenseRatioPct = "0"

	response, err := Calculate(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.SchemaVersion != 1 || response.Currency != "USD" ||
		response.Disclaimer != Disclaimer || response.Methodology != Methodology {
		t.Fatalf("response metadata = %+v", response)
	}
	if response.Assumptions.AnnualizedContribution != "1200.00" ||
		response.Assumptions.ContributionTiming != "end_of_period" ||
		response.Assumptions.ContributionsPerYear != 12 {
		t.Fatalf("assumptions = %+v", response.Assumptions)
	}
	if len(response.Scenarios) != 3 {
		t.Fatalf("scenarios = %d", len(response.Scenarios))
	}
	for _, scenario := range response.Scenarios {
		if scenario.EndingBalance != "2200.00" ||
			scenario.InflationAdjustedEndingBalance != "2200.00" ||
			scenario.TotalContributed != "2200.00" ||
			scenario.InvestmentGrowth != "0.00" ||
			scenario.EstimatedFeeDrag != "0.00" {
			t.Fatalf("%s scenario = %+v", scenario.Name, scenario)
		}
	}
}

func TestCalculateProducesBiweeklyScenarioRangeAndFeeDrag(t *testing.T) {
	request := validRequest()
	request.ContributionFrequency = "biweekly"
	response, err := Calculate(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Assumptions.ContributionsPerYear != 26 ||
		response.Assumptions.AnnualizedContribution != "13000.00" ||
		response.Assumptions.AnnualReturnPct != "7.0000" ||
		response.Assumptions.ReturnVariancePct != "2.0000" ||
		response.Assumptions.AnnualInflationPct != "2.5000" ||
		response.Assumptions.AnnualExpenseRatioPct != "0.2000" {
		t.Fatalf("assumptions = %+v", response.Assumptions)
	}
	if response.Scenarios[0].Name != "conservative" ||
		response.Scenarios[0].GrossAnnualReturnPct != "5.0000" ||
		response.Scenarios[1].Name != "base" ||
		response.Scenarios[1].NetAnnualReturnPct != "6.8000" ||
		response.Scenarios[2].Name != "optimistic" ||
		response.Scenarios[2].GrossAnnualReturnPct != "9.0000" {
		t.Fatalf("scenarios = %+v", response.Scenarios)
	}
	for _, scenario := range response.Scenarios {
		if scenario.EndingBalance == "0.00" || scenario.EstimatedFeeDrag == "0.00" {
			t.Fatalf("%s scenario did not compound or model fee drag: %+v", scenario.Name, scenario)
		}
	}
}

func TestCalculateMatchesDocumentedMonthlyGoldenResult(t *testing.T) {
	response, err := Calculate(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	base := response.Scenarios[1]
	if base.EndingBalance != "285354.71" ||
		base.InflationAdjustedEndingBalance != "174143.69" ||
		base.TotalContributed != "130000.00" ||
		base.InvestmentGrowth != "155354.71" ||
		base.EstimatedFeeDrag != "7110.32" {
		t.Fatalf("base scenario = %+v", base)
	}
}

func TestCalculateRejectsEveryInvalidInputClass(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "initial format", mutate: func(r *Request) { r.InitialInvestment = "-1" }},
		{name: "initial maximum", mutate: func(r *Request) { r.InitialInvestment = "1000000000001" }},
		{name: "contribution format", mutate: func(r *Request) { r.ContributionAmount = "1.001" }},
		{name: "contribution maximum", mutate: func(r *Request) { r.ContributionAmount = "1000000001" }},
		{name: "frequency", mutate: func(r *Request) { r.ContributionFrequency = "daily" }},
		{name: "years low", mutate: func(r *Request) { r.Years = 0 }},
		{name: "years high", mutate: func(r *Request) { r.Years = 101 }},
		{name: "return format", mutate: func(r *Request) { r.AnnualReturnPct = "7%" }},
		{name: "return range", mutate: func(r *Request) { r.AnnualReturnPct = "-100" }},
		{name: "variance", mutate: func(r *Request) { r.ReturnVariancePct = "-1" }},
		{name: "inflation", mutate: func(r *Request) { r.AnnualInflationPct = "51" }},
		{name: "expense", mutate: func(r *Request) { r.AnnualExpenseRatioPct = "21" }},
		{
			name: "conservative net return",
			mutate: func(r *Request) {
				r.AnnualReturnPct = "-90"
				r.ReturnVariancePct = "10"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validRequest()
			tt.mutate(&request)
			if _, err := Calculate(request); err == nil {
				t.Fatal("Calculate() unexpectedly succeeded")
			}
		})
	}
}

func TestFormattingNormalizesNegativeZero(t *testing.T) {
	if got := formatMoney(-0.001); got != "0.00" {
		t.Fatalf("formatMoney(-0.001) = %q", got)
	}
	if got := formatPercentage(-1.25); got != "-1.2500" {
		t.Fatalf("formatPercentage(-1.25) = %q", got)
	}
}
