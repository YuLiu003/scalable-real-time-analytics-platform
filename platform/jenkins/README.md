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

## Run

```bash
make -C platform/jenkins quality
make -C platform/jenkins e2e-ephemeral
```

The ephemeral target creates a dedicated Colima VM, deploys Jenkins to kind,
builds the immutable agent toolchain, runs PS0/PS1/PS2, then deletes the VM and
all container data.

## Production boundary

For a real deployment, replace local images with registry digests, use OIDC for
administrators, install a GitHub App on the controller, put integration agents
in an isolated autoscaled node pool, use CSI snapshots or a backup controller
for the PVC, and scrape `/prometheus/`. Target RTO is 60 minutes from Helm,
JCasC, and the latest backup; target RPO is 24 hours for build history.

Official design sources:

- https://www.jenkins.io/doc/book/security/controller-isolation/
- https://www.jenkins.io/doc/book/managing/casc/
- https://plugins.jenkins.io/kubernetes/
- https://plugins.jenkins.io/github-branch-source/
