# Personal portfolio cash-flow ledger contract

## Scope

V2-2 adds an offline importer for manual JSON or CSV records of external cash
flows. It normalizes deposits and withdrawals without treating either as
investment return. It does not yet model trades, lots, dividends, fees, cost
basis, or brokerage accounts.

The strict input row has exactly five fields:

```text
transaction_id,occurred_at,cash_flow_type,amount,currency
```

- `transaction_id` is stable and unique within the import.
- `occurred_at` is RFC3339 with a timezone and at most six fractional digits;
  it is normalized to UTC without silent precision loss.
- `cash_flow_type` is exactly `deposit` or `withdrawal`.
- `amount` is a positive fixed-point decimal with at most eight places.
- Every row must match the requested portfolio base currency.

The canonical v1 output sorts by time and transaction ID and includes separate
`total_deposits`, `total_withdrawals`, and signed
`net_external_cash_flow = deposits - withdrawals`. Equivalent JSON and CSV
inputs produce byte-identical output. Re-importing cannot double-apply a row
because normalization is a pure rebuild, not a mutable append.

## Private local use

Keep real exports outside the repository. The importer writes atomically with
mode `0600` and logs only an aggregate record count:

```bash
PYTHONPATH="$PWD/services/portfolio-analytics" \
  .venv/bin/python -m portfolio_analytics.import_ledger \
  --input-format csv \
  --portfolio private \
  --currency USD \
  --input /path/outside/repository/cash-flows.csv \
  --output /path/outside/repository/cash-flow-ledger.v1.json
```

Run that command from the repository root.

The committed JSON and CSV fixtures are fictional and exist only to prove
equivalence, invalid-input rejection, deterministic ordering, and exact
fixed-point totals.

## Storage boundary

The ledger stays offline until authenticated access, retention, and deletion
contracts exist. PostgreSQL, Valkey, Kafka, and object storage do not improve
this single-writer normalization step. The allocation API remains byte-
compatible and does not expose the ledger. A later performance slice may join
prices and dated external flows only after defining time-weighted and money-
weighted return semantics.
