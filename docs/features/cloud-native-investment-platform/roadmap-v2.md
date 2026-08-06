# Investment platform roadmap v2

| Field | Decision |
| --- | --- |
| Status | Active |
| Last updated | 2026-08-05 |
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

## Milestone numbering

IDs in this document use the `V2-` prefix. Older files named Slice 1, Slice 2,
or Slice 3 are historical delivery records that now make up the V2-0 baseline;
they are not the unprefixed equivalents of V2 roadmap milestones.

## Delivery roadmap

Each milestone must leave the repository usable and must attach executable
proof.

| Milestone | Branch | Deliverable | Required proof | Status |
| --- | --- | --- | --- | --- |
| V2-0 | Existing stack | kind, Kafka, object storage, analytics, API, quality gates, EKS/OpenTofu static lab, disposable Colima | Reproducible lifecycle, replay, 100% application coverage | Complete |
| V2-1 | `feature/contribution-projection-engine` | Monthly/biweekly projection API and dashboard with return range, inflation, expenses, and transparent assumptions | Golden domain/API tests and Kubernetes API acceptance | Implemented |
| V2-2 | `feature/personal-portfolio-ledger` | Read-only manual/CSV transaction import and normalized cash-flow ledger | Redacted equivalent fixtures; strict invalid-input tests; deposits and withdrawals remain external flows, not return | Implemented offline importer; not yet consumed by performance analytics |
| V2-3 | `feature/aws-event-projection-local` | Lambda-compatible Kafka batch handler and DynamoDB Local projection | Duplicate delivery cannot double-apply an event; replay rebuilds state | Not started |
| V2-4 | `feature/aws-ecs-runtime-contracts` | Existing API image, ECS task/service definitions, IAM and health contracts | Static OpenTofu tests plus process replacement and graceful-stop evidence | Not started |
| V2-5 | `feature/kafka-scale-lab` | Adjustable producers and partition-aware consumers | Ordering, replay, lag-driven 1-to-3-to-1 scaling, and bounded consumer replacement | Implemented correctness/recovery baseline; not a capacity benchmark |
| V2-5A | `feature/private-market-feed-adapter` | User-selected runtime watchlist and credential-backed live provider adapter using the canonical event contract | Fake-provider contract tests, reconnect/rate-limit recovery, and proof that no watchlist or credential enters Git, metrics, or CI artifacts | Implemented and contract-tested; real credential smoke remains operator-run and unverified |
| V2-5B | `feature/kafka-capacity-benchmark` | Repeatable 10K/50K/100K synthetic trials in a disposable local environment | Durable throughput, p50/p95/p99 latency, per-partition lag, scenario-scoped CPU/memory, and exact loss/duplicate/order results across repeated runs | Complete locally; PS2 paced smoke `c216025414` and unbounded 15-trial suite `c216031559` passed on `ca93f5a` |
| V2-5C | `feature/private-portfolio-inputs` | Private stock/ETF allocation and contribution-planning workflow over the live-feed contract | Strict external inputs, fictional end-to-end Kubernetes acceptance, private logging, protected API, and no committed private data | Complete locally; credential-free PS2 acceptance passed, while real provider smoke remains operator-run and unverified |
| V2-6 | `feature/platform-observability-rollback` | OpenTelemetry, actionable alerts, Argo CD reconciliation, rollback | Trace across the event path and a detected, rolled-back bad release | Not started |
| V2-7 | `feature/presubmit-quality-gates` | Credential-free Jenkins Pipeline calling repository-owned `PS0`/`PS1`/`PS2` targets | Isolated agents, exact-commit gate, GitHub status, automatic cleanup | Implemented; per-head proof is `jenkins / presubmit` |
| V2-7A | `feature/jenkins-garage-retention` | Bounded Jenkins history, dedicated Garage artifact storage, and a foreground operator UI across disposable lab runs | Cross-run restore/readback, three-day/20-build controller policy, three-day Garage lifecycle, 1 GiB bucket quota, least-privilege key, owner-token fencing, local-disk report, at-most-4-GiB guard, VM cleanup, UI lifecycle contracts, and disposable login/artifact smoke | Complete locally; retention, artifact readback, operator login, and cleanup proof are recorded below |
| V2-8 | `feature/free-cloud-provider-contracts` | AWS/GCP/Azure IaC mocks and provider responsibility comparison | Validated configuration and documented emulator gaps; no paid apply | Not started |

