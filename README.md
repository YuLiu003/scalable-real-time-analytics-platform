# Cloud-Native Investment Analytics Platform

This repository is a free, production-like learning platform for Kubernetes,
event-driven systems, cloud infrastructure, and long-term investment analytics.
It processes deterministic public fixtures or an optional private stock/ETF
watchlist, builds portfolio products, normalizes external cash flows, and
exposes contribution projections through a Go API and dashboard.

The workload is intentionally useful without pretending a personal portfolio
needs hyperscale infrastructure. Synthetic traffic and fault injection provide
the load and failure conditions needed to practice Kafka, Kubernetes, recovery,
observability, and cloud-platform engineering.

## Current status

- The complete local platform runs on disposable `kind` clusters.
- Kafka uses Strimzi in KRaft mode.
- KEDA scales an isolated archive consumer group from Kafka lag.
- A separate fixed-worker benchmark runs repeatable 10K/50K/100K synthetic
  trials and rejects incomplete, duplicated, reordered, or unmeasured runs.
- An opt-in Alpaca WebSocket adapter uses runtime-only credentials and
  watchlists, historical gap backfill, and stable Kafka event identities.
- An offline JSON/CSV importer creates a private normalized cash-flow ledger
  without confusing deposits or withdrawals with return.
- Garage provides the local S3-compatible object-storage contract.
- Python and DuckDB build deterministic Parquet analytics products.
- The Go portfolio API serves holdings and contribution projections.
- Prometheus and Grafana provide cluster and workload observability.
- OpenTofu validates the AWS EKS architecture without requiring a paid apply.
- Jenkins runs repository-owned PS0, PS1, and PS2 gates on isolated Kubernetes
  agents and publishes exact-commit evidence to GitHub.
- A separate Codex plugin routes focused code reviews and reports descriptive
  quality-per-token cohorts without including prompts, diffs, or paths in its
  records.
- Measured application scopes require 100% statement coverage; Python analytics
  also requires 100% branch coverage.

This is verified local and static cloud evidence, not a claim that the project
operates a continuously available production service or a live AWS account.

## Architecture

```text
Synthetic producer -----------\
                               > Strimzi Kafka -> raw archiver -> Garage / S3 bronze
Private Alpaca adapter -------/                              |
                                                             | synthetic source + demo holdings
                                                             v
                                                  Python + DuckDB analytics
                                                             |
                                                             v
                                                  Parquet portfolio products
                                                             |
                                                             v
                                                Go portfolio API + dashboard

Synthetic scale producer --> isolated Kafka scale topic
                                      |
                                      v
                            KEDA-scaled archivers
                                      |
                                      v
                             aggregate evidence
```

The default analytics Job intentionally selects only the committed synthetic
source and demo holdings. Private feed records stop at the bronze boundary
until an authenticated private-holdings and retention contract exists.

The acceptance path injects ambiguous producer acknowledgements, consumer
crashes, duplicate delivery, replay, dependency loss, and readiness failures.
Immutable object writes and deterministic input identities prevent a replay
from silently changing a result.

Capacity evidence is deliberately separate. It removes the artificial archive
delay, fixes three consumers to the three-partition concurrency ceiling, and
reports broker-acknowledgement throughput separately from durable
Kafka-to-object-storage throughput. The repository does not contain a committed
full-matrix result yet, so no throughput number is claimed here.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `services/market-pipeline/` | Market event contracts, producer, Kafka consumer, and immutable archive |
| `services/portfolio-analytics/` | Portfolio model, DuckDB transformations, Parquet products, and queries |
| `services/portfolio-api/` | Holdings, results, contribution projections, and dashboard |
| `platform/gitops/` | Reusable local and AWS Kubernetes desired state |
| `platform/local/` | Disposable kind lifecycle, verification, diagnostics, and fault injection |
| `platform/jenkins/` | Jenkins controller, isolated agents, and exact-commit presubmit |
| `infra/opentofu/aws/` | AWS network, EKS, identity, registry, storage, encryption, and cost controls |
| `scripts/ci/` | Repository-owned PS0, PS1, and PS2 implementation |
| `tools/codex-plugins/agent-review-optimizer/` | Privacy-safe Codex review routing and efficiency analysis |
| `tools/codex-plugins/cloud-platform-engineering/` | Project-specific cloud-platform review and research tools |

