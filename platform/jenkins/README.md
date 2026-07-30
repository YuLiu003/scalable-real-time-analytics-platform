# Jenkins Platform

This is a production-like, free Jenkins reference environment for the
investment platform. It proves Jenkins controller operations and ephemeral
Kubernetes agents without claiming that a local lab is a production deployment.

## Architecture

- The official Jenkins Helm chart `5.9.45` runs Jenkins `2.568.1` in a dedicated
  Kubernetes namespace.
- Jenkins Configuration as Code and Job DSL own global configuration and the
  pipeline job.
- The controller has zero executors, no cloud credentials, namespace-scoped
  RBAC, a PVC, probes, a disruption budget, and network policy.
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

## Run

```bash
make -C platform/jenkins quality
make -C platform/jenkins e2e-ephemeral
platform/jenkins/scripts/report-github-status.py \
  --pr 29 \
  --log /absolute/path/to/retained-operator.log
```

The ephemeral target creates a dedicated Colima VM, deploys Jenkins to kind,
builds the immutable agent toolchain, runs PS0/PS1/PS2, then deletes the VM and
all container data. It defaults to 8 CPUs and 16 GiB because PS2 creates a
single-node nested functional-test cluster; `JENKINS_COLIMA_CPUS` and
`JENKINS_COLIMA_MEMORY_GIB` can override those values. A clean run can take up
to two hours on a slow connection because it downloads pinned infrastructure
images and providers. The outer cluster preloads the integration-agent images
before starting the bounded Jenkins build.

The trigger selects the current source branch and binds the build to its exact
40-character commit. Every stage fails if its checkout differs. The status
reporter creates or updates a PR evidence comment, then points the
`jenkins / presubmit` commit status to that comment.

The nested CI topology is deliberately smaller than the normal three-node local
topology because it runs inside privileged DinD. Multi-node scheduling remains
available through `platform/local/kind/cluster.yaml`; the AWS OpenTofu tests
independently enforce the three-zone cloud design.

## Production boundary

For a real deployment, replace local images with registry digests, use OIDC for
administrators, replace the operator reporter with GitHub Branch Source and a
GitHub App on the trusted controller, put integration agents in an isolated
autoscaled node pool, use CSI snapshots or a backup controller for the PVC,
retain console logs and artifacts outside the disposable cluster, and scrape
`/prometheus/`. Target RTO is 60 minutes from Helm, JCasC, and the latest
backup; target RPO is 24 hours for build history.

Official design sources:

- https://www.jenkins.io/doc/book/security/controller-isolation/
- https://www.jenkins.io/doc/book/managing/casc/
- https://plugins.jenkins.io/kubernetes/
- https://plugins.jenkins.io/github-branch-source/
