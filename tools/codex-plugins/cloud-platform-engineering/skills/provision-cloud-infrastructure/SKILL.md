---
name: provision-cloud-infrastructure
description: Design, implement, review, and validate cloud infrastructure with Terraform or OpenTofu across AWS, GCP, and Azure. Use for VPC/VNet design, managed Kubernetes, IAM and workload identity, registries, load balancers, DNS, KMS, databases, streaming services, remote state, modules, CI plans, policy checks, migrations, drift, or infrastructure pull requests.
---

# Provision Cloud Infrastructure

Treat infrastructure code as a lifecycle system, not a resource-creation script.

## Workflow

1. Inspect existing state boundaries, providers, lock files, modules, environments, and CI before editing.
2. Capture region, availability, network, identity, data, compliance, RTO/RPO, and cost constraints.
3. Use `cloud-docs` to verify current provider and service behavior. Consult provider schemas for exact resource arguments.
4. Design ownership and dependency boundaries; keep foundational networking, cluster/add-ons, data services, and applications independently evolvable.
5. Implement least privilege, encryption, private connectivity, tagging, observability, backup, and deletion protection appropriate to the environment.
6. Format and validate. Run static policy/security checks and tests available in the repository.
7. Produce a speculative plan only when credentials and backend access are authorized. Never apply unless the user explicitly requests it.
8. Review replacements, deletions, secret exposure, state moves, quotas, rollout order, rollback, and cost before any apply.

Read [iac-review.md](references/iac-review.md) for infrastructure design or pull-request review.
