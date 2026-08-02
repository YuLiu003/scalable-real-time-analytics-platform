# Slice 2 Event and Delivery Contract

Slice 2 proves one deliberately thin distributed path:

```text
synthetic producer -> Strimzi Kafka -> raw-event archiver -> Garage S3 API
                                      |
                                      +-> quarantine topic for invalid input
```

It is an integration and failure-semantics exercise, not a market-data product.
The producer uses synthetic prices, and the single Kafka broker and single
Garage instance are resource-bounded local learning components rather than a
high-availability topology.

## Canonical record

`contracts/events/market.price.observed.v1.schema.json` is the source-controlled
contract for `market.price.observed` version 1. The Go implementation performs
strict validation against that schema plus cross-field time and instrument
constraints before archiving a record.

- `event_id` is stable across retries and identifies the durable archive effect.
- `partition_key` is the canonical instrument and is also the Kafka message key.
- `price` is a decimal string; it is never parsed as a binary floating-point
  number at the event boundary.
- `occurred_at` describes source time and controls the archive date partition.
- `ingested_at` cannot precede `occurred_at`.
- Unknown fields and trailing JSON values are invalid in version 1.
- `trace_id` is a non-zero, 16-byte lowercase hexadecimal correlation ID. Full
  OpenTelemetry propagation is deferred to Slice 4.

Version 1 deliberately carries one generic observed decimal named `price`; it
does not distinguish an ETF trade/close, mutual-fund NAV, or index level. Slice
3 supplies those semantics through its versioned instrument/portfolio fixture.
A real source adapter must emit each value at its correct cadence, and the event
schema should evolve before downstream consumers need observation-kind
semantics at the event boundary.

The archive layout is deterministic:

```text
bronze/market.price.observed/v1/
  date=<UTC event date>/source=<source>/instrument=<instrument>/<event_id>.json
```

## Delivery boundary

The declared contract is **at-least-once delivery with an idempotent archive
effect**.

1. The producer requests all in-sync replica acknowledgements and enables the
   Kafka client's idempotent retry behavior.
   Both clients authenticate with a Strimzi KafkaUser certificate and validate
   the broker with the separate, public cluster CA copied into their namespace
   as a ConfigMap.
2. A producer success log means the broker acknowledged that send. It does not
   prove that a caller persisted the acknowledgement before crashing.
3. The archiver validates the record, checks the deterministic object key, and
   writes the raw JSON with a SHA-256 metadata value.
4. The consumer marks the Kafka message only after the archive write or the
   quarantine write succeeds. Kafka commits consumer-group progress separately,
   so a crash can still cause redelivery.
5. The same key and content is a successful duplicate. The same `event_id` and
   key with different content is a collision and is quarantined; it is never
   allowed to replace the original object.

This slice does not claim end-to-end exactly-once processing. The archive's
idempotency boundary is one object key. Later portfolio aggregates must provide
their own idempotency strategy.

## Failure outcomes

| Failure | Durable outcome | Offset outcome |
| --- | --- | --- |
| Invalid canonical record | One record on `ingestion.quarantine` | Marked only after quarantine acknowledgement |
| Exact duplicate | Existing object retained; duplicate reported | Marked after content equality is proven |
| Event ID/content collision | Existing object retained; collision quarantined | Marked only after quarantine acknowledgement |
| Object store unavailable | No successful effect | Not marked; record is retried after recovery |
| Producer crashes after broker acknowledgement | Caller may retry the stable `event_id`; Kafka may contain both sends | Not applicable to producer |
| Consumer crashes after object write, before marking | Object exists and the Kafka record is redelivered | New consumer recognizes the duplicate, then marks it |
| Quarantine topic unavailable | Invalid record is not silently skipped | Not marked |

## Runtime acceptance test

The scripted test publishes seven Kafka records:

- Four baseline records: two unique valid events, an exact duplicate, and one
  malformed event.
- Two records with the same stable ID around an injected producer crash after
  acknowledgement.
- One valid record while the consumer is killed after its object write and
  before offset marking.

The exact terminal state is:

- `market.prices`: 7 retained records across 3 partitions.
- `ingestion.quarantine`: 1 malformed-record outcome.
- `bronze/`: 4 immutable objects (DEMO-ASSET-A, DEMO-ASSET-B, DEMO-ASSET-C, and DEMO-BENCH-D).
- The replacement archiver logs the expected one-way event reference as a
  duplicate after redelivery; it does not log the event ID or object key.

Run the complete test with:

```bash
make -C platform/local bootstrap-data-path
```

Re-check an already tested environment with:

```bash
make -C platform/local verify-data-path
```

The verification record belongs in
`platform/local/SLICE2-VERIFICATION.md`. Destroying this slice deletes its Kafka
and Garage persistent volumes and therefore requires a separate typed
confirmation.
