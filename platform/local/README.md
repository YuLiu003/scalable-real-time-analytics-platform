# Local Kubernetes Platform Baseline

This directory implements Slice 1 and the local runtime for Slices 2 and 3 of the
[Cloud-Native Investment Analytics Platform proposal](../../docs/features/cloud-native-investment-platform/README.md).
It creates a disposable, multi-node Kubernetes environment with explicit
namespace ownership, local resource guardrails, Prometheus, and Grafana.

Runtime proof is recorded in the
[`Slice 1 verification`](VERIFICATION.md),
[`Slice 2 verification`](SLICE2-VERIFICATION.md), and
[`Slice 3 verification`](SLICE3-VERIFICATION.md) records.

Slice 2 adds a Strimzi-managed Kafka broker, Garage object storage, a synthetic
market producer, and a raw-event archiver. Its exact event and failure semantics
are defined in the
[`Slice 2 contract`](../../docs/features/cloud-native-investment-platform/slice-2-event-contract.md).

Slice 3 adds a deterministic DuckDB Job, silver/gold Parquet products, and a
stateless Go API with an embedded portfolio-allocation dashboard. Its exact
calculation, publication, readiness, and replay semantics are defined in the
[`Slice 3 contract`](../../docs/features/cloud-native-investment-platform/slice-3-analytics-contract.md).

The current baseline replaces the retired Minikube, raw-manifest, and incomplete
Helm paths. Supported local workflows use kind through the targets documented
here.

## Local quality gate

Install the pinned Python development dependencies in an isolated environment,
then run the same coverage and race-test gate used by CI:

```bash
python3 -m venv .venv
.venv/bin/python -m pip install \
  --requirement services/portfolio-analytics/requirements-dev.txt
PYTHON_BIN=.venv/bin/python make -C platform/local quality
```

The command fails below 100% measured application coverage. Its exact scope,
the separate kind end-to-end test, and the continuous-delivery boundary are
documented in the
[quality-gates contract](../../docs/features/cloud-native-investment-platform/quality-gates.md).

## What this proves

- A version-pinned Kubernetes cluster can be created and removed repeatably.
- Platform, stateful-data, and application namespaces have explicit ownership.
- Local workloads receive default resource requests and limits.
- Data and application namespaces have bounded resource budgets.
- Prometheus and Grafana are installed from a pinned chart version.
- Verification and diagnostics are scripted and exclude Kubernetes Secrets.
- Retained bronze objects can deterministically reconstruct silver/gold products
  without rerunning a producer or reading Kafka.
- A stateless API can expose a user-visible result while separating process
  liveness from result-dependent readiness.

It does not prove cloud-zone availability, durable monitoring retention,
production security, disaster recovery, historical portfolio performance, or
concurrent analytical publication.

## Pinned baseline

Versions are recorded in [`versions.lock`](versions.lock). The Kubernetes node
image is pinned by digest rather than tag. Version updates require a reviewed
change and a fresh bootstrap/verify/destroy cycle.

