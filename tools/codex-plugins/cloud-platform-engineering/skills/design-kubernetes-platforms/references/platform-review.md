# Kubernetes Platform Review

## Workload correctness

- Define stateless versus stateful components and external consistency requirements.
- Align replica count with application concurrency and data partitioning semantics.
- Define startup, readiness, liveness, graceful shutdown, and termination behavior.
- Set requests and limits from measurements; define overload and backpressure behavior.
- Test rollout, drain, reschedule, duplicate execution, and dependency failure.

## Platform boundaries

- Choose regional/zonal topology and map every dependency to failure domains.
- Separate system, platform, tenant, and application boundaries deliberately.
- Use workload identity instead of long-lived cloud credentials.
- Define ingress, egress, DNS, service discovery, network policy, and certificate ownership.
- Choose storage by access mode, durability, backup, restore, and failover semantics.

## Operations

- Define supported Kubernetes versions, upgrade cadence, and add-on compatibility.
- Separate infrastructure provisioning, cluster add-ons, and workload delivery lifecycles.
- Use immutable artifacts, promotion, policy checks, drift detection, and rollback.
- Collect metrics, logs, traces, events, audits, and deployment metadata.
- Define SLOs, alerts, runbooks, capacity thresholds, and incident access.

## Decision output

For each major component, record owner, responsibility boundary, chosen option, rejected alternatives, source evidence, failure mode, recovery mechanism, and validation test.
