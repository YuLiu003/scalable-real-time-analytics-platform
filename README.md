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
- A private local workflow joins that feed to runtime-only stock/ETF holdings,
  five-minute analytics refreshes, and a bearer-protected dashboard.
- An offline JSON/CSV importer creates a private normalized cash-flow ledger
  without confusing deposits or withdrawals with return.
- Garage provides the application's local S3-compatible object-storage
  contract. Jenkins uses a separate Garage instance and bucket so CI artifacts
  never share market-data credentials or storage.
- Python and DuckDB build deterministic Parquet analytics products.
- The Go portfolio API serves holdings and contribution projections.
- Prometheus and Grafana provide cluster and workload observability.
- OpenTofu validates the AWS EKS architecture without requiring a paid apply.
- Jenkins runs repository-owned PS0, PS1, and PS2 gates on isolated Kubernetes
  agents, stores verification artifacts in a dedicated Garage bucket, and
  publishes exact-commit evidence to GitHub. Native history is limited to three
  days and 20 builds; its Garage bucket is limited to 1 GiB and 10,000 objects.
  A trusted Pipeline definition orchestrates the exact source commit under test.
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
                                                             +-> synthetic source + demo holdings
                                                             |       -> public analytics and API
                                                             |
                                                             +-> private source + runtime-only holdings
                                                                     -> private analytics and protected API

Synthetic scale producer --> isolated Kafka scale topic
                                      |
                                      v
                            KEDA-scaled archivers
                                      |
                                      v
                             aggregate evidence
```

The default analytics Job intentionally selects only the committed synthetic
source and demo holdings. The opt-in private workflow uses separate Secrets,
analytics/API workloads, and a fixed private tenant; it never publishes a real
watchlist or holding to Git or CI artifacts.

The acceptance path injects ambiguous producer acknowledgements, consumer
crashes, duplicate delivery, replay, dependency loss, and readiness failures.
Immutable object writes and deterministic input identities prevent a replay
from silently changing a result.

Capacity evidence is deliberately separate. It removes the artificial archive
delay, fixes three consumers to the three-partition concurrency ceiling, and
reports broker-acknowledgement throughput separately from durable
Kafka-to-object-storage throughput. The roadmap preserves the committed
aggregate result and its exact local hardware and topology boundary.

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
deleted. Stop any other running Colima profile first; the benchmark checks this
at startup and never mutates the other profile. See the capacity contract before
presenting any result.

Run the production-like Jenkins path:

```bash
make -C platform/jenkins e2e-ephemeral
```

That command also uses a dedicated disposable Colima VM and automatically
deletes the profile and container data on success, failure, or handled
interruption. If deletion itself fails, the command fails and preserves its
ownership fences instead of allowing another writer. It retains only bounded
Jenkins metadata, Garage artifacts, credentials, and privacy-safe reports under
`~/.local/share/investment-platform/jenkins`: builds are limited to three days
and 20 records, and the artifact bucket is limited to 1 GiB. See the Jenkins
runbook for retention timing, disk measurements, and the local-only boundary.

## Use the application

The `e2e-*` commands above are verification workflows: they delete their live
runtime when finished and do not leave a dashboard running. The Jenkins path's
explicitly mounted retained-state directory is the only exception. Use one of
the persistent workflows below when you want to interact with the application.

### Explore the fictional demo

Start the cluster, event pipeline, analytics, and dashboard in order:

```bash
make -C platform/local bootstrap
make -C platform/local bootstrap-data-path
make -C platform/local bootstrap-analytics
make -C platform/local portfolio-dashboard
```

Keep the last command running and open <http://127.0.0.1:8080>. This demo needs
no credentials or access token and labels all holdings and prices as synthetic.
Press `Ctrl-C` to close the dashboard tunnel; the cluster continues running.

If port 8080 is occupied, use another loopback port:

```bash
make -C platform/local portfolio-dashboard PORTFOLIO_DASHBOARD_PORT=18080
```

Then open <http://127.0.0.1:18080>.

### Use a private live stock/ETF portfolio

Create the three absolute-path, mode-`0600` input files described in the
[`private portfolio workflow`](docs/features/cloud-native-investment-platform/private-portfolio-workflow.md).
Those files hold the Alpaca credentials and selected instruments, portfolio
quantities, and dashboard token; keep them outside this repository. Start the
private workflow with:

```bash
PRIVATE_FEED_ENV_FILE=/absolute/path/alpaca-market-feed.env \
PRIVATE_HOLDINGS_FILE=/absolute/path/portfolio.json \
PRIVATE_ACCESS_TOKEN_FILE=/absolute/path/portfolio.token \
  make -C platform/local bootstrap-private-portfolio

