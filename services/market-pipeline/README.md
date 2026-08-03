# Market Event Pipeline

This Go module implements Slice 2's synthetic producer, strict canonical-event
validation, Kafka-to-S3 raw archiver, and runtime inspectors. Its behavioral
contract is documented in
[`slice-2-event-contract.md`](../../docs/features/cloud-native-investment-platform/slice-2-event-contract.md).

## Commands

| Command | Responsibility |
| --- | --- |
| `synthetic-producer` | Publish deterministic success, duplicate, malformed, and failure-test fixtures |
| `raw-event-archiver` | Validate Kafka records and persist idempotent raw objects or quarantine outcomes |
| `topic-inspector` | Assert run ordering/count or report exact per-partition consumer lag |
| `archive-inspector` | Assert the object keys present under an S3 prefix |
| `capacity-report` | Validate trial artifacts and aggregate every planned repetition |

The binaries share mutual-TLS Kafka configuration and run in one minimal,
non-root container image. The archiver uses an S3-compatible API so the local
Garage endpoint can later be replaced by AWS S3 through configuration, while
cloud identity and authorization remain an explicit Slice 6 concern.

## Local tests

The module intentionally has its own Go version boundary while the repository's
legacy services remain unchanged.

```bash
cd services/market-pipeline
GOWORK=off go test ./...
```

The portfolio feature quality gate additionally requires 100% statement
coverage for the canonical event, synthetic-fixture, scale, metrics, and
capacity-report domain packages, while the complete Kafka/S3 process boundary
is exercised in kind:

```bash
# From the repository root:
PYTHON_BIN="$PWD/.venv/bin/python" make -C platform/local quality
```

Use `make -C platform/local build-data-path` to test, cross-compile, build the
container, and load it into the existing kind cluster.
