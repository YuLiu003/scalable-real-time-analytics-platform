# Slice 3 Analytics and Replay Contract

Slice 3 adds one deterministic analytical path without changing the Slice 2
delivery contract:

```text
Garage bronze JSON ──> portfolio-analytics Job (DuckDB)
                              │
                              ├──> silver market-price Parquet
                              └──> gold allocation Parquet + canonical JSON
                                                        │
                                                        v
                                                Go API + dashboard
```

The analytics Job reads immutable bronze objects for its configured source and
a source-controlled synthetic holdings snapshot. It does not read Kafka, call
the producer, or fetch market data from the internet. The selected bronze
source is the replay boundary and keeps the analytical engine replaceable.

## First calculation

The first result is portfolio allocation, not performance return. The later
offline v1 ledger records external deposits and withdrawals, but this analytics
Job does not consume it and there are still no trade, dividend, fee, or cost-
basis events. Claiming time-weighted return, money-weighted return, or gain/loss
would therefore invent financial history.

The versioned `demo` fixture is explicitly synthetic and holds:

| Instrument | Type | Valuation | Quantity |
| --- | --- | --- | ---: |
| DEMO-ASSET-A | ETF | Market price | 1 |
| DEMO-ASSET-B | ETF | Market price | 2 |
| DEMO-ASSET-C | Mutual fund | Daily NAV | 3 |

`DEMO-BENCH-D` represents a non-investable synthetic benchmark level. It has no
quantity, contributes nothing to portfolio market value, and is not presented
as an investable holding.

All identities, classifications, prices, and quantities are fictional. The two
ETF positions are independent examples; they do not encode a real issuer,
index, account, or investment recommendation.

For each instrument, DuckDB selects the latest valid price by `occurred_at`,
then `provider_sequence`, then `event_id`. All quantities, prices, market values,
and percentages use fixed-point `DECIMAL` arithmetic.

```text
market_value = quantity * latest_price
allocation_pct = round(market_value / portfolio_total * 100, 4)
```

With the synthetic Slice 2 fixtures, the expected total is exactly
`600.00000000` USD across three positions. These values are deterministic test
inputs, not current quotes or investment advice.

## Data products

The Job computes a SHA-256 input-set identity over the versioned holdings bytes
and every sorted bronze object key plus its content hash selected for the
configured `BRONZE_SOURCE`. The identity becomes the immutable run partition:

```text
silver/market_prices/v1/run=<input_set_sha256>/part-00000.parquet
gold/portfolio_allocations/v2/portfolio=demo/run=<input_set_sha256>/allocation.parquet
gold/portfolio_allocations/v2/portfolio=demo/latest.json
```

Silver contains normalized, deduplicated observations. Gold contains one row
per holding and a canonical JSON representation for the API, including the
separate benchmark. The run-scoped
Parquet objects are immutable for a given input identity. `latest.json` is a
replaceable materialized pointer/result and is therefore not an audit record.

The canonical JSON contains no wall-clock build time. Its `as_of` value is the
maximum selected source event time, so identical inputs produce identical JSON
bytes and the same result SHA-256.

## Failure and consistency boundaries

| Failure | Outcome |
| --- | --- |
| Missing selected-source bronze objects | Job fails; existing gold result remains readable |
| Invalid selected-source bronze JSON or contract mismatch | Job fails without publishing a new result |
| Selected-source event has another `tenant_id` | Job fails closed; portfolios cannot consume another tenant's input |
| Holding lacks a price | Job fails and names the missing instrument |
| Currency mismatch | Job fails; this slice performs no FX conversion |
| S3 unavailable during download | Job fails before computation |
| S3 unavailable during publication | Partial run-scoped objects may exist; replay safely rewrites the same deterministic run |
| Job reruns with identical input | Same input identity and canonical result; no duplicate analytical rows |
| API cannot read `latest.json` | Readiness and result endpoint fail; liveness remains healthy |

Publication is not an atomic multi-object transaction. The API reads only
`latest.json`, which the Job writes last after both Parquet objects succeed. A
failed build therefore cannot point the API at an incomplete new run.

## Replay proof

The acceptance test performs the following sequence:

1. Build silver/gold from the four Slice 2 objects selected by `BRONZE_SOURCE`.
2. Assert one silver Parquet object, one immutable gold Parquet object, and one
   gold `latest.json` object.
3. Assert the API returns three positions, the DEMO-BENCH-D benchmark, and total value
   `600.00000000`.
4. Record the canonical result SHA-256.
5. Delete only the `silver/market_prices/` and
   `gold/portfolio_allocations/` prefixes.
6. Confirm the API result becomes unavailable while liveness remains healthy.
7. Rerun the analytics Job without rerunning any producer or reading Kafka.
8. Assert the object counts, total, input identity, and result SHA-256 exactly
   match the first build.

This demonstrates deterministic reconstruction from retained, selected-source
bronze inputs. It does not yet prove historical corrections, table
transactions, concurrent writers, object-version recovery, or financial
performance calculations.
