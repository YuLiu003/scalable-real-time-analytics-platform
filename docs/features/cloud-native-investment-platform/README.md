# Cloud-Native Investment Analytics Platform

| Field | Decision |
| --- | --- |
| Status | Implemented local platform; roadmap continues |
| Product goal | Transparent long-term portfolio analysis and contribution planning |
| Engineering goal | Observable and recoverable Kubernetes and event-driven system |
| Cost boundary | Required work runs locally or in included CI without a paid cloud apply |
| Instruments | QQQ, QQQM, FSELX, and S&P 500 benchmark fixtures |

## Why this project exists

The project joins one useful workload with a deliberate platform-engineering
curriculum:

1. Analyze holdings, benchmark performance, and monthly or biweekly
   contribution scenarios with explicit assumptions.
2. Practice Kafka delivery semantics, Kubernetes operations, object storage,
   replay, observability, CI/CD, infrastructure as code, and failure recovery.

Real portfolio traffic is low. Synthetic producers create configurable load for
partitioning, lag, backpressure, autoscaling, and recovery exercises. The
documentation must not claim that personal finance naturally requires this
scale.

## End-to-end system

```text
market producer
    -> Kafka market-prices topic
    -> raw-event archiver
    -> immutable bronze objects
    -> deterministic analytics job
    -> Parquet portfolio products
    -> portfolio API and dashboard
```

The same application and Kubernetes contracts run locally and inform the AWS
design. Provider-specific control planes remain separate:

| Responsibility | Free local implementation | AWS target |
| --- | --- | --- |
| Scheduling and rollout | kind | EKS |
| Streaming | Strimzi Kafka | Strimzi on EKS for the first lab |
| Object storage | Garage S3 API | S3 |
| Analytics | Python and DuckDB jobs | Kubernetes jobs using S3 |
| API | Go container | EKS workload; ECS contract is a roadmap exercise |
| Identity | Kubernetes service accounts | EKS Pod Identity |
| Infrastructure | OpenTofu tests | OpenTofu AWS provider |
| Presubmit | Disposable Jenkins on kind | Isolated Jenkins agents or managed CI |

## Implemented behavior

- Strict versioned market-event envelopes and deterministic fixtures.
- Synthetic producer cases for normal delivery and ambiguous acknowledgements.
- Kafka consumer recovery after a crash between object write and offset marking.
- Immutable S3-compatible archive writes with duplicate-versus-collision checks.
- Replay from bronze objects without relying on mutable derived state.
- DuckDB analytics and independent Parquet query verification.
- Holdings and contribution-projection APIs with transparent timing, return,
  expense, and inflation assumptions.
- Local Kubernetes readiness, diagnostics, monitoring, and automatic cleanup.
- AWS EKS, network, identity, encryption, registry, storage, observability, and
  budget contracts validated without an account apply.
- PS0, PS1, and PS2 gates shared by local development, GitHub, and Jenkins.

## Core invariants

- Duplicate delivery cannot create a second durable effect.
- The same canonical input set produces the same identity and result.
- Missing or invalid market prices fail closed without publishing partial data.
- A readiness endpoint fails when a required dependency is unavailable.
- Replay can rebuild derived products from immutable bronze data.
- No personal financial data, cloud credential, or Terraform state enters Git.
- Runtime resources are disposable and clean up after success or failure.

## Adding a producer

A new producer is complete only when it:

1. emits the canonical versioned event envelope;
2. uses a stable event ID and instrument key;
3. defines retry and ambiguous-acknowledgement behavior;
4. has deterministic fixtures for happy, invalid, duplicate, and failure cases;
5. preserves partition-ordering assumptions;
6. exposes metrics for throughput, errors, retries, and lag; and
7. passes replay and Kubernetes acceptance.

Producer-specific payloads belong behind the shared envelope. A new source
should not create a parallel archive or analytics contract unless its semantics
cannot fit the existing one.

## Deliberate boundaries

- The platform is read-only and educational; it does not place trades or give
  investment advice.
- PostgreSQL, Valkey, and a warehouse are added only when a concrete
  transactional, cache, or analytical requirement justifies them.
- Local S3 and workload identity contracts do not prove cloud IAM behavior.
- GCP and Azure are comparison milestones after the AWS learning path.
- Live market-data licensing and brokerage integrations are outside the current
  scope.

## Roadmap and evidence

- [Roadmap](roadmap-v2.md)
- [Quality gates](quality-gates.md)
- [Market event contract](slice-2-event-contract.md)
- [Analytics contract](slice-3-analytics-contract.md)
- [Contribution projection contract](contribution-projection-contract.md)
- [AWS streaming decision](aws-streaming-decision.md)
- [Free-first lab strategy](free-first-lab-strategy.md)
- [Local Kubernetes runbook](../../../platform/local/README.md)
- [Jenkins verification](../../../platform/jenkins/VERIFICATION.md)
