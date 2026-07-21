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

The analytics Job reads immutable bronze objects and a source-controlled
synthetic holdings snapshot. It does not read Kafka, call the producer, or fetch
market data from the internet. This makes bronze storage the replay boundary and
keeps the analytical engine replaceable.

## First calculation

The first result is portfolio allocation, not performance return. Slice 2 has
market prices but no canonical transaction, cash-flow, or cost-basis events, so
claiming time-weighted return, money-weighted return, or gain/loss would invent
financial history.

The versioned `demo` fixture holds:

| Instrument | Quantity |
| --- | ---: |
| AAPL | 10 |
| VTI | 5 |
| MSFT | 2 |
| GOOG | 3 |

For each instrument, DuckDB selects the latest valid price by `occurred_at`,
then `provider_sequence`, then `event_id`. All quantities, prices, market values,
and percentages use fixed-point `DECIMAL` arithmetic.

```text
market_value = quantity * latest_price
allocation_pct = round(market_value / portfolio_total * 100, 4)
```

With the Slice 2 fixtures, the expected total is exactly `5341.67000000` USD
across four positions.

## Data products

The Job computes a SHA-256 input-set identity over the versioned holdings bytes
and every sorted bronze object key plus its content hash. The identity becomes
the immutable run partition:

```text
silver/market_prices/v1/run=<input_set_sha256>/part-00000.parquet
gold/portfolio_allocations/v1/portfolio=demo/run=<input_set_sha256>/allocation.parquet
gold/portfolio_allocations/v1/portfolio=demo/latest.json
```

Silver contains normalized, deduplicated price observations. Gold contains one
row per holding and a canonical JSON representation for the API. The run-scoped
Parquet objects are immutable for a given input identity. `latest.json` is a
replaceable materialized pointer/result and is therefore not an audit record.

The canonical JSON contains no wall-clock build time. Its `as_of` value is the
maximum selected source event time, so identical inputs produce identical JSON
bytes and the same result SHA-256.

## Failure and consistency boundaries

| Failure | Outcome |
| --- | --- |
| Missing bronze objects | Job fails; existing gold result remains readable |
| Invalid bronze JSON or contract mismatch | Job fails without publishing a new result |
| Bronze event has another `tenant_id` | Job fails closed; portfolios cannot consume another tenant's input |
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

1. Build silver/gold from the four Slice 2 bronze objects.
2. Assert one silver Parquet object, one immutable gold Parquet object, and one
   gold `latest.json` object.
3. Assert the API returns four positions and total value `5341.67000000`.
4. Record the canonical result SHA-256.
5. Delete only the `silver/market_prices/` and
   `gold/portfolio_allocations/` prefixes.
6. Confirm the API result becomes unavailable while liveness remains healthy.
7. Rerun the analytics Job without rerunning any producer or reading Kafka.
8. Assert the object counts, total, input identity, and result SHA-256 exactly
   match the first build.

This demonstrates deterministic reconstruction from retained bronze inputs. It
does not yet prove historical corrections, table transactions, concurrent
writers, object-version recovery, or financial performance calculations.
