# Contribution projection contract

| Field | Decision |
| --- | --- |
| Status | Implemented |
| Schema version | `1` |
| API | `POST /api/v1/projections/contributions` |
| State | Stateless; assumptions and results are not persisted |
| Intended use | Educational, hypothetical long-term contribution scenarios |

## User problem

The projection answers a bounded planning question: given an initial amount,
an amount contributed monthly or biweekly, a time horizon, and explicit return,
inflation, and expense assumptions, what are the resulting conservative, base,
and optimistic values?

It does not predict a security's return, recommend an instrument, execute a
trade, or guarantee an outcome. The API returns this disclaimer with every
response.

## Request

Money and percentage values are decimal strings so JSON binary floating-point
parsing does not silently change the input contract.

```json
{
  "initial_investment": "10000.00",
  "contribution_amount": "500.00",
  "contribution_frequency": "monthly",
  "years": 20,
  "annual_return_pct": "7.00",
  "return_variance_pct": "2.00",
  "annual_inflation_pct": "2.50",
  "annual_expense_ratio_pct": "0.20"
}
```

Supported cadences are `monthly` (12 contributions/year) and `biweekly` (26
contributions/year). Contributions occur at the end of each period. The
dashboard shows the annualized contribution explicitly because equal amounts
at those two cadences do not represent equal annual budgets.

## Calculation

For each conservative, base, and optimistic scenario:

1. Gross annual return is `base return - variance`, `base return`, or `base
   return + variance`.
2. Net annual return is gross annual return minus the annual expense ratio.
3. An effective annual return is converted to the contribution cadence:
   `periodic rate = (1 + annual rate)^(1 / periods per year) - 1`.
4. Each period compounds the prior balance and then adds the contribution.
5. Inflation-adjusted ending value is
   `nominal ending value / (1 + inflation)^years`.
6. Estimated fee drag is the difference between an otherwise identical gross
   return path and the expense-adjusted return path.

The expense calculation is an educational approximation. Actual fund expenses
are reflected in NAV or market performance and may not behave like a direct
annual deduction. Returns are assumed to be total returns only when the caller's
assumption includes reinvested distributions.

## Response

Every response includes normalized assumptions, annualized contribution,
contribution timing, total contributions, nominal and inflation-adjusted ending
values, investment growth, and estimated fee drag for all three scenarios.
Money is rounded to cents only at the response boundary.

The source-controlled golden case uses a $1,000 initial amount, twelve $100
monthly end-of-period contributions, one year, and zero return, inflation, and
expenses. Every scenario must return:

```json
{
  "ending_balance": "2200.00",
  "total_contributed": "2200.00",
  "investment_growth": "0.00",
  "estimated_fee_drag": "0.00"
}
```

The same golden result is checked in domain tests, HTTP tests, and the
Kubernetes acceptance verifier.

## Validation and safety

- Unknown JSON fields, trailing JSON, malformed decimal strings, and requests
  larger than 8 KiB are rejected.
- Initial investment and contribution amounts must be non-negative and within
  documented learning-environment limits.
- The horizon is 1–100 years.
- Every modeled net annual return must remain above -100%.
- The API is stateless and does not accept brokerage credentials, account
  numbers, orders, or tax data.
- Responses use `Cache-Control: no-store`.

## Authoritative product references

- [Investor.gov compound interest calculator](https://www.investor.gov/financial-tools-calculators/calculators/compound-interest-calculator)
- [SEC mutual fund and ETF fee guidance](https://www.investor.gov/introduction-investing/general-resources/news-alerts/alerts-bulletins/mutual-fund-and-etf-fees-and-expenses-investor-bulletin)
