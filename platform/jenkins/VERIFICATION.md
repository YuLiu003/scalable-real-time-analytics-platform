# Jenkins verification record

## Result

The production-like local Jenkins path completed `PS0`, `PS1`, and `PS2` on
2026-07-29. The proof run started from a clean disposable Colima VM at 13:05
America/Los_Angeles, passed at 13:21, and deleted the VM and all container data.
Commit `b7c1f8f1bc66bf53dba0adfa432e9bd834b7b355` is the retained initial runtime
proof. It predates the dedicated Garage artifact-retention feature and does not
prove that feature. The `jenkins / presubmit` status on each pull-request head
is the authoritative evidence for later revisions only after that revision has
actually completed the disposable run.

## Observed evidence

- Jenkins `2.568.1` was installed from chart `5.9.45` on a two-node outer kind
  cluster.
- Runtime checks confirmed zero controller executors, JCasC-owned job
  configuration, namespace-scoped RBAC, persistent storage, and network policy.
- `PS0` and `PS1` ran on separate non-root, tokenless Kubernetes agents.
- `PS1` enforced race tests plus 100% application, plugin, and
  presubmit-runner coverage. The current gate also measures the added
  Jenkins-reporter Python code at 100%; its per-head status proves that revision.
- `PS2` passed two OpenTofu foundation tests and five three-zone AWS platform
  tests before creating the nested functional cluster.
- The single-node nested cluster reached Ready, followed by Prometheus,
  Grafana, Strimzi, Kafka in KRaft mode, Garage object storage, and the
  investment analytics services.
- Producer failure, retry, consumer-crash, archive, query, and replay checks
  completed. Intentionally failed fault-injection jobs were evaluated by the
  verifier rather than treated as healthy workloads.
- The integration agent used the architecture-specific, digest-pinned DinD
  source preloaded under a local runtime tag.
- The success path and a later `TERM` interruption both deleted the dedicated
  Colima profile and its data.

## Retention proof status

The current Garage-retention implementation is contract-tested, but its
disposable and cross-run proof is pending. Completion requires one exact-commit
run that proves the two PVCs, three-day/20-build policy, dedicated bucket
quota/lifecycle, artifact redirect and unauthenticated presigned readback,
sanitized storage report, retained-state boundary, owner-token fencing, and VM
cleanup. A second clean Colima recreation using the same host directory must
prove that build history and artifacts restore across runs. The proof must also
show that the Pipeline definition came from the explicitly trusted branch while
source stages checked out the requested exact commit. No storage measurements
are claimed here until those runs complete.

Reproduce the proof with:

```bash
make -C platform/jenkins quality
make -C platform/jenkins e2e-ephemeral
platform/jenkins/scripts/report-github-status.py \
  --pr "$PR_NUMBER" \
  --log /absolute/path/to/retained-operator.log
```

The trusted trigger supplies the exact 40-character PR head. Every Jenkins
stage rejects a different checkout. The host-side reporter verifies the open
PR head, ordered success and cleanup markers, posts an evidence digest, and
then publishes `jenkins / presubmit`.

## Boundaries

- This proves a disposable local control plane and real Kubernetes agents; it
  is not evidence of operating a continuously available production Jenkins
  service.
- The nested CI cluster is deliberately one node. The normal local topology is
  three nodes, and OpenTofu independently tests a three-zone AWS design.
- Privileged DinD exists only in the disposable integration agent. Production
  needs a separate disposable node pool or account for that workload.
- The baseline proof deleted the local controller and console. The current
  implementation instead retains controller history, console logs, and Garage
  artifacts on one developer host; it does not publish them in the GitHub
  evidence comment and is not backup, HA, or cloud-object-storage evidence.
- The lab reporter uses an authenticated trusted operator after Jenkins exits.
  Production needs GitHub Branch Source and a GitHub App held by the controller
  plugin, never by PR-controlled Pipeline code.
- No paid AWS apply occurred. AWS results are configuration and OpenTofu test
  evidence, not managed-cloud runtime evidence.
