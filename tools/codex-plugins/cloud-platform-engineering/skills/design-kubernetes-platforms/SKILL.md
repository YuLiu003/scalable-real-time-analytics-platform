---
name: design-kubernetes-platforms
description: Design and review Kubernetes platforms and workloads across self-managed clusters, EKS, GKE, and AKS. Use for architecture diagrams, ADRs, namespace and tenancy models, networking, identity, storage, autoscaling, GitOps, observability, upgrades, security, availability, disaster recovery, or managed-versus-self-hosted decisions.
---

# Design Kubernetes Platforms

Treat Kubernetes as one control plane in a larger system. Design application and cloud responsibility boundaries together.

## Workflow

1. Capture workload requirements: traffic, latency, state, tenancy, compliance, RTO/RPO, deployment frequency, team size, and budget constraints.
2. Model failure domains and data flows before choosing add-ons.
3. Decide what belongs in Kubernetes and what should be a managed cloud service.
4. Define cluster, node-pool, namespace, identity, and network trust boundaries.
5. Design workload availability, disruption, rollout, scaling, and state-recovery behavior.
6. Design delivery, policy, secrets, telemetry, upgrades, and incident access.
7. Use `cloud-docs` to validate version-sensitive features against Kubernetes plus the selected provider.
8. Produce explicit decisions, rejected alternatives, risks, and verification experiments.

Read [platform-review.md](references/platform-review.md) for any production-readiness review or target architecture.
