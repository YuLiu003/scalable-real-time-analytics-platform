# Portfolio Analytics Builder

This Python workload uses DuckDB to turn immutable bronze market-price JSON
objects into normalized silver Parquet and a deterministic gold portfolio
allocation. Its replay and consistency boundary is documented in the
[`Slice 3 analytics contract`](../../docs/features/cloud-native-investment-platform/slice-3-analytics-contract.md).

The versioned public v2 fixture models DEMO-ASSET-A and DEMO-ASSET-B as ETF
market-price holdings, DEMO-ASSET-C as a mutual-fund NAV holding, and
DEMO-BENCH-D as a non-position benchmark index level. The private Kubernetes
acceptance uses separate fictional stock/ETF symbols. All committed quantities
and values are synthetic test data.

The Job deliberately downloads bronze objects through the S3 API and runs
DuckDB against local ephemeral storage. It therefore needs no DuckDB network
extension at runtime, and the same transformation can later use S3, Cloud
Storage interoperability, or another object-storage adapter.

The separate offline ledger importer normalizes manual JSON or CSV deposits
and withdrawals without publishing private data to the platform:

```bash
PYTHONPATH="$PWD/services/portfolio-analytics" \
  .venv/bin/python -m portfolio_analytics.import_ledger --help
```

Its behavior is defined by the
[`personal cash-flow ledger contract`](../../docs/features/cloud-native-investment-platform/personal-portfolio-ledger-contract.md).

The private stock/ETF workflow validates three owner-only files outside Git,
then mounts the holdings through a Kubernetes Secret. Private mode accepts
stock/ETF market-price positions and a stock/ETF market-price benchmark, reads
only `alpaca-iex` bronze records from the two latest source-bearing UTC date
partitions, reduces them to the latest required observation per instrument, and
logs aggregate counts and publication outcomes without instruments, holdings,
values, or hashes:

```bash
PYTHONPATH="$PWD/services/portfolio-analytics" \
  .venv/bin/python -m portfolio_analytics.private_inputs --help
```

See the
[`private portfolio workflow`](../../docs/features/cloud-native-investment-platform/private-portfolio-workflow.md)
for the supported file contracts and end-to-end command.

Run its 100% statement-and-branch coverage gate from the repository root after
installing `requirements-dev.txt` in an isolated environment:

```bash
PYTHON_BIN=.venv/bin/python make -C platform/local quality
```

Build and run the tests through the pinned production container image as part
of the kind workflow:

```bash
make -C platform/local build-analytics
```
