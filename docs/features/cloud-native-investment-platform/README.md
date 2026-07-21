# Feature Proposal: Cloud-Native Investment Analytics Platform

| Field | Value |
| --- | --- |
| Status | Accepted; Slices 1, 2, and 3 complete |
| Current branch | `feature/portfolio-analytics-slice3` |
| Scope owner | Repository maintainers |
| Last updated | 2026-07-21 |
| First environment | Local multi-node Kubernetes with `kind` |
| First cloud target | AWS; GCP and Azure follow as comparison exercises |
| Review gate | Approve the goals, boundaries, and first vertical slice before implementation |

## Summary

Evolve the existing sensor-oriented real-time analytics repository into a
cloud-platform engineering capstone using personal investment analytics as the
stable workload.

The product workload is deliberately small:

1. Producers ingest market, reference, and portfolio events.
2. Kafka transports versioned canonical events.
3. An archiver persists immutable raw events to object storage.
4. Processors produce cleaned Parquet datasets and portfolio aggregates.
5. A Go API and dashboard expose the results.

The primary outcome is not a commercial finance product or a claim that a
personal portfolio requires hyperscale infrastructure. The outcome is
reproducible evidence that the platform can be provisioned, deployed, observed,
broken, recovered, secured, and compared across cloud providers.

