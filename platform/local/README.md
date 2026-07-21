# Local Kubernetes Platform Baseline

This directory implements Slice 1 of the
[Cloud-Native Investment Analytics Platform proposal](../../docs/features/cloud-native-investment-platform/README.md).
It creates a disposable, multi-node Kubernetes environment with explicit
namespace ownership, local resource guardrails, Prometheus, and Grafana.

Current test results and remaining runtime proof are recorded in
[`VERIFICATION.md`](VERIFICATION.md).

The baseline is intentionally separate from the legacy Minikube, raw-manifest,
and incomplete Helm deployment paths. Those paths remain untouched until a
later migration proposal defines their disposition.

## What this proves

- A version-pinned Kubernetes cluster can be created and removed repeatably.
- Platform, stateful-data, and application namespaces have explicit ownership.
- Local workloads receive default resource requests and limits.
- Data and application namespaces have bounded resource budgets.
- Prometheus and Grafana are installed from a pinned chart version.
- Verification and diagnostics are scripted and exclude Kubernetes Secrets.

It does not prove cloud-zone availability, durable monitoring retention,
production security, disaster recovery, or application correctness.

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

- Docker with a reachable daemon
- kind at the exact version in `versions.lock`
- kubectl compatible with the pinned Kubernetes version
- Helm 3
- OpenSSL
- Approximately 6 GiB of free memory for the three nodes and monitoring stack

The bootstrap does not install host tools automatically. Run the preflight to
receive actionable missing-tool or version errors:

```bash
make -C platform/local preflight
```

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
