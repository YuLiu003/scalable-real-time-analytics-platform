# Jenkins Platform

This is a production-like, free Jenkins reference environment for the
investment platform. It exercises Jenkins controller operations, ephemeral
Kubernetes agents, and S3-compatible artifact management without claiming that
a local lab is a production deployment.

## Architecture

- The official Jenkins Helm chart `5.9.45` runs Jenkins `2.568.1` in a dedicated
  Kubernetes namespace.
- Jenkins Configuration as Code and Job DSL own global configuration and the
  pipeline job.
- The job loads its Pipeline definition from the trusted `main` branch, then
  that trusted Pipeline checks out the requested source branch at the exact
  commit. PR-controlled source cannot replace the agent Pod templates.
- The controller has zero executors, no cloud credentials, namespace-scoped
  RBAC, probes, a disruption budget, network policy, and a host-retained PVC.
  A second host-retained PVC stores Garage data. Each requests 2 GiB, but the
  local hostPath driver does not enforce those requests as filesystem quotas.
- The Artifact Manager on S3 plugin sends archived verification output to a
  dedicated single-node Garage bucket. Jenkins receives a read/write-only key;
  its agents receive neither that key nor the Garage administrative key.
- Jenkins build metadata and Garage data survive recreation of the disposable
  VM. The native discarder removes a build when either its three-day age or the
  20-build count boundary applies. Garage independently applies a three-day
  object lifecycle plus a 1 GiB/10,000-object bucket quota.
- `jenkins-verify` agents run PS0 and PS1 as non-root pods without Kubernetes
  service-account tokens.
- `jenkins-integration` runs PS2 with a privileged Docker-in-Docker sidecar.
  That agent is allowed only inside the disposable lab VM. A real deployment
  must place it in a separate disposable node pool or account away from the
  controller.
- GitHub remains the merge authority. A production Jenkins installation should
  use a GitHub App on the trusted controller for webhook delivery and commit
  status reporting. Tokens are never bound into an untrusted PR Pipeline.
- The lab publishes `jenkins / presubmit` only from a trusted host-side
  reporter after it verifies the exact PR head, ordered success evidence, and
  VM deletion. The reporter token is never available to the Pipeline.

## Prerequisites

The disposable workflow requires macOS, Colima, the Docker CLI, kind at the
exact version in `versions.lock`, a compatible kubectl client, Helm 3, Make,
OpenSSL, Python 3, curl, and Git. A Docker daemon does not need to be running
before invocation because the workflow creates an isolated Colima VM.