## Run locally

Run commands from the repository root. Host prerequisites and pinned versions
are documented in the
[`local platform runbook`](platform/local/README.md) and
[`Jenkins runbook`](platform/jenkins/README.md).

Install the Python test dependencies once:

```bash
python3 -m venv .venv
.venv/bin/python -m pip install \
  --requirement services/portfolio-analytics/requirements-dev.txt \
  --requirement tools/codex-plugins/agent-review-optimizer/requirements-dev.txt \
  --requirement tools/codex-plugins/cloud-platform-engineering/mcp/requirements-dev.txt
```

Run the fast source and correctness gates:

```bash
make presubmit-ps0
PYTHON_BIN="$PWD/.venv/bin/python" make presubmit-ps1
```

Run the complete platform in a disposable Colima VM:

```bash
make -C platform/local e2e-ephemeral
```

The command isolates Docker and Kubernetes contexts, then deletes the VM and
all container data on success or failure. It also runs the Kafka scale,
autoscaling, replay, and recovery acceptance. Use the persistent development
workflow in the local-platform runbook only when you need to inspect a running
cluster.

Run the repeated capacity matrix in its own disposable Colima VM:

```bash
make -C platform/local e2e-capacity-ephemeral
```

This runs the scale/recovery acceptance once, then five zero-delay trials at
10K, 50K, and 100K events. Generated reports remain under the ignored
`artifacts/kafka-capacity/` path while the owned VM and all container data are
deleted. See the capacity contract before presenting any result.

For private product inputs, follow the
[`cash-flow ledger contract`](docs/features/cloud-native-investment-platform/personal-portfolio-ledger-contract.md)
or the runbook's
[`optional live-feed procedure`](platform/local/README.md#optional-private-live-market-feed).

Run the production-like Jenkins path:

```bash
make -C platform/jenkins e2e-ephemeral
```

That command also uses a dedicated disposable Colima VM and deletes its
container data when it finishes.

## Cloud learning boundary

| Local contract | AWS responsibility | GCP equivalent | Azure equivalent |
| --- | --- | --- | --- |
| kind Kubernetes | EKS | GKE | AKS |
| Strimzi Kafka | MSK or Strimzi on EKS | Managed Service for Apache Kafka or Strimzi | Event Hubs Kafka endpoint or Strimzi |
| Garage S3 API | S3 | Cloud Storage | Blob Storage |
| Kubernetes service accounts | EKS Pod Identity | Workload Identity Federation | Workload Identity |
| OpenTofu tests | AWS provider and APIs | Google provider and APIs | AzureRM provider and APIs |

Local emulators teach workload contracts but do not reproduce cloud control
planes, IAM propagation, managed-service failure modes, billing, or support
operations. A paid apply is outside the required definition of done.

## Documentation

- [Architecture and feature scope](docs/features/cloud-native-investment-platform/README.md)
- [Roadmap](docs/features/cloud-native-investment-platform/roadmap-v2.md)
- [Quality gates](docs/features/cloud-native-investment-platform/quality-gates.md)
- [Kafka scale lab](docs/features/cloud-native-investment-platform/kafka-scale-lab-contract.md)
- [Kafka capacity benchmark](docs/features/cloud-native-investment-platform/kafka-capacity-benchmark-contract.md)
- [Private market feed](docs/features/cloud-native-investment-platform/private-market-feed-contract.md)
- [Personal cash-flow ledger](docs/features/cloud-native-investment-platform/personal-portfolio-ledger-contract.md)
- [Local Kubernetes runbook](platform/local/README.md)
- [Jenkins platform](platform/jenkins/README.md)
- [AWS OpenTofu lab](infra/opentofu/aws/README.md)
- [Presubmit contract](docs/engineering/presubmit-gates.md)
- [Security boundaries](docs/SECURITY.md)

## License

See [LICENSE](LICENSE).
