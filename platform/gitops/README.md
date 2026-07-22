# GitOps Desired State

This directory separates cluster desired state from imperative bootstrap logic.

```text
clusters/
  local/
    namespaces/   # Namespace ownership and local resource guardrails
```

The local bootstrap applies this desired state with `kubectl apply -k` until an
Argo CD control plane is introduced in a later delivery slice. Moving the same
directory under Argo CD must not require rewriting the resources.

Repository boundaries:

- `platform/local`: cluster creation, pinned local dependencies, verification,
  diagnostics, and teardown.
- `platform/gitops`: declarative cluster and workload desired state.
- `infra/opentofu/aws`: provider-specific state and EKS lab lifecycles.
- `terraform`: legacy ECS infrastructure; it is not used by the investment
  analytics AWS path.
- `charts`: application Helm packages; legacy chart behavior is not assumed to
  be correct until separately verified.

Secrets are not stored here. Local bootstrap creates the Grafana credential in
the cluster, and later cloud environments will use workload identity plus a
managed secret service.

Reusable resources live under `apps/base`, `clusters/base`, and
`platform/base`. The `local` overlays retain Garage and single-node Kafka
settings; the `aws-lab` overlays replace them with EKS Pod Identity, S3/ECR
images, EBS storage, and a three-node replicated Kafka topology.
