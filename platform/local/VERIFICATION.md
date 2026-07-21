# Slice 1 Verification Record

| Field | Value |
| --- | --- |
| Date | 2026-07-21 |
| Branch | `feature/local-platform-baseline` |
| Status | Runtime lifecycle passed; Slice 1 complete |
| Final state | Rebuilt cluster running on context `kind-investment-platform` |

## Test environment

| Component | Observed value |
| --- | --- |
| Host | Apple Silicon macOS, 16 GiB RAM |
| Local runtime | Colima `0.10.3`, 4 CPUs, 8 GiB RAM, 60 GiB disk |
| Docker | CLI `29.6.2`, server `29.5.2` |
| kind | `v0.31.0` (`darwin/arm64`, release checksum verified) |
| Kubernetes | `v1.33.7`, node image pinned by digest |
| kubectl | `v1.32.3` |
| Helm | `v3.21.3` |
| Monitoring | `kube-prometheus-stack` `87.12.2` |

The existing `minikube` kubeconfig entry was preserved. A named Colima profile,
`investment-platform`, isolated the Docker runtime from other local environments.

## Static verification

The following checks passed:

```bash
bash -n platform/local/scripts/*.sh
find platform/local platform/gitops -type f \
  \( -name '*.yaml' -o -name '*.yml' \) \
  -print0 | xargs -0 ruby -e \
  'require "yaml"; ARGV.each { |path| YAML.load_stream(File.read(path)) }'
kubectl kustomize platform/gitops/clusters/local
git diff --check
```

Observed results:

- All five lifecycle scripts passed Bash syntax validation and retained their
  executable bits.
- All seven YAML files parsed successfully.
- Kustomize rendered eight resources: three Namespaces, three LimitRanges, and
  two ResourceQuotas.
- The rendered namespace policies match the verified runtime policies.

## Failure evidence and corrections

The lifecycle was not treated as successful until two failures were reproduced,
diagnosed, and corrected:

1. The host Docker config referenced a removed `docker-credential-desktop`
   helper. The original preflight only checked daemon reachability, so the first
   image pull failed. Preflight now detects a configured-but-missing helper and
   exits before mutation with an actionable error. Runtime testing used a
   temporary, credential-free `DOCKER_CONFIG` pointed at the Colima socket; the
   user's global Docker config was not changed.
2. Pod Security Admission at `baseline` rejected node exporter because it needs
   host namespaces, host ports, and read-only hostPath mounts. Evidence was
   captured at
   `${TMPDIR}/investment-platform-diagnostics-20260721T071617Z`. The exception
   is now scoped to the dedicated `platform-observability` namespace, which
   enforces `privileged` while continuing to audit and warn against `restricted`.
   Data remains at `baseline`; applications remain at `restricted`.

Grafana credential creation was also made idempotent and secret-safe: bootstrap
reuses an existing Secret and never prints the generated password.

## Successful lifecycle evidence

The required sequence passed:

```bash
make -C platform/local preflight
make -C platform/local bootstrap
make -C platform/local verify
make -C platform/local diagnose
CONFIRM_DESTROY=investment-platform make -C platform/local destroy
make -C platform/local bootstrap
make -C platform/local verify
```

On this host, the commands used an isolated `DOCKER_CONFIG` because of the stale
global credential-helper setting described above.

Observed results:

- One control-plane and two worker nodes reached `Ready` on Kubernetes `v1.33.7`.
- All three namespace ownership labels and Pod Security tiers matched desired
  state.
- All three LimitRanges and both ResourceQuotas were present.
- Helm release `monitoring` reached `deployed` with seven healthy pods: Grafana,
  Prometheus, the operator, kube-state-metrics, and one node exporter per node.
- Grafana `/api/health` returned HTTP 200 with database status `ok`.
- Prometheus `/-/ready` returned HTTP 200 and `Prometheus Server is Ready.`
- `verify.sh` now exercises both HTTP endpoints through the Kubernetes service
  proxy on every run.
- The post-success diagnostic bundle was written to
  `${TMPDIR}/investment-platform-diagnostics-20260721T073218Z`; Secret objects
  and values were intentionally excluded.
- Confirmed teardown removed all three node containers and the
  `kind-investment-platform` kubeconfig context; `kind get clusters` reported no
  clusters.
- A clean rebuild from desired state completed in `80.93s` with cached local
  images, and the standalone verification passed again.

Resource snapshot after the clean rebuild:

| Node container | CPU | Memory |
| --- | ---: | ---: |
| `investment-platform-control-plane` | 17.01% | 967.4 MiB |
| `investment-platform-worker` | 3.85% | 594.3 MiB |
| `investment-platform-worker2` | 14.04% | 379.9 MiB |

This is a point-in-time local snapshot, not a capacity benchmark.

## Conclusion

Slice 1 proves a reproducible local Kubernetes control-plane baseline with
version pins, namespace boundaries, resource guardrails, monitoring, health
verification, diagnostics, destructive teardown, and clean recovery. It does
not prove multi-zone availability, durable monitoring retention, production
security, cloud portability, disaster recovery, or application correctness.