Install the repository's Python development dependencies using the
[root setup instructions](../../README.md#run-locally) before running the
quality gate. The disposable VM defaults to 8 CPUs, 16 GiB of memory, and
50 GiB of disk.

## Run

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

The ephemeral target creates a dedicated Colima VM, deploys Jenkins to kind,
builds the immutable agent toolchain, runs PS0/PS1/PS2, then deletes the VM and
all container data. The 50 GiB VM disk therefore remains temporary. Only the
bounded state described below remains on the host. Temporary Docker and
Kubernetes configuration keeps the caller's active contexts unchanged and
discards generated context records. The VM defaults to 8 CPUs and 16 GiB
because PS2 creates a single-node nested functional-test cluster;
`JENKINS_COLIMA_CPUS` and `JENKINS_COLIMA_MEMORY_GIB` can override those values.
A clean run can take up to two hours on a slow connection because it downloads
pinned infrastructure images and providers. The outer cluster preloads the
integration-agent images before starting the bounded Jenkins build. Cleanup
first scales down Jenkins, terminates agents, scales down Garage, and waits for
their Pods before deleting kind and the Colima VM. If Colima deletion fails,
the command returns failure and retains both ownership fences rather than
claiming cleanup or permitting a second writer.

The trigger selects the current source branch and binds the build to its exact
40-character commit. Every stage fails if its checkout differs. The status
reporter creates or updates a PR evidence comment, then points the
`jenkins / presubmit` commit status to that comment.

The trusted Pipeline branch defaults to `main`. While developing a Jenkinsfile
change before it reaches `main`, an operator who has reviewed that branch may
explicitly trust it for the disposable proof:

```bash
JENKINS_TRUSTED_PIPELINE_BRANCH="$(git branch --show-current)" \
  make -C platform/jenkins e2e-ephemeral
```

Do not set this override to an unreviewed PR branch. It changes which code is
allowed to define privileged CI orchestration; it does not change the source
commit being tested.

### Temporary web UI

Start a temporary operator session in the first terminal:

```bash
make -C platform/jenkins ui
```

This creates an isolated Colima VM, kind cluster, and Jenkins runtime, then
forwards Jenkins to `http://127.0.0.1:18080` and Garage to
`http://127.0.0.1:13900`. It stays in the foreground for 7,200 seconds by
default. `JENKINS_UI_TIMEOUT_SECONDS` may shorten that interval or extend it to
at most 14,400 seconds. Press Ctrl-C to stop earlier. Normal exit, timeout, and
handled interruption tear down kind and Colima; if runtime deletion cannot be
proven, the command fails and retains its ownership fences for diagnosis.
If local port `18080` is already in use, set `JENKINS_LOCAL_PORT` on the `ui`
command and open the URL it prints.

In a second trusted terminal, explicitly retrieve the current runtime's
password:

```bash
make -C platform/jenkins ui-password
```

Sign in as `admin`. Treat the password output as a secret: do not paste it into
logs, issues, chat, or Git. Bootstrap creates a fresh administrator password
for each runtime, and that current password unlocks all bounded history
restored into the controller. No live password can be retrieved after the
runtime stops. Garage artifact links use signed URLs through the automatic
port `13900` forward and require no separate Garage login.

UI mode does not trigger a Jenkins build or run PS0, PS1, or PS2. The Colima
VM, kind cluster, controller, and port-forwards consume local compute and disk
only while the session is live; the bounded Jenkins history and Garage
artifacts described below remain on local disk afterward. If overriding the
retained-state location, pass the same absolute path to both terminals:

```bash
# Terminal 1
JENKINS_RETAINED_STATE_DIR=/absolute/private/path \
  make -C platform/jenkins ui

# Terminal 2
JENKINS_RETAINED_STATE_DIR=/absolute/private/path \
  make -C platform/jenkins ui-password
```

The normal `e2e-ephemeral` workflow remains unattended and does not require UI
login.

The nested CI topology is deliberately smaller than the normal three-node local
topology because it runs inside privileged DinD. Multi-node scheduling remains
available through `platform/local/kind/cluster.yaml`; the AWS OpenTofu tests
independently enforce the three-zone cloud design.

## Retained history and disk use

The default retained-state directory is:

```text
~/.local/share/investment-platform/jenkins
```

It contains `controller`, `garage`, `reports`, and `secrets` directories.
Override it only with an absolute path outside the repository, without any
symlink or dot components, colons, or newlines:

```bash
JENKINS_RETAINED_STATE_DIR=/absolute/private/path \
  make -C platform/jenkins e2e-ephemeral
```

After a build, inspect the total retained size and the newest per-build report:

```bash
du -sh "${JENKINS_RETAINED_STATE_DIR:-$HOME/.local/share/investment-platform/jenkins}"
python3 -m json.tool "$(ls -t \
  "${JENKINS_RETAINED_STATE_DIR:-$HOME/.local/share/investment-platform/jenkins}"/reports/*.json \
  | head -n 1)"
```

The report contains only the commit, build number, result, allocated-byte
baseline/peak/growth for retained state and the Colima profile, plus controller
build and console sizes. It does not contain prompts, source paths, financial
data, credentials, or artifact contents. Reports use the
`reports/<commit>-build-<number>.json` shape, remain local, and are not included
in the GitHub evidence comment. Reports older than three days are pruned at the
next retained-state preparation.

Jenkins removes build records older than three days and keeps no more than 20.
Garage expires objects after three days and refuses writes after the dedicated
bucket reaches 1 GiB or 10,000 objects. Old local records are reconciled when
Jenkins and Garage next run, so a laptop that remains off does not perform
wall-clock cleanup in the background. Startup and successful completion also
reject retained state above the 4 GiB safety boundary. The PVC sizes describe
the Kubernetes storage contract; hostPath does not enforce those sizes as a
filesystem quota. The 4 GiB check is fail-fast detection, not a filesystem
quota or an automatic deletion of newer history.

An owner-token-protected retained-state lock prevents different Colima profiles
from mounting the same controller and Garage directories concurrently. A
separate profile lock closes the inverse race. Manual bootstrap records its
owner token in the live cluster so a later `destroy` can validate ownership
before mutation. Normal completion and handled signals release both locks only
after runtime deletion succeeds. Failed or uncertain deletion keeps both
fences; a hard host/process kill requires explicit operator recovery after
confirming that no lab still owns the state.

The Jenkins UI is local and exists only while the disposable runtime is active;
history reappears on the next run because the controller metadata is remounted.
This state consumes local Mac disk. Garage avoids AWS charges and exercises the
S3 API contract, but it does not turn local storage into free cloud storage or
an off-host backup. MinIO is intentionally not added because Garage already
fills that role in this repository.

The local operator owns this directory. To remove all retained Jenkins history,
artifacts, reports, and credentials, use the marker- and confirmation-gated
purge command when no Jenkins lab is running:

```bash
CONFIRM_JENKINS_RETAINED_PURGE=investment-platform-jenkins \
  make -C platform/jenkins purge-retained
```

## Security boundary

Retained secrets are created outside Git with mode `0600`; the secret and
report directories use mode `0700`. Kubernetes agents do not mount the
host-retained directory. Jenkins receives only the dedicated bucket's runtime
key, which Garage grants read/write access while denying bucket creation and
owner access. That credential is system-scoped rather than Pipeline-visible,
and the administrative key never enters Jenkins or its agents. Kubernetes
Secrets are created from protected files so long-lived values do not appear in
host process arguments.

This is still a trusted local lab. Garage uses HTTP inside the disposable
cluster, its static local keys persist on one developer host, and no
Garage-specific NetworkPolicy is claimed.

## Production boundary

For a real deployment, replace local images with registry digests, use OIDC for
administrators, replace the operator reporter with GitHub Branch Source and a
GitHub App on the trusted controller, and put integration agents in an isolated
autoscaled node pool. Replace single-node HTTP Garage and hostPath volumes with
a TLS-protected, replicated object store and CSI-backed controller storage with
tested backups; then scrape `/prometheus/`. Target RTO is 60 minutes from Helm,
JCasC, and the latest backup; target RPO is 24 hours for build history. Those
are unverified design targets, not results from this local lab.

Official design sources:

- https://www.jenkins.io/doc/book/security/controller-isolation/
- https://www.jenkins.io/doc/book/managing/casc/
- https://plugins.jenkins.io/kubernetes/
- https://plugins.jenkins.io/github-branch-source/
- https://plugins.jenkins.io/artifact-manager-s3/
- https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/
