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
- `terraform`: legacy infrastructure until provider-specific OpenTofu modules
  are introduced in the AWS slice.
- `charts`: application Helm packages; legacy chart behavior is not assumed to
  be correct until separately verified.

Secrets are not stored here. Local bootstrap creates the Grafana credential in
the cluster, and later cloud environments will use workload identity plus a
managed secret service.
