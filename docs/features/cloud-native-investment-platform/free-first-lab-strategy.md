# Free-first cloud platform lab strategy

| Field | Decision |
| --- | --- |
| Status | Accepted for developer learning and routine CI |
| Date | 2026-07-22 |
| Default runtime | Ephemeral local kind on a disposable Colima profile |
| Routine end-to-end proof | Ephemeral kind on GitHub-hosted CI |
| Managed-cloud apply | Optional, time-boxed, separately authorized exercise |

## Decision

Routine learning and validation must not require a continuously running local
cluster or a paid cloud account. Use the same investment analytics workload in
three increasingly provider-specific layers:

1. Unit, contract, static IaC, and Kustomize tests run without a cluster.
2. Full Kubernetes, Strimzi Kafka, object storage, analytics, replay, and
   failure tests run in an ephemeral kind environment that is deleted after the
   run.
3. EKS, GKE, or AKS is used only to prove provider behavior that local tools
   cannot reproduce, such as workload identity, managed networking, cloud
   storage encryption, billing tags, managed load balancers, or actual zone
   recovery.

The AWS OpenTofu implementation therefore remains valuable even when never
applied: it is statically tested architecture and a concrete comparison target.
A real apply is a separate learning lab, not the normal development loop.

## Responsibility mapping

| Learning responsibility | Free-first implementation | Managed comparison |
| --- | --- | --- |
| Kubernetes API, scheduling, rollout and failure | kind | EKS, GKE, or AKS |
| Kafka partitions, quorum, replay and lag | Strimzi Kafka | MSK or another managed Kafka service |
| S3-compatible object lifecycle | Garage | S3 |
| Metrics and dashboards | Prometheus and Grafana | CloudWatch, Cloud Monitoring, or Azure Monitor |
| Infrastructure as code | OpenTofu mocked plans and validation | Provider-backed remote plan/apply |
| Workload isolation | Kubernetes service accounts, RBAC and Pod Security | EKS Pod Identity, GKE Workload Identity Federation, or Azure Workload Identity |
| Image delivery | Local Docker and GHCR | ECR, Artifact Registry, or ACR |

These are responsibility analogues, not API-identical replacements. For
example, Garage teaches S3-compatible object behavior but does not emulate GCS
or Azure Blob APIs, and Kubernetes RBAC does not reproduce a cloud IAM token
service.

## Current managed-service cost boundary

- Amazon EKS charges a per-cluster hourly fee and separately charges worker
  compute, EBS, public IPv4 addresses, and other supporting resources.
- GKE provides a monthly credit equivalent to one Autopilot or zonal cluster
  management fee, but the credit does not pay for compute or regional-cluster
  fees.
- AKS Free removes the cluster-management charge for learning and small test
  clusters, but consumed resources such as worker VMs, disks, networking, and
  registries remain pay-as-you-go.

There is therefore no assumption that a useful three-node Kafka lab is fully
free on a managed provider. Before any apply, require a sandbox account,
budget/credit check, reviewed plan, expected-account guardrail, exact teardown
command, and a short expiration window.

## Recommended operating model

- Run application coverage and mocked infrastructure tests locally.
- Run `make -C platform/local e2e-ephemeral` only when interactive cluster
  debugging is necessary.
- Let the existing GitHub Actions job run the routine full kind acceptance
  test. Standard GitHub-hosted runners are free for public repositories; private
  repositories use the plan's included minutes before paid overage.
- Keep the AWS workflow plan-only by default.
- Perform one short managed-cloud lab per provider-specific milestone, capture
  evidence, and destroy it the same day.

## Authoritative references

- [Docker pruning behavior](https://docs.docker.com/engine/manage-resources/pruning/)
- [kind cluster deletion](https://kind.sigs.k8s.io/docs/user/quick-start/#deleting-a-cluster)
- [Amazon EKS pricing](https://aws.amazon.com/eks/pricing/)
- [Google Kubernetes Engine pricing](https://cloud.google.com/kubernetes-engine/pricing)
- [AKS cluster-management pricing tiers](https://learn.microsoft.com/en-us/azure/aks/free-standard-pricing-tiers)
- [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
