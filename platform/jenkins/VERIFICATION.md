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

The dedicated Garage retention path completed two consecutive clean local runs
on 2026-08-05 at commit
`6197e8a7acdb5e47dbd7c41616b3ef679669cb2c`. Each run recreated Colima and
kind, passed `PS0`, `PS1`, and `PS2`, and removed the disposable runtime.

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
- Both retention runs exposed 34 Jenkins artifact entries backed by the
  dedicated Garage bucket and read back a current-build artifact successfully.
- During the second fresh runtime, Jenkins restored build `#1` and its Garage
  artifact returned HTTP `200` before build `#2` completed.
- Cleanup after each retention run left no Jenkins Colima profile or active
  owner lock.

## Retention proof

The local cross-run proof is complete. It covered both retained PVCs, the
three-day/20-build Jenkins policy, the three-day Garage lifecycle, the 1 GiB
and 10,000-object bucket limits, least-privilege runtime credentials, artifact
redirect and readback, sanitized reports, owner-token fencing, exact source
commit checks, and VM cleanup.

The monitored post-bootstrap build windows added 6,311,936 allocated bytes in
the first run and 692,224 in the second. The observed retained-state peak after
the second build was 375,136,256 allocated bytes, approximately 358 MiB. Each
controller build directory used 393,216 bytes, with console logs of 205,719 and
206,767 bytes. These are two-run local measurements, not total growth from
empty state, steady-state capacity, production, or AWS evidence.

Run a normal single-runtime validation and publish its status with:

```bash
(
  set -eu
  set -o pipefail
  : "${PR_NUMBER:?set PR_NUMBER to the open pull-request number}"
  PYTHON_BIN="$PWD/.venv/bin/python" make -C platform/jenkins quality
  operator_log="$(mktemp "${TMPDIR:-/tmp}/jenkins-operator.XXXXXX")"
  chmod 0600 "${operator_log}"
  if ! make -C platform/jenkins e2e-ephemeral 2>&1 | tee "${operator_log}"; then
    printf 'Jenkins failed; retained log: %s\n' "${operator_log}" >&2
    exit 1
  fi
  if ! platform/jenkins/scripts/report-github-status.py \
    --pr "${PR_NUMBER}" --log "${operator_log}"; then
    printf 'Status reporting failed; retained log: %s\n' "${operator_log}" >&2
    exit 1
  fi
  rm -f "${operator_log}"
)
```

Repeating the command reuses the bounded retained directory and exercises
history restoration plus current-build artifact readback. The older build
artifact HTTP assertion recorded above was a separate trusted-operator check
during the second runtime; the one-run target does not repeat that assertion.

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
