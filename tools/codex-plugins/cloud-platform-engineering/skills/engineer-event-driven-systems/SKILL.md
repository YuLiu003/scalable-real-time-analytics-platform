---
name: engineer-event-driven-systems
description: Design, review, test, and operate Kafka and event-driven distributed systems. Use for topics and partitions, keys and ordering, consumer groups, offset commits, retries, dead-letter handling, schemas, idempotency, transactions, replay, stream processing, backpressure, lag-based scaling, multi-tenancy, Kafka on Kubernetes, Strimzi, MSK, or delivery-semantics failures.
---

# Engineer Event-Driven Systems

Start with the end-to-end effect and failure boundary. Broker durability alone does not define processing semantics.

## Workflow

1. Draw the full path from producer intent through broker, consumer, side effects, and user-visible result.
2. Define event identity, key, schema, event time, tenant, and trace context.
3. Define ordering scope, partition count, replication, retention, compaction, and replay requirements.
4. Define producer acknowledgement, retry, timeout, and idempotence behavior.
5. Define consumer group membership, offset ownership, rebalance behavior, concurrency, and poison-message policy.
6. Choose a delivery contract and make sinks idempotent or make offset and effect atomic where possible.
7. Define lag, throughput, backpressure, capacity, autoscaling, and recovery objectives.
8. Use `cloud-docs` to verify Kafka, Strimzi, KEDA, and managed-service behavior for the selected versions.
9. Test crashes at every boundary, duplicate delivery, reordering, replay, schema evolution, broker loss, and sink outage.

Read [streaming-review.md](references/streaming-review.md) before asserting exactly-once behavior or scaling a consumer deployment.
