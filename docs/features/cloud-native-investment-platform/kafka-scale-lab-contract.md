# Kafka Scale Lab Contract

## Purpose

This slice tests whether the event path remains correct when production rate
exceeds consumer capacity. Personal portfolio traffic is deliberately not used
to justify Kafka or Kubernetes. A configurable generator creates non-personal
load against the same canonical event, archive, and replay contracts.

This default run is a correctness, autoscaling, and recovery acceptance—not a
capacity benchmark. The separate
[capacity benchmark contract](kafka-capacity-benchmark-contract.md) owns
zero-delay repeated trials and durable throughput/resource comparisons.

```text
synthetic load Job -> market.prices.scale (3 partitions)
                         |
                         v
                scale archiver consumer group
                         |
              KEDA reads committed lag
                         |
            Deployment scales from 1 to 3
                         |
                         v
                 Garage / S3 archive
```

The scale topic, identities, consumer group, Deployment, and archive source are
isolated from the deterministic portfolio acceptance path. Portfolio analytics
selects only its configured source segment before reading object bodies. The
full acceptance run deletes and rebuilds derived analytics after the scale
phase, proving that scale objects cannot change the portfolio result.

## Privacy boundary

- Only fictional instrument identifiers are committed.
- A personal watchlist, provider token, brokerage export, or account identifier
  must be runtime input and must not enter Git, CI variables, fixture files,
  metric labels, reports, or diagnostics.
- Metrics aggregate by the fixed `baseline` or `scale` scope. They never label
  instruments, events, tenants, accounts, or portfolios.
- Archiver logs replace event IDs and object keys with a one-way event
  reference. Diagnostics record aggregate archive counts, not archive keys.
- Kafka records and archive objects necessarily contain the instrument from the
  event contract. Real-feed data therefore belongs only in a private runtime
  with an explicit retention and deletion policy.
- The scale Job must use non-personal identifiers. A future live-feed adapter
  will use a private Secret-backed watchlist and the normal market topic; it
  will not be the capacity generator.

A normal commit removes private identifiers from the current tree but cannot
remove them from existing Git history, forks, caches, CI artifacts, or runtime
storage. History cleanup is a separate coordinated operation.

## Producer contract

`PRODUCER_SCENARIO=load` generates records incrementally rather than allocating
the full run in memory.

| Variable | Default | Boundary |
| --- | --- | --- |
| `LOAD_RUN_ID` | `local-scale` | Lowercase public run slug, at most 26 characters |
| `LOAD_PHASE` | `original` | `original` or byte-identical `replay` |
| `LOAD_EVENT_COUNT` | `1000` | 1 through 1,000,000 |
| `LOAD_INSTRUMENTS` | Three fictional keys | 1 through 100 canonical identifiers |
| `LOAD_BASE_TIME` | Fixed UTC fixture time | RFC 3339 |
| `LOAD_TARGET_RATE` | `0` | Unbounded, or 1 through 100,000 events/second |

The partition key equals the canonical instrument. A fixed topic partition
count and Sarama's hash partitioner preserve order for each instrument. The
same run ID, instruments, event count, and base time reproduce byte-identical
event values. The Kafka `phase` header separates the original and replay
records without changing the archive identity.

The producer emits one aggregate completion record containing acknowledged
count, elapsed time, broker-acknowledgement throughput, and acknowledgement p95.
It does not log the instrument set.

## Consumer and scaling contract

- The scale consumer uses the dedicated `scale-event-archiver-v1` group.
- Offsets are marked only after the S3-compatible effect is durable.
- Archive keys and payload hashes make redelivery and replay idempotent.
- KEDA is pinned and installed only for the free local environment.
- One consumer is always running. Lag can scale the Deployment to at most three
  replicas because the topic has three partitions.
- The default injected work retains lag for at least 35 seconds per partition,
  spanning the HPA observation window. Configuration that cannot meet that
  bound fails before changing the cluster.
- The verifier refuses to start while an earlier scale producer Job is still
  unfinished, and it accepts zero lag only from a present, valid HPA metric.
- After KEDA naturally reaches three ready replicas, the verifier pins three
  consumers before deleting a pod. Lag drain therefore cannot race the
  replacement consumer-group proof.
- KEDA uses the existing mTLS KafkaUser identity and the broker cluster CA. Its
  namespace is explicitly allowed by the Strimzi listener.
- The pinned KEDA chart render must enable operator Prometheus metrics and
  create its ServiceMonitor.
- Scale experiments use a fresh disposable cluster when changing partition
  count. Increasing partitions on a populated topic can remap keys and break
  the original ordering assumption.

## Executable evidence

`make -C platform/local verify-scale-lab` performs two phases:

1. Reject unfinished prior producers, then reset to and prove an idle
   one-replica baseline with a present zero-lag metric.
2. Publish a deterministic burst while a bounded archive delay creates
   backpressure.
3. Observe positive KEDA lag and scaling from one to three consumers.
4. Delete one scale-consumer pod and require the replacement pod to join the
   stable Kafka group with a partition assignment.
5. Require lag to drain and validate exact record count, key agreement,
   per-instrument contiguous sequence, and stable partition assignment.
6. Replay byte-identical values, compare privacy-safe phase digests, and require
   exactly `N` duplicate archive effects with zero creates, quarantines, or
   errors.
7. Require the Deployment to settle back to one replica.

The full `e2e-ephemeral`/PS2 path then deletes and rebuilds analytics from
retained bronze inputs and reproduces the neutral portfolio result.

The versioned JSON report records:

- producer throughput and broker-acknowledgement p95 for each phase;
- a diagnostic cumulative Kafka-to-durable-archive p95;
- maximum sampled KEDA consumer lag;
- maximum and settled consumer replicas;
- assigned Kafka consumer-group recovery time;
- aggregate ordering and partition evidence;
- exact replay outcome deltas and value-digest equality; and
- the explicit `local_kind_synthetic` evidence scope.

The default run uses 1,200 events and a 100 millisecond archive delay per
phase. Larger local experiments are explicit:

```bash
SCALE_EVENT_COUNT=10000 \
SCALE_ARCHIVER_DELAY_MS=15 \
SCALE_PHASE_TIMEOUT_SECONDS=900 \
  make -C platform/local verify-scale-lab
```

The event-count and delay combination must retain at least 35 seconds of
injected work per partition. Extending the phase timeout does not fix a burst
whose lag disappears before HPA observes it. Each run clears the known evidence
files first, streams aggregate scale samples, and writes a bounded resource
snapshot on failure so an always-upload step cannot publish stale evidence as
the current run.

Use the disposable end-to-end workflow for large tests so Kafka data, object
storage, images, volumes, and the Colima VM are deleted afterward.

Do not use the diagnostic latency value in this acceptance as capacity
evidence: the acceptance includes replay, consumer replacement, and an
artificial post-write window. The capacity benchmark instead uses exact
before/after histogram deltas with fixed consumers and no injected delay.

## Unsupported claims

This evidence does not prove AWS capacity, production availability, broker
failover, multi-zone durability, provider market-data entitlements, or a need
for high throughput in a personal portfolio. The local Kafka broker and Garage
service each have one replica. Large-provider ingestion must be measured
separately under the provider's rate, retention, and licensing limits.

References:

- [KEDA Kafka scaler](https://keda.sh/docs/2.20/scalers/apache-kafka/)
- [KEDA deployment requirements](https://keda.sh/docs/2.20/deploy/)
