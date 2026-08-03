# Investment platform roadmap v2

| Field | Decision |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-26 |
| Product objective | Private, transparent long-term portfolio planning |
| Engineering objective | Observable, recoverable event-driven cloud platform |
| Cost boundary | Required work must run locally or in included CI without a paid cloud apply |

## Direction

The project has two equally important paths that share one workload:

1. A useful read-only investment planner for contribution scenarios, imported
   holdings, actual performance, and benchmark comparisons.
2. A distributed-systems lab that applies real Kafka and Kubernetes failure
   behavior plus free local equivalents of Lambda, DynamoDB, ECS, and Jenkins.

Real portfolio traffic is deliberately low. Synthetic producers provide the
separate, configurable load needed to measure partitions, consumer lag,
backpressure, autoscaling, and recovery. The documentation must not claim that
a personal portfolio naturally requires hyperscale infrastructure.

## Delivery roadmap

Each slice must leave the repository usable and must attach executable proof.

| Slice | Branch | Deliverable | Required proof | Status |
| --- | --- | --- | --- | --- |
| 0 | Existing stack | kind, Kafka, object storage, analytics, API, quality gates, EKS/OpenTofu static lab, disposable Colima | Reproducible lifecycle, replay, 100% application coverage | Complete |
| 1 | `feature/contribution-projection-engine` | Monthly/biweekly projection API and dashboard with return range, inflation, expenses, and transparent assumptions | Golden domain/API tests and Kubernetes API acceptance | Implemented |
| 2 | `feature/personal-portfolio-ledger` | Read-only manual/CSV transaction import and normalized cash-flow ledger | Redacted fixtures; deposits and withdrawals cannot be confused with return | Not started |
| 3 | `feature/aws-event-projection-local` | Lambda-compatible Kafka batch handler and DynamoDB Local projection | Duplicate delivery cannot double-apply an event; replay rebuilds state | Not started |
| 4 | `feature/aws-ecs-runtime-contracts` | Existing API image, ECS task/service definitions, IAM and health contracts | Static OpenTofu tests plus process replacement and graceful-stop evidence | Not started |
| 5 | `feature/kafka-scale-lab` | Adjustable producers and partition-aware consumers | Measured throughput, p95 latency, lag, backpressure, and bounded recovery | Implemented; exact-head PS2 evidence is required for each merge |
| 5A | `feature/private-market-feed-adapter` | User-selected runtime watchlist and credential-backed live provider adapter using the canonical event contract | Redacted contract tests, reconnect/rate-limit recovery, and proof that no watchlist or credential enters Git, metrics, or CI artifacts | Not started |
| 6 | `feature/platform-observability-rollback` | OpenTelemetry, actionable alerts, Argo CD reconciliation, rollback | Trace across the event path and a detected, rolled-back bad release | Not started |
| 7 | `feature/presubmit-quality-gates` | Credential-free Jenkins Pipeline calling repository-owned `PS0`/`PS1`/`PS2` targets | Isolated agents, exact-commit gate, GitHub status, automatic cleanup | Implemented; per-head proof is `jenkins / presubmit` |
| 8 | `feature/free-cloud-provider-contracts` | AWS/GCP/Azure IaC mocks and provider responsibility comparison | Validated configuration and documented emulator gaps; no paid apply | Not started |

## Required free boundary

The local implementations are required curriculum, not optional mentions:

- Real kind Kubernetes, Strimzi Kafka, Garage, Prometheus, and Grafana.
- AWS SAM local runtime for Lambda-compatible handlers.
- DynamoDB Local through the real AWS SDK API surface.
- Real application containers plus validated ECS task and IAM contracts.
- Real disposable Jenkins controller and pipeline.
- OpenTofu mocked-provider tests for cloud infrastructure.

A paid AWS, GCP, or Azure apply is not a completion requirement. Local
emulation does not prove cloud control-plane behavior, and documentation must
state that limitation rather than claim production provider experience.

## Adjacent developer tooling

The repository also contains an independent
[`agent-review-optimizer`](../agent-review-optimizer/README.md) Codex plugin.
Its local MVP is implemented: deterministic reviewer routing, privacy-safe
Codex JSONL usage extraction, and aggregate outcome summaries. It is not an
investment workload and does not justify Kafka or Kubernetes by itself. A
future producer may publish only its allowlisted aggregate record after a
seeded benchmark proves the measurements useful; prompts, code, diffs, paths,
identity, session, and financial data remain outside that event boundary.

## Definition of done for every slice

- Domain behavior is implemented, not hardcoded in a dashboard.
- Application statements remain at 100% measured coverage.
- Happy-path, invalid-input, duplicate, dependency-failure, and recovery
  behavior appropriate to the slice are tested.
- CI and local commands use the same repository-owned gates.
- Runtime resources are disposable and clean up on success and failure.
- No brokerage credentials, account identifiers, personal exports, cloud
  access keys, state, or secrets enter Git.
- The result includes a verification record and honest unsupported boundaries.
