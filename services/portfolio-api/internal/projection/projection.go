package projection

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
)

const (
	Disclaimer  = "Hypothetical educational projection; not a forecast, guarantee, or investment recommendation."
	Methodology = "Effective annual returns are converted to the contribution cadence; contributions occur at period end; the expense ratio is subtracted from the gross annual return."
)

var (
	moneyPattern        = regexp.MustCompile(`^(0|[1-9][0-9]{0,12})(\.[0-9]{1,2})?$`)
	percentagePattern   = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,2})(\.[0-9]{1,4})?$`)
	errInvalidRequest   = errors.New("invalid contribution projection request")
	frequencyPerYear    = map[string]int{"monthly": 12, "biweekly": 26}
	scenarioDefinitions = []struct {
		name       string
		rateOffset float64
	}{
		{name: "conservative", rateOffset: -1},
		{name: "base", rateOffset: 0},
		{name: "optimistic", rateOffset: 1},
	}
)

type Request struct {
	InitialInvestment     string `json:"initial_investment"`
	ContributionAmount    string `json:"contribution_amount"`
	ContributionFrequency string `json:"contribution_frequency"`
	Years                 int    `json:"years"`
	AnnualReturnPct       string `json:"annual_return_pct"`
	ReturnVariancePct     string `json:"return_variance_pct"`
	AnnualInflationPct    string `json:"annual_inflation_pct"`
	AnnualExpenseRatioPct string `json:"annual_expense_ratio_pct"`
}

type Assumptions struct {
	InitialInvestment      string `json:"initial_investment"`
	ContributionAmount     string `json:"contribution_amount"`
	ContributionFrequency  string `json:"contribution_frequency"`
	ContributionsPerYear   int    `json:"contributions_per_year"`
	AnnualizedContribution string `json:"annualized_contribution"`
	Years                  int    `json:"years"`
	AnnualReturnPct        string `json:"annual_return_pct"`
	ReturnVariancePct      string `json:"return_variance_pct"`
	AnnualInflationPct     string `json:"annual_inflation_pct"`
	AnnualExpenseRatioPct  string `json:"annual_expense_ratio_pct"`
	ContributionTiming     string `json:"contribution_timing"`
}

type Scenario struct {
	Name                           string `json:"name"`
	GrossAnnualReturnPct           string `json:"gross_annual_return_pct"`
	NetAnnualReturnPct             string `json:"net_annual_return_pct"`
	EndingBalance                  string `json:"ending_balance"`
	InflationAdjustedEndingBalance string `json:"inflation_adjusted_ending_balance"`
	TotalContributed               string `json:"total_contributed"`
	InvestmentGrowth               string `json:"investment_growth"`
	EstimatedFeeDrag               string `json:"estimated_fee_drag"`
}

type Response struct {
	SchemaVersion int         `json:"schema_version"`
	Currency      string      `json:"currency"`
	Disclaimer    string      `json:"disclaimer"`
	Methodology   string      `json:"methodology"`
	Assumptions   Assumptions `json:"assumptions"`
	Scenarios     []Scenario  `json:"scenarios"`
}

func Decode(reader io.Reader) (Request, error) {
	var request Request
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("%w: decode: %v", errInvalidRequest, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Request{}, fmt.Errorf("%w: trailing JSON", errInvalidRequest)
	}
	return request, nil
}

func Calculate(request Request) (Response, error) {
	initial, err := parseMoney("initial_investment", request.InitialInvestment, 1_000_000_000_000)
	if err != nil {
		return Response{}, err
	}
	contribution, err := parseMoney("contribution_amount", request.ContributionAmount, 1_000_000_000)
	if err != nil {
		return Response{}, err
	}
	contributionsPerYear, exists := frequencyPerYear[request.ContributionFrequency]
	if !exists {
		return Response{}, fmt.Errorf("%w: contribution_frequency", errInvalidRequest)
	}
	if request.Years < 1 || request.Years > 100 {
		return Response{}, fmt.Errorf("%w: years", errInvalidRequest)
	}
	annualReturn, err := parsePercentage("annual_return_pct", request.AnnualReturnPct, -99, 100)
	if err != nil {
		return Response{}, err
	}
	variance, err := parsePercentage("return_variance_pct", request.ReturnVariancePct, 0, 100)
	if err != nil {
		return Response{}, err
	}
	inflation, err := parsePercentage("annual_inflation_pct", request.AnnualInflationPct, 0, 50)
	if err != nil {
		return Response{}, err
	}
	expenseRatio, err := parsePercentage("annual_expense_ratio_pct", request.AnnualExpenseRatioPct, 0, 20)
	if err != nil {
		return Response{}, err
	}
	if annualReturn-variance-expenseRatio <= -100 {
		return Response{}, fmt.Errorf("%w: conservative net return must exceed -100%%", errInvalidRequest)
	}

	scenarios := make([]Scenario, 0, len(scenarioDefinitions))
	for _, definition := range scenarioDefinitions {
		scenarios = append(scenarios, calculateScenario(
			definition.name,
			annualReturn+definition.rateOffset*variance,
			initial,
			contribution,
			inflation,
			expenseRatio,
			contributionsPerYear,
			request.Years,
		))
	}

	return Response{
		SchemaVersion: 1,
		Currency:      "USD",
		Disclaimer:    Disclaimer,
		Methodology:   Methodology,
		Assumptions: Assumptions{
			InitialInvestment:      formatMoney(initial),
			ContributionAmount:     formatMoney(contribution),
			ContributionFrequency:  request.ContributionFrequency,
			ContributionsPerYear:   contributionsPerYear,
			AnnualizedContribution: formatMoney(contribution * float64(contributionsPerYear)),
			Years:                  request.Years,
			AnnualReturnPct:        formatPercentage(annualReturn),
			ReturnVariancePct:      formatPercentage(variance),
			AnnualInflationPct:     formatPercentage(inflation),
			AnnualExpenseRatioPct:  formatPercentage(expenseRatio),
			ContributionTiming:     "end_of_period",
		},
		Scenarios: scenarios,
	}, nil
}

func parseMoney(field, value string, maximum float64) (float64, error) {
	if !moneyPattern.MatchString(value) {
		return 0, fmt.Errorf("%w: %s", errInvalidRequest, field)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed > maximum {
		return 0, fmt.Errorf("%w: %s", errInvalidRequest, field)
	}
	return parsed, nil
}

func parsePercentage(field, value string, minimum, maximum float64) (float64, error) {
	if !percentagePattern.MatchString(value) {
		return 0, fmt.Errorf("%w: %s", errInvalidRequest, field)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%w: %s", errInvalidRequest, field)
	}
	return parsed, nil
}

func calculateScenario(
	name string,
	grossAnnualReturn float64,
	initial float64,
	contribution float64,
	inflation float64,
	expenseRatio float64,
	contributionsPerYear int,
	years int,
) Scenario {
	netAnnualReturn := grossAnnualReturn - expenseRatio
	grossPeriodReturn := periodicRate(grossAnnualReturn, contributionsPerYear)
	netPeriodReturn := periodicRate(netAnnualReturn, contributionsPerYear)
	grossBalance := initial
	netBalance := initial
	periods := contributionsPerYear * years
	for period := 0; period < periods; period++ {
		grossBalance = grossBalance*(1+grossPeriodReturn) + contribution
		netBalance = netBalance*(1+netPeriodReturn) + contribution
	}
	totalContributed := initial + contribution*float64(periods)
	inflationAdjusted := netBalance / math.Pow(1+inflation/100, float64(years))

	return Scenario{
		Name:                           name,
		GrossAnnualReturnPct:           formatPercentage(grossAnnualReturn),
		NetAnnualReturnPct:             formatPercentage(netAnnualReturn),
		EndingBalance:                  formatMoney(netBalance),
		InflationAdjustedEndingBalance: formatMoney(inflationAdjusted),
		TotalContributed:               formatMoney(totalContributed),
		InvestmentGrowth:               formatMoney(netBalance - totalContributed),
		EstimatedFeeDrag:               formatMoney(grossBalance - netBalance),
	}
}

func periodicRate(annualPercentage float64, periodsPerYear int) float64 {
	return math.Pow(1+annualPercentage/100, 1/float64(periodsPerYear)) - 1
}

func formatMoney(value float64) string {
	rounded := math.Round(value*100) / 100
	if math.Abs(rounded) < 0.005 {
		rounded = 0
	}
	return strconv.FormatFloat(rounded, 'f', 2, 64)
}

func formatPercentage(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}
