# Curriculum Rubric

## Capability levels

- Explain: describe the abstraction and responsibility boundary.
- Build: implement the smallest working version.
- Observe: expose useful metrics, logs, traces, and state.
- Break: reproduce a realistic failure.
- Recover: prove bounded recovery and data behavior.
- Compare: justify an alternative using current documentation and workload constraints.

## Core progression

1. Containers and Linux: processes, signals, filesystems, namespaces, cgroups, networking, images.
2. Distributed data: partitions, ordering, retries, idempotency, consistency, replication, backpressure.
3. Kubernetes workloads: probes, resources, scheduling, rollout, service discovery, configuration, storage.
4. Kubernetes platform: ingress/Gateway, policy, identity, secrets, autoscaling, GitOps, observability.
5. Cloud foundation: accounts/projects, IAM, VPC/VNet, DNS, load balancing, KMS, registries, managed data.
6. IaC and delivery: modules, state, plans, policy checks, OIDC, promotion, rollback, drift.
7. Reliability and security: SLOs, alerting, capacity, failure injection, backup/restore, least privilege, incident response.

## Proof standards

- A manifest alone does not prove runtime behavior.
- Multiple replicas do not prove high availability.
- A passing happy-path test does not prove delivery semantics.
- A dashboard does not prove actionable observability.
- A successful apply does not prove safe lifecycle management.

Require a reproducible experiment and recorded result for each major claim.