## Verified V2-5B capacity evidence

Suite `c216031559` ran from clean commit
`ca93f5aa421224d34ecac1bacd3f089583f77374` on one disposable arm64,
three-node kind environment allocated 4 CPUs, 8 GiB memory, and 30 GiB disk.
It used three Kafka partitions, three fixed consumers, one Kafka broker, one
Garage object-store replica, no injected archive delay, and an unbounded
producer. All 15 planned reports passed exact acknowledged/topic/archive
counts, ordering, zero loss, zero unexpected duplicates, zero quarantine or
errors, stable pod identity, and no container restart-count changes during the
measured suite.

| Events | Repetitions | Producer median events/s | Durable median events/s | Producer ack p95 median | Durable p95 median | Median max lag | Median durable completion |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10,000 | 5 | 43,373.79 | 402.97 | 65.00 ms | 28.06 s | 9,786 | 24.816 s |
| 50,000 | 5 | 48,977.88 | 424.25 | 93.95 ms | 113.37 s | 48,965 | 117.856 s |
| 100,000 | 5 | 46,115.17 | 387.95 | 109.88 ms | 283.09 s | 99,079 | 257.768 s |

The unbounded 10K scenario intentionally omitted CPU/memory because its short
durable boundary cannot reliably contain the required 25-second CPU window.
The 50K/100K scenarios required all six Prometheus series. Their p95
distributions of per-run resource peaks were:

| Events | Archiver CPU / memory | Kafka CPU / memory | Garage CPU / memory |
| ---: | ---: | ---: | ---: |
| 50,000 | 0.402 cores / 43.84 MiB | 0.085 cores / 887.77 MiB | 0.500 cores / 365.11 MiB |
| 100,000 | 0.412 cores / 44.05 MiB | 0.168 cores / 917.91 MiB | 0.501 cores / 495.88 MiB |

With five repetitions, the nearest-rank p95 is the maximum observed value.
Generated reports remain ignored under `artifacts/kafka-capacity/`; this
committed record preserves only aggregate, privacy-safe results. These are
local synthetic burst measurements, not live-provider, sustained-feed, AWS,
multi-broker, multi-zone, failover, public-deployment, or production evidence.

## Verified V2-7A local retention evidence

At commit `6197e8a7acdb5e47dbd7c41616b3ef679669cb2c`, two consecutive
Jenkins builds each ran `PS0`, `PS1`, and `PS2` in a fresh disposable
Colima/kind runtime using the same bounded host-retained directory. The second
runtime restored build `#1`, and one of that build's Garage-backed artifacts
returned HTTP `200`. The reports recorded post-bootstrap build-window growth
of 6,311,936 and 692,224 allocated bytes and a second-run peak of 375,136,256
bytes, approximately 358 MiB. Both runs removed the dedicated Colima profile
and left no active owner lock. This is local cross-run persistence and cleanup
evidence, not total growth from empty state, production, AWS runtime, off-host
backup, high-availability, or cloud durability evidence.

On 2026-08-05, the foreground operator UI restored three builds, accepted the
fresh runtime administrator password returned only by the explicit helper, and
returned a signed Garage artifact through the local port forward with HTTP
`200`. Ctrl-C ended the session with status `130`, removed kind and the Colima
profile, released both ownership locks, removed the runtime kubeconfig, and
left the UI and password helper unreachable. No presubmit build was triggered.
This is local operator-path evidence, not public, production, AWS, off-host, or
high-availability evidence.

## Required free boundary

The local implementations are required curriculum, not optional mentions:

- Real kind Kubernetes, Strimzi Kafka, Garage, Prometheus, and Grafana.
- AWS SAM local runtime for Lambda-compatible handlers.
- DynamoDB Local through the real AWS SDK API surface.
- Real application containers plus validated ECS task and IAM contracts.
- Real disposable Jenkins runtime backed by explicitly bounded host-local
  retained state.
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
- Runtime resources are disposable and clean up on success and failure, except
  for intentionally retained state with a documented bound, owner, and purge
  path.
- No brokerage credentials, account identifiers, personal exports, cloud
  access keys, state, or secrets enter Git.
- Retained Jenkins and Garage credentials remain host-local; that persistence
  is not production security, backup, or durability evidence.
- The result includes a verification record and honest unsupported boundaries.
