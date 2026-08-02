# Slice 3 Verification Record

| Field | Value |
| --- | --- |
| Date | Historical run 2026-07-21; current revalidation 2026-07-30 |
| Historical branch | `feature/portfolio-analytics-slice3` |
| Current revalidation | `feature/kafka-scale-lab` working tree based on `5ae7dd3` |
| Status | Current disposable runtime revalidation passed |
| Final state | kind and Colima deleted after successful verification |

> Evidence integrity notice: this record was captured before the public
> fixtures were replaced with neutral identities and values. Identifiers in
> this copy are fictional replacements, and the prior digests are intentionally
> absent. Treat the detailed narrative as historical design context. The
> current revalidation below uses only fictional public fixtures.

## Current revalidation

The 2026-07-30 disposable run built and replayed the configured `synthetic`
source both before and after 1,200 scale objects were archived. Both builds read
exactly 4 portfolio inputs and reproduced:

```text
input_set_sha256=907e5275a884a446285cdff7023638f3bbc0a66660ee9b0a8bf53c3777c8405f
result_sha256=a3756ba641c48a7518bbf5cfd8338d85338d882bb62648a40a8c0547a0b3bae2
```

This is local deterministic evidence, not historical market-performance or
cloud-availability evidence.

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
total=600.00000000
positions=3
input_objects=4
silver_objects=1
gold_objects=2
input_set_sha256=<historical digest omitted>
result_sha256=<historical digest omitted>
```

The API returned three fund positions, the separate DEMO-BENCH-D benchmark, and the
embedded dashboard HTML. The dashboard response includes an explicit synthetic,
non-live-data disclosure. Screenshot-level browser QA was not available in the
verification session because no in-app or Chrome browser was connected.
The process-only `/healthz` endpoint returned HTTP 200. The `/readyz` endpoint
depended on fetching and validating the canonical gold result.

An independent DuckDB query opened the live silver and gold Parquet files and
reported:

```json
{"event":"DuckDB Parquet query passed","gold_rows":3,"silver_rows":4,"total_market_value":"600.00000000"}
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
gold objects under the historical fixture.

The prior fixture digests remain omitted; the current neutral-fixture digests
are recorded above.

## Scoped lifecycle evidence

The Slice 3-only lifecycle was tested from the previously running result:

```bash
CONFIRM_DESTROY_ANALYTICS=portfolio-analytics \
  make -C platform/local destroy-analytics
make -C platform/local bootstrap-analytics
```

Teardown removed the API, analytics Jobs, service accounts, Service, generated
holdings ConfigMap, and derived silver/gold products. It preserved the Slice 2
Kafka, Garage, archiver, topics, identities, and four bronze objects. The
historical clean rebuild finished in `34.59s` and reproduced digests that are
not retained in this anonymized copy.

An interruption was then simulated after the Parquet query and derived reset
Jobs completed but while the replay Job remained suspended. At that checkpoint
the API Deployment was intentionally `0/1` Ready, the process remained live,
and bronze remained present. Re-running `verify-analytics` detected the Job
state, skipped the impossible pre-replay readiness wait, resumed the replay, and
returned the API to `1/1` Ready with the historical result identity.

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
${TMPDIR}/portfolio-analytics-diagnostics-20260721T233939Z
```

It contains workload state, events, component logs, and result metadata. The
diagnostic workflow intentionally excludes Secret objects and values, bronze
payloads, portfolio result contents, kubeconfigs, and credentials.

## Conclusion

The current revalidation and historical Slice 3 run cover the user-visible
analytical path from
retained immutable events through fixed-point DuckDB computation and Parquet
products to a stateless Go API/dashboard. Transaction and cash-flow contracts,
performance returns, historical corrections, concurrent publication,
table-format transactions, observability, GitOps rollback, node failure, and
cloud-managed equivalents remain later work.