make -C platform/local private-portfolio-dashboard
```

Keep the last command running, open <http://127.0.0.1:8080>, and enter the token
from `portfolio.token`. The token remains only in the page's memory. Instruments
come from the private input files rather than a source-controlled portfolio.

### Read and model the portfolio

The dashboard shows the current total value, selected benchmark and observation
time, allocation by market value, and each holding's quantity, price, value,
and allocation percentage.

In **Long-term contribution projection**, enter:

- the starting value, contribution amount, and monthly or biweekly cadence;
- the number of years and assumed annual return;
- the return range, inflation rate, and annual expense ratio.

The application calculates conservative, base, and optimistic scenarios. Each
scenario reports the ending balance, inflation-adjusted balance, total
contributions, investment growth, and estimated fee drag. Contributions are
modeled at the end of each period. These are hypothetical calculations, not
forecasts, recommendations, or trade execution.

### Refresh, change inputs, and clean up

Private analytics publishes a new snapshot every five minutes. The browser does
not poll automatically, so reload the page to display a newer snapshot. Rerun
`bootstrap-private-portfolio` with the three input paths to change credentials,
instruments, quantities, or the access token.

Stop the private workloads and remove their runtime Secrets and checkpoint:

```bash
CONFIRM_DESTROY_PRIVATE_PORTFOLIO=private-portfolio \
  make -C platform/local destroy-private-portfolio
```

Kafka and Garage retain shared market and derived data after that command. Use
the explicit all-data purge documented in the private workflow when those data
must also be removed. To delete the entire project-owned local cluster,
containers, volumes, and Colima VM disk, run:

```bash
CONFIRM_RUNTIME_CLEANUP=investment-platform \
  make -C platform/local reclaim-runtime
```

The credential-free PS2 acceptance proves the private wiring with fictional
records; it does not contact Alpaca or prove a live provider subscription.
Transaction-grounded performance based on the separate
[`cash-flow ledger contract`](docs/features/cloud-native-investment-platform/personal-portfolio-ledger-contract.md)
remains future work.

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
operations. In particular, Jenkins' single-host Garage and hostPath persistence
do not prove cloud object-storage durability, TLS, workload identity,
backup/restore, or high availability. A paid apply is outside the required
definition of done.

## Documentation

- [Architecture and feature scope](docs/features/cloud-native-investment-platform/README.md)
- [Roadmap](docs/features/cloud-native-investment-platform/roadmap-v2.md)
- [Quality gates](docs/features/cloud-native-investment-platform/quality-gates.md)
- [Kafka scale lab](docs/features/cloud-native-investment-platform/kafka-scale-lab-contract.md)
- [Kafka capacity benchmark](docs/features/cloud-native-investment-platform/kafka-capacity-benchmark-contract.md)
- [Private market feed](docs/features/cloud-native-investment-platform/private-market-feed-contract.md)
- [Private portfolio workflow](docs/features/cloud-native-investment-platform/private-portfolio-workflow.md)
- [Personal cash-flow ledger](docs/features/cloud-native-investment-platform/personal-portfolio-ledger-contract.md)
- [Local Kubernetes runbook](platform/local/README.md)
- [Jenkins platform](platform/jenkins/README.md)
- [AWS OpenTofu lab](infra/opentofu/aws/README.md)
- [Presubmit contract](docs/engineering/presubmit-gates.md)
- [Security boundaries](docs/SECURITY.md)

## License

See [LICENSE](LICENSE).
