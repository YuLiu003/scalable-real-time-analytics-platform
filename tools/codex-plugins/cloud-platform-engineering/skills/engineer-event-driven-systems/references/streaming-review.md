# Event-Driven Systems Review

## Event contract

- Stable event ID and documented idempotency scope.
- Schema name/version, compatibility policy, required fields, and unknown-field behavior.
- Event time, ingestion time, tenant, producer, key, and trace/correlation context.
- Data classification, retention, deletion, and replay authorization.

## Producer

- Key selection matches ordering and load-distribution requirements.
- Acknowledgements and replication match durability objectives.
- Retries cannot silently reorder or duplicate unacceptable effects.
- Timeouts, batching, compression, and maximum record size are bounded.

## Consumer and sink

- Consumer groups match independent subscriber semantics.
- Concurrency does not exceed useful partition parallelism without a reason.
- Offset commit occurs only after the intended durable effect.
- Duplicate processing is safe through a unique event ID, upsert, inbox, or atomic offset/effect transaction.
- Invalid and poison records have bounded retries, quarantine, alerting, and replay procedures.
- Rebalances and shutdown stop intake, finish or abandon work deliberately, and commit safely.

## Operations

- Monitor per-partition lag, processing rate, error/retry rate, rebalance duration, ISR/replication health, disk, and end-to-end latency.
- Scale from backlog and service time, respecting partition limits and downstream capacity.
- Test broker, consumer, network, and sink failures plus restoration and replay.
- Define topic ownership, quotas, ACLs, certificate rotation, upgrade compatibility, and disaster recovery.

Exactly-once is an end-to-end claim. State its boundary and prove it with crash tests.