The initial pins come from the official
[kind v0.31.0 release](https://github.com/kubernetes-sigs/kind/releases/tag/v0.31.0)
and the published
[kube-prometheus-stack chart](https://artifacthub.io/packages/helm/prometheus-community/kube-prometheus-stack).

The cluster uses one control-plane node and two worker nodes. Worker labels model
two local failure domains for scheduling exercises, but they are containers on
one physical machine and are not independent availability zones.

## Prerequisites

- Docker CLI
- kind at the exact version in `versions.lock`
- kubectl compatible with the pinned Kubernetes version
- Helm 3
- Go `1.25.12`
- Make
- OpenSSL
- Python 3 for the local quality gate

The recommended disposable workflow additionally requires macOS and Colima. It
does not require a running Docker daemon before invocation because it creates
its own Colima VM. The default VM reserves 4 CPUs, 8 GiB of memory, and 30 GiB
of disk.

Persistent development mode requires a reachable Docker-compatible daemon.
Approximately 6 GiB of free memory is needed for the three kind nodes and
monitoring stack.

The full platform is intentionally heavyweight: kind stores each node's
containerd data in a Docker volume, and a persistent Colima VM retains that
data until explicitly deleted. On a developer Mac, prefer the ephemeral path
below instead of leaving the cluster running between sessions.

The bootstrap does not install host tools automatically. For persistent mode,
start the Docker-compatible daemon and run:

```bash
make -C platform/local preflight
```

The disposable command checks its host tools before creating the VM and runs
the remaining preflight checks after its isolated daemon starts.

If preflight reports a missing Docker credential helper after Docker Desktop was
removed, do not weaken or overwrite the global Docker config just for this lab.
For the named Colima profile used in the verification record, isolate the
session instead:

```bash
export DOCKER_HOST="$(docker context inspect colima-investment-platform \
  --format '{{.Endpoints.docker.Host}}')"
export DOCKER_CONFIG="${TMPDIR:-/tmp}/investment-platform-docker-config"
mkdir -p "${DOCKER_CONFIG}"
```

Resolve `DOCKER_HOST` before changing `DOCKER_CONFIG`; Docker context metadata
lives in the original config directory. These exports affect only the current
shell and leave the user's global credential configuration unchanged.

## Create and verify

### Recommended: disposable full-platform run

On macOS with Colima, run the complete platform in an isolated runtime that is
deleted on both success and failure:

```bash
make -C platform/local e2e-ephemeral
```

This command creates the reserved `investment-platform-ephemeral` Colima
profile, bootstraps and verifies all three slices, writes failure diagnostics
to the host or CI log before cleanup, deletes the kind cluster, and finally
runs `colima delete --force --data`. It refuses to reuse or delete a
pre-existing profile. Temporary Docker and Kubernetes configuration prevents
the run from changing the caller's active contexts or retaining generated
context records.
The tradeoff is that images and charts must be downloaded again on the next
run; CI is therefore the preferred place for frequent full end-to-end tests.

The following variables can reduce or increase the temporary VM boundary:

```bash
EPHEMERAL_COLIMA_CPUS=4 \
EPHEMERAL_COLIMA_MEMORY_GIB=8 \
EPHEMERAL_COLIMA_DISK_GIB=30 \
  make -C platform/local e2e-ephemeral
```

### Persistent development mode

Optionally supply the Grafana password without writing it to a file:

```bash
export GRAFANA_ADMIN_PASSWORD='choose-a-local-only-password'
```

Create the cluster, apply desired state, install monitoring, and verify it:

```bash
make -C platform/local bootstrap
```

Run verification again at any time:

```bash
make -C platform/local verify
```

## Producer-to-storage data path

With the Slice 1 cluster running, build and exercise the complete Slice 2 data
path, including producer- and consumer-crash tests:

```bash
make -C platform/local bootstrap-data-path
```

Re-run the terminal-state assertions without repeating completed failure
injections:

```bash
make -C platform/local verify-data-path
```

Collect Slice 2 diagnostics without Secret objects or values:

```bash
make -C platform/local diagnose-data-path
```

Delete only the Slice 2 workloads and durable data:

```bash
CONFIRM_DESTROY_DATA_PATH=market-data-path \
  make -C platform/local destroy-data-path
```

This local topology has one Kafka broker and one Garage replica. It tests API,
identity, persistence, and failure boundaries but does not claim broker or
object-store availability.

## Analytics and portfolio dashboard

With the Slice 2 data path running, build silver/gold products, deploy the API,
query the Parquet products directly, and prove exact reconstruction from bronze:

```bash
make -C platform/local bootstrap-analytics
```

Re-run settled-state assertions without deleting data or repeating completed
Jobs:

```bash
make -C platform/local verify-analytics
```

Collect Slice 3 diagnostics without Secrets, bronze payloads, or portfolio
result contents:

```bash
make -C platform/local diagnose-analytics
```

Expose the dashboard at <http://localhost:8080>:

```bash
make -C platform/local portfolio-dashboard
```

If port 8080 is occupied, choose another local port:

```bash
make -C platform/local portfolio-dashboard PORTFOLIO_DASHBOARD_PORT=18080
```

The dashboard labels its source-controlled quantities and prices as synthetic.
It models QQQ and QQQM as ETFs using market prices, FSELX as a mutual fund using
daily NAV, and the S&P 500 as a benchmark index level rather than a holding.

Delete only the Slice 3 workloads and derived silver/gold products. Bronze,
Kafka, Garage, and all Slice 2 resources remain intact:

```bash
CONFIRM_DESTROY_ANALYTICS=portfolio-analytics \
  make -C platform/local destroy-analytics
```

If no Grafana password is supplied, bootstrap generates one and stores it only
in the Kubernetes Secret. Re-running bootstrap reuses that Secret rather than
silently rotating the credential. Retrieve the password explicitly when needed:

```bash
kubectl --context kind-investment-platform \
  --namespace platform-observability \
  get secret grafana-admin-credentials \
  --output=jsonpath='{.data.admin-password}' | base64 --decode
```

## Access monitoring

Run one command per terminal:

```bash
make -C platform/local grafana
make -C platform/local prometheus
```

- Grafana: <http://localhost:3000>
- Prometheus: <http://localhost:9090>

Port-forwarding is intentionally explicit. A Gateway and cloud load-balancer
boundary will be introduced in a later slice.

## Diagnose

```bash
make -C platform/local diagnose
```

The command writes cluster state, events, Helm status, and kind logs to a
timestamped directory under `${TMPDIR:-/tmp}`. It does not collect Secret
objects or values.

## Destroy

Interactive deletion requires typing the cluster name:

```bash
make -C platform/local destroy
```

For automation:

```bash
CONFIRM_DESTROY=investment-platform make -C platform/local destroy
```

The cluster is disposable. Monitoring data and generated local credentials are
deleted with it.

Deleting only the kind cluster removes its node containers and volumes but
retains the Colima VM and cached images. To reclaim the entire disk of the
dedicated `investment-platform` profile, use the separately confirmed command:

```bash
CONFIRM_RUNTIME_CLEANUP=investment-platform \
  make -C platform/local reclaim-runtime
```

This deletes all images, volumes, settings, and VM disk in that dedicated
profile. It deliberately refuses to target profiles whose names do not start
with `investment-platform`.

## Namespace model

| Namespace | Owner | Responsibility |
| --- | --- | --- |
| `platform-observability` | Platform | Prometheus, Grafana, and future telemetry controllers |
| `analytics-data` | Data platform | Kafka, object storage, and other explicitly stateful services |
| `analytics-apps` | Application | Producers, processors, APIs, and dashboards |

Pod Security Admission is enforced at `restricted` for application workloads
and `baseline` for stateful-data workloads. The dedicated observability
namespace enforces `privileged` because node exporter requires host namespaces,
host ports, and read-only hostPath mounts. It remains audited and warned against
`restricted`, keeping the exception visible and isolated from application and
data workloads.

Default-deny NetworkPolicies are deliberately deferred until required ingress,
egress, DNS, scraping, and control-plane flows are enumerated and accompanied by
a recovery procedure.

## Lifecycle boundaries

1. `kind` creates and deletes the local cluster.
2. Kustomize holds cluster desired state under `platform/gitops`.
3. Helm installs pinned third-party platform add-ons.
4. Argo CD will reconcile desired state in a later slice.
5. Application charts remain independent of cluster provisioning and add-on
   installation.

This separation mirrors the eventual cloud boundary: OpenTofu provisions cloud
resources and managed Kubernetes, platform delivery installs add-ons, and GitOps
reconciles application workloads.
