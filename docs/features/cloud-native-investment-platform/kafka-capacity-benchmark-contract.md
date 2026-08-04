# Kafka Capacity Benchmark Contract

## Purpose

This benchmark quantifies one disposable local Kafka-to-object-storage path.
It is separate from the Kafka scale acceptance because the two experiments ask
different questions:

- `verify-scale-lab` injects a post-write delay and consumer replacement to
  prove autoscaling, replay, idempotency, ordering, and recovery.
- `verify-capacity-benchmark` fixes three consumers, removes the artificial
  delay, and measures repeated original-write trials.

Neither experiment claims that personal portfolio traffic needs this scale.
The benchmark uses only fixed fictional identifiers and the canonical market
event contract.

## Trial matrix

The default suite performs one unmeasured 600-event warm-up followed by five
trials at each of 10,000, 50,000, and 100,000 events. Trials run in ascending
size order against three Kafka partitions and three fixed archive consumers.
`CAPACITY_TARGET_RATE=0` means the producer is unbounded; an explicit positive
rate measures that configured arrival rate instead of burst capacity.

Each measured trial uses a unique run ID, original writes only, zero injected
archive delay, and before/after Prometheus snapshots. Replay remains in the
correctness profile because duplicate `HeadObject` work has a different cost
than an original `HeadObject` plus `PutObject` effect.

The runner captures each partition's newest offset immediately before the
producer and after durable completion. Ordering inspection reads only that
exact offset window. It rejects a window whose total delta is not `N`, avoiding
history-growing verification work as earlier trials accumulate in the topic.

## Measurement boundaries

| Measurement | Exact boundary |
| --- | --- |
| Producer throughput | Event generation through acknowledgement from the single local Kafka broker |
| Producer p50/p95/p99 | Per-message enqueue-to-broker-acknowledgement duration |
| Durable throughput | Kubernetes Job submission through zero committed lag and exact archive-object count; includes Job startup and bounded polling overhead |
| Durable p50/p95/p99 | Kafka record timestamp through successful S3-compatible archive or quarantine acknowledgement, calculated from trial-scoped histogram deltas |
| Per-partition lag | Kafka newest offset minus the consumer group's committed offset, sampled directly for every partition |
| Lag drain | First sampled positive lag through the first post-producer zero-lag sample |
| CPU and memory | Peak sampled Prometheus twenty-five-second CPU rate and memory samples for the archivers, Kafka broker, and Garage, bounded by Job submission and durable completion |

The report keeps producer acknowledgement throughput separate from durable
end-to-end throughput. A broker acknowledgement does not prove that the
archive side effect occurred.

The local cAdvisor target is scraped at the kubelet's ten-second housekeeping
interval. Each CPU rate uses a twenty-five-second source-timestamped window
that starts no earlier than Job submission. The extra five seconds prevents
ordinary scrape jitter from leaving only one counter sample. Historical
queries retain their original start and end timestamps while retrying for up
to thirty seconds so Prometheus can ingest delayed scrapes without admitting
post-completion usage. A series that remains absent still fails the trial
instead of inventing a value. Memory queries reject source samples timestamped
before submission. Producer-container resources are not reported because a
short producer Job can finish before its first scrape; its acknowledgement
throughput and latency come from application telemetry.

Instant counter and histogram queries also retry for at most thirty seconds,
cap each Kubernetes API request at ten seconds, validate the required labeled
vector, and publish through an atomic rename. A suite-level pod snapshot makes
any baseline pod replacement or container restart fail the trial, so retrying
a recovering Prometheus endpoint cannot hide a platform interruption.

## Correctness assertions

A trial is rejected unless all of the following hold:

- Kafka acknowledged exactly `N` events.
- Topic inspection finds exactly `N` events for the run, with stable
  per-instrument partitioning and contiguous provider sequence.
- The archive contains exactly `N` unique objects for the run.
- Prometheus counter deltas contain exactly `N` creates and zero duplicates,
  quarantines, or errors.
- The durable-latency histogram delta contains exactly `N` observations.
- All three consumers remain available and measured CPU/memory series exist
  for the consumers, Kafka, and Garage.
- No baseline Kubernetes pod is replaced and no container restart count
  changes during the measured suite.
- The durable-completion window is long enough to contain a complete
  twenty-five-second CPU rate sample.

The summary is written atomically only after every planned trial passes. It
reports the median and p95 across all repetitions; it never selects the best
trial.

## Privacy and artifact boundary

Generated evidence is ignored by Git and stored under:

```text
artifacts/kafka-capacity/<suite-id>/
├── environment.json
├── plan.json
├── summary.json
└── runs/<run-id>/
    ├── report.json
    └── raw/
```

Raw evidence also contains trial-bounded Kafka offsets and the post-trial pod
stability snapshot; it contains no event payloads or archive keys.

Reports may contain the Git revision, clean/dirty state, CPU/memory/disk
allocation, component versions, aggregate counts, partition numbers, and
timings. They must not contain payloads, instrument names, watchlists,
credentials, usernames, hostnames, IP addresses, kubeconfigs, or account data.
Suite IDs are immutable: a run refuses an existing suite directory and never
deletes or replaces its evidence.

## Commands and cleanup

Run the default full matrix in a dedicated Colima profile:

```bash
make -C platform/local e2e-capacity-ephemeral
```

For reproducible local evidence, the command refuses to start while another
Colima profile is running and never stops that profile automatically. Stop an
existing development profile first (for example,
`colima stop investment-platform`). On macOS, the workflow uses `caffeinate`
when available to prevent idle sleep for its lifetime.

The workflow bootstraps the data path, runs the scale/recovery acceptance once,
runs the capacity matrix, preserves host-side aggregate artifacts, and deletes
the kind cluster, Docker data, volumes, images, and owned Colima VM on success,
failure, or interruption.

For a smaller explicit matrix in the same disposable workflow:

```bash
CAPACITY_EVENT_COUNTS=10000,50000 \
CAPACITY_REPETITIONS=3 \
  make -C platform/local e2e-capacity-ephemeral
```

`make -C platform/local verify-capacity-smoke` runs one 10,000-event trial at a
fixed 250 events per second against an already bootstrapped cluster and is the
bounded PS2 merge gate. The controlled rate guarantees enough in-boundary time
for the required twenty-five-second CPU sample; it is execution-path evidence,
not burst-capacity evidence, and does not replace the unbounded repeated matrix.
Direct runs must provide
`CAPACITY_ALLOCATED_CPUS`, `CAPACITY_ALLOCATED_MEMORY_GIB`, and
`CAPACITY_ALLOCATED_DISK_GIB`; the disposable Colima and CI workflows supply
those values from the runtime they own.

## Unsupported claims

This benchmark does not prove sustained provider-feed throughput, AWS or
production capacity, multi-broker replication, multi-zone availability,
broker or object-store failover, public deployment, or real-user traffic. The
single local Kafka broker and Garage replica are intentional cost boundaries.
Host contention can change results, so comparisons require equivalent recorded
environments and distributions rather than one favorable number.
