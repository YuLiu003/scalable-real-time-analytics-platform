# Portfolio Analytics Builder

This Python workload uses DuckDB to turn immutable bronze market-price JSON
objects into normalized silver Parquet and a deterministic gold portfolio
allocation. Its replay and consistency boundary is documented in the
[`Slice 3 analytics contract`](../../docs/features/cloud-native-investment-platform/slice-3-analytics-contract.md).

The Job deliberately downloads bronze objects through the S3 API and runs
DuckDB against local ephemeral storage. It therefore needs no DuckDB network
extension at runtime, and the same transformation can later use S3, Cloud
Storage interoperability, or another object-storage adapter.

Run its tests through the pinned container image:

```bash
make -C platform/local build-analytics
```
