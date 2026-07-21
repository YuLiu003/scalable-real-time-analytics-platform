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
| `topic-inspector` | Assert the retained Kafka record count from partition offsets |
| `archive-inspector` | Assert the object keys present under an S3 prefix |

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

Use `make -C platform/local build-data-path` to test, cross-compile, build the
container, and load it into the existing kind cluster.