This proposal follows the useful parts of the
[Kubernetes Enhancement Proposal template](https://github.com/kubernetes/enhancements/blob/master/keps/NNNN-kep-template/README.md):
goals, non-goals, risks, test plans, rollout, rollback, and graduation criteria.
Implementation should follow Google's guidance on
[small, reviewable changes](https://google.github.io/eng-practices/review/developer/small-cls.html)
and record both what changed and why.

## Motivation

The repository already contains Go services, Kafka, Kubernetes manifests,
Terraform, Helm, Prometheus, and Grafana. However, the presence of manifests or
multiple replicas does not prove distributed correctness, availability, safe
delivery, or recovery.

This feature gives the repository one coherent purpose:

> Build and operate a portable event-driven analytics workload to learn
> Kubernetes, cloud infrastructure and services, distributed-system behavior,
> infrastructure as code, security, observability, and reliability.

Investment data is a suitable workload because it naturally includes multiple
producers, batch and streaming ingestion, historical corrections, ordering,
deduplication, replay, sensitive data, and analytical queries.

## Goals

- Keep Kubernetes as the primary learning and operating control plane.
- Maintain one thin end-to-end workload throughout the curriculum.
- Run the core platform locally with free and open-source components.
- Deploy the same workload to one real cloud before comparing all three.
- Make a new producer an isolated adapter and contract change, not a pipeline
  rewrite.
- Define event identity, schemas, partitioning, delivery, retry, and replay
  behavior explicitly.
- Persist enough immutable data to reconstruct derived state.
- Provision cloud infrastructure declaratively with OpenTofu.
- Deliver workloads through Helm and GitOps.
- Demonstrate workload identity and least-privilege access in real cloud labs.
- Produce metrics, logs, traces, alerts, runbooks, and failure-test evidence.
- Replace unsupported "production ready" claims with reproducible verification.

## Non-goals

- Building a brokerage, trading system, tax engine, or financial adviser.
- Claiming that personal finance data requires Kafka or Kubernetes for scale.
- Reimplementing Databricks, AWS, GCP, or Azure.
- Installing every CNCF project in the first milestone.
- Running active-active production across three clouds.
- Guaranteeing end-to-end exactly-once processing without a proven boundary.
- Using real account credentials or private financial records in shared demos.
- Migrating or deleting the existing sensor pipeline before the replacement path
  satisfies its graduation criteria.

## User stories

### Platform learner

As a platform engineer, I can create a local cluster from an empty environment,
deploy the platform declaratively, inspect its health, introduce a failure, and
recover it using a documented procedure.

### Producer developer

As a producer developer, I can add another market or portfolio source without
changing existing consumers when I emit an existing canonical event contract.

### Data consumer

As a data consumer, I can query trusted historical portfolio datasets without
depending on the availability of the original producer.

### Cloud engineer

As a cloud engineer, I can deploy the same thin workload to AWS and later explain
which responsibilities map differently to GCP and Azure.

## End-to-end scope

```text
Market API     Broker import     Synthetic generator
    |                |                    |
    +---------- producer adapters --------+
                     |
          validate and normalize
                     |
                   Kafka
             +-------+--------+
             |                |
       raw archiver     stream processor
             |                |
       object storage         |
       bronze: raw JSON       |
       silver: Parquet        |
       gold: aggregates <-----+
             |
       DuckDB / cloud SQL
             |
        Go API and dashboard
             |
       optional Valkey cache
```

### Delivery contract

The initial processing contract is **at-least-once delivery with idempotent
effects**.

- An acknowledgement from a producer means Kafka accepted the event according
  to the configured acknowledgement and replication policy.
- A consumer commits an offset only after its intended durable effect succeeds.
- Reprocessing the same `event_id` must not double-count a portfolio effect.
- Raw archived events are replayable.
- Exactly-once may only be claimed for a documented boundary backed by crash
  tests.

## Proposed stack

### Required for the first complete local slice

| Responsibility | Choice | Boundary |
| --- | --- | --- |
| Services and producers | Go | Domain logic and adapters |
| Local Kubernetes | `kind` | Disposable multi-node learning cluster |
| Event transport | Kafka managed by Strimzi | Durable event transport and replay window |
| Event contracts | JSON Schema with Apicurio Registry | Compatibility and validation |
| Local object storage | Garage or another actively maintained S3-compatible store | Immutable raw and analytical files |
| Analytical format | Parquet | Portable columnar datasets |
| Local analytical query | DuckDB | Embedded SQL over Parquet |
| Infrastructure as code | OpenTofu | Local bootstrap and cloud resources |
| Workload packaging | Helm | Reusable Kubernetes releases |
| Continuous delivery | Argo CD | Reconciliation, promotion, and rollback |
| Telemetry | OpenTelemetry | Vendor-neutral instrumentation |
| Metrics and dashboards | Prometheus and Grafana | Health, SLIs, and learning evidence |

### Cloud mappings

| Responsibility | Local | AWS | GCP | Azure |
| --- | --- | --- | --- | --- |
| Kubernetes | kind | EKS | GKE | AKS |
| Event streaming | Strimzi Kafka | MSK or Strimzi on EKS | Managed Kafka or Strimzi on GKE | Event Hubs Kafka endpoint or Strimzi on AKS |
| Object storage | S3-compatible store | S3 | Cloud Storage | Blob Storage |
| Registry | Local registry; Harbor later | ECR | Artifact Registry | ACR |
| Identity | Kubernetes service accounts | IAM workload identity | Workload Identity Federation | Managed identities |
| Secrets and keys | Kubernetes Secrets initially; OpenBao later | Secrets Manager and KMS | Secret Manager and Cloud KMS | Key Vault |
| Analytics | DuckDB | Athena | BigQuery | Serverless SQL equivalent |
| Monitoring integration | OpenTelemetry stack | CloudWatch integration | Cloud Monitoring integration | Azure Monitor integration |

Cloud-specific APIs are hidden behind narrow application interfaces where
portability is useful. Provider-specific infrastructure remains explicit in
OpenTofu modules so that differences in identity, networking, encryption, and
service behavior are learned rather than concealed.

### Deferred until justified

- PostgreSQL or CloudNativePG: add for transactional application state, manual
  editing, users, or relational integrity.
- Valkey: add only for rebuildable caches, sessions, rate limits, or hot derived
  state.
- OpenBao: add when the secrets and dynamic-credential milestone begins.
- Knative: add for a deliberate serverless or scale-to-zero experiment.
- Harbor: add when registry administration is itself a learning objective.
- KEDA: add after consumer lag and downstream capacity are measured.
- Flink, Spark, Trino, or Iceberg: add only when a milestone requires their
  processing or table semantics.
- GCP and Azure production-like environments: begin after the AWS slice meets
  its acceptance criteria.

## Event contract

Canonical events use a shared envelope:

```json
{
  "event_id": "polygon:trade:982734",
  "event_type": "market.price.observed",
  "schema_version": 1,
  "source": "polygon",
  "tenant_id": "demo",
  "occurred_at": "2026-07-20T18:30:00Z",
  "ingested_at": "2026-07-20T18:30:01Z",
  "partition_key": "AAPL",
  "trace_id": "01J...",
  "payload": {}
}
```

Requirements:

- `event_id` is stable across producer retries.
- Monetary values use fixed-point integers or decimal strings, not binary
  floating point.
- Market events are normally keyed by instrument to preserve per-instrument
  ordering.
- Portfolio transactions are keyed by account or portfolio according to the
  required ordering scope.
- Schemas define compatibility and unknown-field behavior.
- Invalid source records are quarantined with bounded retention and an operator
  signal.
- Personal or secret data is classified before retention and replay are enabled.

Initial topic families:

```text
market.prices
portfolio.transactions
reference.instruments
portfolio.snapshots
ingestion.quarantine
processing.dead-letter
```

Topics represent business event families, not vendor names. Source-specific raw
responses remain separated in object storage.

## Adding a producer

A new producer is accepted through the following workflow:

1. Identify whether it emits an existing event type or requires a new contract.
2. Define a stable source event ID and partition key.
3. Add or evolve the schema under the documented compatibility policy.
4. Implement the source adapter behind the shared producer library.
5. Archive the untouched source response when replay or audit requires it.
6. Grant a dedicated service account write access only to its allowed topics and
   storage prefix.
7. Package the producer as a Deployment, CronJob, or Job according to its
   execution model.
8. Add metrics for attempted, accepted, rejected, retried, and quarantined
   events.
9. Test timeout, duplicate delivery, malformed input, source outage, and graceful
   shutdown.
10. Deploy through Helm and Argo CD and record the verification evidence.

If the new producer emits an existing canonical contract, downstream consumers
must not require source-specific changes. A separate topic requires a documented
reason such as isolation, incompatible schema, retention, security, or throughput.

## Platform boundaries

- Use separate namespaces for platform add-ons and application workloads.
- Use service accounts per workload; do not share cloud credentials.
- Default-deny network policy is introduced only with explicit required flows and
  a recovery procedure.
- Stateful components define storage, backup, restore, rescheduling, and
  disruption behavior before being called highly available.
- Local Kubernetes is for learning and integration, not evidence of cloud-zone
  availability.
- In cloud environments, prefer managed stateful services when the learning goal
  is application/platform integration; self-host them only when operating the
  service is the explicit lab.
- Infrastructure provisioning, cluster add-ons, and application delivery have
  separate lifecycles.

## Delivery plan

Each slice should be a small pull request that leaves the repository usable and
includes its verification evidence.

### Slice 0: Approve the proposal

Deliverables:

- This feature README.
- Agreed goals, non-goals, first cloud, and first end-to-end path.
- Issues or follow-up ADRs for unresolved component decisions.

Proof:

- Review approval records why the direction was selected.

### Slice 1: Reproducible local platform baseline

Deliverables:

- Version-pinned local `kind` cluster configuration.
- Namespace and ownership model.
- Helm and GitOps repository layout.
- Prometheus and Grafana baseline.

Proof:

- A clean machine can create, verify, and delete the environment through
  documented commands.
- A failed bootstrap produces actionable diagnostics.

### Slice 2: First canonical producer-to-storage path

Deliverables:

- Shared event envelope and schema policy.
- One synthetic or public market-data producer.
- Strimzi-managed Kafka.
- Raw event archiver and object storage layout.

Proof:

- Accepted events appear in Kafka and immutable object storage.
- Duplicate and malformed events have the documented outcomes.
- Producer and consumer crashes are tested at acknowledgement boundaries.

### Slice 3: Analytics and user-visible result

Deliverables:

- Raw-to-Parquet transformation.
- Portfolio performance or allocation calculation.
- DuckDB query path.
- Minimal Go API and dashboard result.

Proof:

- The same input produces a deterministic analytical result.
- A replay reconstructs the result without contacting the original producer.

### Slice 4: Delivery, observability, and rollback

Deliverables:

- Argo CD reconciliation.
- OpenTelemetry correlation across producer, Kafka processing, and API.
- SLIs, alerts, runbook, and rollback procedure.

Proof:

- A deliberately broken release is detected and rolled back.
- An operator can follow a trace from ingestion to the visible result.

### Slice 5: Stateful and node-failure exercises

Deliverables:

- Pod and node failure experiments.
- Backup and restore procedure for durable state.
- Kafka lag and backpressure exercise.

Proof:

- Recovery time and data outcome are measured and recorded.
- Claims are scoped to the failure domain actually tested.

### Slice 6: AWS deployment

Deliverables:

- OpenTofu modules for network, identity, Kubernetes, registry, object storage,
  encryption, and required managed services.
- Workload identity with no static AWS access key in Kubernetes.
- Budget guardrails and documented teardown.

Proof:

- The same workload runs on AWS.
- An unauthorized workload is denied object access and an authorized workload
  succeeds.
- The environment can be destroyed without orphaning in-scope resources.

### Slice 7: GCP and Azure comparison

Deliverables:

- Minimal equivalent deployment or focused service labs.
- Decision record comparing identity, networking, storage, managed Kubernetes,
  observability, operations, and cost boundaries.

Proof:

- Differences are demonstrated with provider APIs and runtime evidence, not only
  a service-name mapping table.

## Observability and reliability targets

Initial targets are hypotheses to test, not production guarantees:

- Every event includes a correlation or trace identifier.
- Producer success, failure, retry, and quarantine rates are measurable.
- Consumer lag is visible per consumer group and partition.
- End-to-end latency is measured from `occurred_at`/`ingested_at` to analytical
  availability.
- Alerts describe an operator action and link to a runbook.
- Graceful shutdown, rebalance, duplicate delivery, sink outage, and replay are
  tested.
- Recovery documentation records the tested RPO, RTO, and failure boundary.

## Security and privacy

- Use synthetic or public market data by default.
- Never commit brokerage credentials, tokens, account numbers, or personal
  exports.
- Use separate identities and least-privilege permissions for producers,
  consumers, operators, and CI.
- Encrypt cloud storage and transport; document local-development exceptions.
- Record who can replay, export, or delete retained financial events.
- Keep quarantine and dead-letter payload access restricted because failed
  records may contain sensitive source data.
- Add audit evidence for cloud identity and object access in the AWS milestone.

## Rollout and rollback

- Build the investment path alongside the current sensor path.
- Introduce new namespaces, topics, and storage prefixes without reusing or
  deleting existing data.
- Enable workloads through Git-managed desired state.
- Roll back application releases by reverting the GitOps revision.
- Preserve compatible raw events across rollback so derived data can be rebuilt.
- Treat incompatible schema rollback as a migration requiring an explicit plan.
- Remove the legacy path only in a separate proposal after the new path graduates.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Tool sprawl replaces learning | Add a component only when a milestone introduces a new responsibility or failure experiment. |
| Personal workload does not justify the scale | State that infrastructure complexity is educational and keep the workload thin. |
| Three-cloud breadth prevents depth | Complete local Kubernetes and AWS before GCP and Azure comparisons. |
| Stateful services overwhelm a laptop | Start single-purpose and resource-bounded; move advanced HA labs to suitable infrastructure. |
| Duplicate or replayed events corrupt results | Stable event IDs, idempotent sinks, deterministic replay tests, and post-effect offset commits. |
| Sensitive financial data leaks | Use synthetic data by default and enforce identity, secret, logging, and retention rules. |
| Cloud labs create unexpected cost | Apply budgets, TTL labels, small environments, teardown verification, and no unattended multi-cloud clusters. |
| Existing repository claims exceed evidence | Replace claims with links to tests, measurements, dashboards, or recovery reports. |
| Emulators hide provider differences | Use them for development only and verify provider-specific behavior in short-lived real-cloud labs. |

## Alternatives considered

### Continue with the IoT/sensor domain only

This remains technically valid, but investment analytics provides more natural
examples of multiple producers, immutable history, corrections, and analytical
data products. Existing sensor components can be reused while the new path is
built.

### Make PostgreSQL the primary system of record

This is preferable for an application centered on interactive transactional
editing. It is not required for the initial read-mostly analytics workload, which
can reconstruct derived state from immutable events and object storage.

### Make Valkey the primary database

Rejected because the initial workload requires durable, replayable history.
Valkey remains appropriate for rebuildable hot state and caches.

### Adopt a hosted lakehouse platform

This would accelerate analytics delivery but hide many of the Kubernetes, data,
identity, and infrastructure responsibilities that the project is intended to
teach.

### Build a full local OpenStack cloud first

Rejected for the initial scope because it adds substantial private-cloud
operational complexity before the core Kubernetes and workload behaviors are
proven.

## Graduation criteria

The feature is complete when all of the following have reproducible evidence:

- [ ] A clean local environment can be provisioned and removed from documented,
      version-pinned inputs.
- [ ] A canonical event travels from a producer through Kafka to immutable object
      storage and a user-visible analytical result.
- [ ] A second producer emitting the same contract is added without modifying
      existing consumers.
- [ ] Duplicate, invalid, delayed, and replayed events produce documented and
      tested outcomes.
- [ ] Consumer offsets and durable effects follow the declared at-least-once
      contract.
- [ ] Metrics, logs, traces, deployment metadata, and actionable alerts cover the
      end-to-end path.
- [ ] Pod, process, dependency, and node failure experiments have recorded
      recovery results.
- [ ] Durable data has a tested backup or reconstruction procedure.
- [ ] GitOps rollback of a broken application release is demonstrated.
- [ ] AWS infrastructure is provisioned with OpenTofu and uses workload identity
      instead of static credentials.
- [ ] Teardown and cost guardrails are tested for the AWS environment.
- [ ] GCP and Azure differences are documented from focused runtime exercises.
- [ ] The root documentation no longer makes unqualified production-readiness or
      high-availability claims.

## Open decisions

These decisions require follow-up ADRs or implementation evidence:

1. Validate Garage's S3 compatibility and operational behavior beyond the
   single-node Slice 2 learning environment before using it for a resilient
   deployment.
2. Select JSON Schema serialization details and Apicurio compatibility policy.
3. Choose Strimzi-on-EKS versus MSK for the first AWS streaming milestone based
   on learning objective and cost boundary.
4. Define the first public or synthetic data source and its rate limits.
5. Define canonical transaction and cash-flow events before adding portfolio
   performance calculations; Slice 3 deliberately proves allocation first.
6. Set measured resource budgets for the local cluster.
7. Decide when the existing sensor pipeline is deprecated, if at all.

## Change strategy

The proposal is documentation-only. After approval, implementation should be
split by the delivery slices above. Each pull request should include:

- A concise statement of what changed and why.
- Tests and runtime verification appropriate to the slice.
- New or changed failure modes.
- Rollout and rollback instructions.
- Observability and security effects.
- Follow-up work that is explicitly out of scope.

Large repository-wide rewrites should not be combined with the first working
vertical slice.

## Implementation tracking

| Slice | Status | Evidence |
| --- | --- | --- |
| 0: Approve the proposal | Complete | Scope accepted on 2026-07-20; commit `8d953a2` |
| 1: Reproducible local platform baseline | Complete | [`verification record`](../../../platform/local/VERIFICATION.md) |
| 2: Producer-to-storage path | Complete | [`contract and failure model`](slice-2-event-contract.md); [`verification record`](../../../platform/local/SLICE2-VERIFICATION.md) |
| 3: Analytics and visible result | Complete | [`analytics and replay contract`](slice-3-analytics-contract.md); [`verification record`](../../../platform/local/SLICE3-VERIFICATION.md) |
| 4-7 | Not started | Graduation evidence will be linked as each slice begins |
