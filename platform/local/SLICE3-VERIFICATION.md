# Slice 3 Verification Record

| Field | Value |
| --- | --- |
| Date | 2026-07-21 |
| Branch | `feature/portfolio-analytics-slice3` |
| Status | Deterministic analytics, API, and exact replay passed; Slice 3 complete |
| Final state | Slice 3 running on context `kind-investment-platform` |

## Tested boundary

The test covers four retained Slice 2 bronze objects in Garage `v2.3.0`, a
one-shot Python `3.13.14`/DuckDB `1.5.4` analytics Job, one silver Parquet
product, one immutable gold Parquet product, a canonical gold JSON result, and a
stateless Go API/dashboard on Kubernetes `v1.33.7`.

The declared calculation and failure behavior are documented in the
[`Slice 3 analytics contract`](../../docs/features/cloud-native-investment-platform/slice-3-analytics-contract.md).
This is evidence for deterministic transformation, object publication, API
health semantics, and reconstruction from retained raw inputs. It is not
evidence for historical investment performance, concurrent lakehouse writers,
object-store availability, or cloud-zone resilience.

## Static and image verification

The following gates passed:

```bash
bash -n platform/local/scripts/{build,bootstrap,verify,diagnose,destroy}-analytics.sh
kubectl kustomize platform/gitops/apps/local/portfolio-analytics
kubectl --context kind-investment-platform apply --dry-run=server \
  -k platform/gitops/apps/local/portfolio-analytics
GOWORK=off GOCACHE=/tmp/portfolio-api-gocache go test -race ./...
GOWORK=off GOCACHE=/tmp/portfolio-api-gocache go vet ./...
git diff --check
```

The pinned analytics image ran five unit tests covering byte-for-byte
deterministic Parquet and JSON replay, order-independent input identity,
cross-tenant fail-closed behavior, duplicate holdings, and missing-price
publication failure. The Go API module used Go `1.25.12`; its HTTP and strict
data-product-key contract tests also passed under the race detector. Both
tested images were loaded into all three kind nodes before deployment.

## Runtime acceptance evidence

`make -C platform/local bootstrap-analytics` first reasserted the Slice 2
terminal state of seven market-price records, one quarantine record, and four
bronze objects. It then produced and inspected this exact result:

```text
portfolio=demo
total=5341.67000000
positions=4
input_objects=4
silver_objects=1
gold_objects=2
input_set_sha256=2f08a5b366aa9512dd6a32f9ede5df8deebce4cd86049911db1963023c107513
result_sha256=bb104c89a8865607b26a97e6aaa42a99f8f0249d8cc41231900eb32b6aa107e3
```

The API returned the expected allocation JSON and the embedded dashboard HTML.
The process-only `/healthz` endpoint returned HTTP 200. The `/readyz` endpoint
depended on fetching and validating the canonical gold result.

An independent DuckDB query opened the live silver and gold Parquet files and
reported:

```json
{"event":"DuckDB Parquet query passed","gold_rows":4,"silver_rows":4,"total_market_value":"5341.67000000"}
```

## Replay and failure-boundary evidence

The verifier recorded the baseline result hash, then ran a dedicated reset
binary that deleted only these prefixes:

```text
silver/market_prices/
gold/portfolio_allocations/
```

The portfolio result and readiness endpoint became unavailable while the
process liveness endpoint remained healthy. Kubernetes recorded the expected
readiness-probe HTTP 503 during that interval; the API process did not restart.

The replay Job then read the same four bronze objects and the versioned holdings
fixture without invoking a producer or Kafka. It recreated one silver and two
gold objects, the exact `5341.67000000` total, the same input-set identity, and
the same canonical result SHA-256:

```text
bb104c89a8865607b26a97e6aaa42a99f8f0249d8cc41231900eb32b6aa107e3
```

All four Slice 3 Jobs were Complete and the portfolio API Deployment settled at
`1/1` Ready with zero restarts.

## Scoped lifecycle evidence

The Slice 3-only lifecycle was tested from the previously running result:

```bash
CONFIRM_DESTROY_ANALYTICS=portfolio-analytics \
  make -C platform/local destroy-analytics
make -C platform/local bootstrap-analytics
```

Teardown removed the API, analytics Jobs, service accounts, Service, generated
holdings ConfigMap, and derived silver/gold products. It preserved the Slice 2
Kafka, Garage, archiver, topics, identities, and four bronze objects. The clean
rebuild and complete acceptance workflow finished in `41.33s` and reproduced
the hashes above.

An interruption was then simulated after the Parquet query and derived reset
Jobs completed but while the replay Job remained suspended. At that checkpoint
the API Deployment was intentionally `0/1` Ready, the process remained live,
and bronze remained present. Re-running `verify-analytics` detected the Job
state, skipped the impossible pre-replay readiness wait, resumed the replay, and
returned the API to `1/1` Ready with the exact result hash above.

## Failures found and durable corrections

1. DuckDB's Python client requires `pytz` when fetching `TIMESTAMPTZ` values.
   The container test caught the missing module before any Slice 3 Kubernetes
   mutation; the runtime dependency and its transitive versions are now pinned.
2. The liveness assertion expected a trailing newline after Bash command
   substitution, which always strips trailing newlines. The assertion now
   compares the actual value `ok`; direct endpoint evidence confirmed the API
   was healthy before the correction.
3. Result-dependent readiness originally allowed twenty consecutive failures.
   The threshold is now three probes, so unavailable derived state is removed
   from Service endpoints in approximately nine seconds at the configured
   period.
4. The verifier initially assumed readiness before handling a post-reset
   interruption. It now derives the expected hash from the completed baseline
   Job and resumes safely across query, reset, and replay Job boundaries before
   requiring result-dependent readiness.

## Evidence handling

The final diagnostic bundle was written to:

```text
${TMPDIR}/portfolio-analytics-diagnostics-20260721T231059Z
```

It contains workload state, events, component logs, and result metadata. The
diagnostic workflow intentionally excludes Secret objects and values, bronze
payloads, portfolio result contents, kubeconfigs, and credentials.

## Conclusion

Slice 3 proves a complete user-visible analytical path from retained immutable
events through fixed-point DuckDB computation and Parquet products to a
stateless Go API/dashboard. Exact replay demonstrates reconstruction rather than
database dependence. Transaction and cash-flow contracts, performance returns,
historical corrections, concurrent publication, table-format transactions,
observability, GitOps rollback, node failure, and cloud-managed equivalents
remain later work.
