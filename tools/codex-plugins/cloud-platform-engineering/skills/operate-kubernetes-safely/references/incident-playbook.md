# Kubernetes Incident Playbook

## Evidence order

1. Scope: context, namespace, resource, owner, impact, start time.
2. Desired state: controller spec, rollout strategy, probes, resources, placement, configuration references.
3. Observed state: status conditions, restarts, readiness, scheduling, endpoints, volume attachments.
4. Timeline: namespace events sorted by time and relevant controller events.
5. Workload signals: current and previous logs, metrics, traces, profiles when appropriate.
6. Dependency signals: DNS, network path, certificate, identity, API, database, queue, and storage health.

## Common hypothesis groups

- Image or startup: pull, command, permissions, filesystem, missing configuration.
- Health contract: incorrect probe path, timeout, dependency-coupled liveness, slow startup.
- Scheduling: requests, taints, affinity, topology, quotas, unbound claims.
- Networking: selector mismatch, missing endpoints, DNS, NetworkPolicy, CNI, ingress/Gateway.
- Identity: service account, RBAC, workload identity, token audience, secret reference.
- Resources: CPU throttling, OOM, disk pressure, PID/file-descriptor exhaustion.
- Stateful behavior: access mode, fencing, leader election, quorum, replay, backup/restore.

## Mutation ladder

1. No mutation: inspect and reproduce.
2. Targeted reversible configuration correction through the declared delivery path.
3. Controlled rollout or single-component restart with disruption bounds.
4. Temporary scaling or traffic shift with explicit data/concurrency analysis.
5. Destructive recovery only with approval, backup, rollback, and evidence capture.

Verify at the service boundary, not only with `Ready=True`.
