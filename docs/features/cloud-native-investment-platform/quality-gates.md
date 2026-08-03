# Portfolio Platform Quality Gates

| Field | Value |
| --- | --- |
| Status | Implemented |
| Coverage threshold | 100% in every measured application scope |
| CI workflows | `Presubmit` and `Portfolio Platform Quality` |
| Required merge check | `jenkins / presubmit` |
| Last verified | 2026-08-03 |

## Policy

Every portfolio-platform feature must add tests with the production change and
must keep its measured application scope at 100%. An average across services is
not accepted because a well-tested package must not hide an untested one.

Coverage is one gate, not the definition of correctness. The pipeline also
requires race tests, strict contract failures, deterministic Parquet replay,
container builds, rendered Kubernetes resources, readiness behavior, injected
producer/consumer failures, and a clean kind deployment.

## Coverage boundary

| Scope | Metric | Required | Current evidence |
| --- | --- | ---: | ---: |
| `portfolio_analytics` Python package | Statements and branches | 100% | 501/501 statements; 130/130 branches |
| Portfolio API `internal/...` packages | Go statements with `-race` | 100% | 100.0% |
| Market `internal/alpaca`, `internal/event`, `internal/synthetic`, `internal/archivemetrics`, `internal/scale`, and `internal/benchmark` | Go statements with `-race` | 100% | 100.0% |

The Python denominator contains the analytical model, transformation, S3
adapter, build command, and independent Parquet query command. Only structural
`if __name__ == "__main__"` launch guards are excluded; their `main()` functions
are called by tests and their real module entrypoints run in Kubernetes.

The Go denominator contains the portfolio API's HTTP, result-validation, and S3
packages plus the market event, private-provider, synthetic-fixture,
archive-metrics, scale-verification, and capacity-report domain packages. Thin
Go process entrypoints, Kafka/S3 SDK wiring from the prior slice, generated
artifacts, Kubernetes YAML, shell, and embedded HTML are not mislabeled as
unit-covered statements. They are still built, race-tested where applicable,
rendered, and exercised by the end-to-end gate. New domain logic must live in a
measured package; moving logic into an entrypoint to evade coverage violates
this policy.

Run the local gate after installing the pinned development dependencies:

```bash
python3 -m venv .venv
.venv/bin/python -m pip install \
  --requirement services/portfolio-analytics/requirements-dev.txt
PYTHON_BIN="$PWD/.venv/bin/python" make -C platform/local quality
```

Coverage files are written outside the repository by default so generated
evidence does not pollute feature diffs.

## CI test pyramid

The workflow runs on relevant pull requests, `main`, matching stacked feature
branches, and manual dispatch.

1. `portfolio / 100% application coverage` runs the exact local quality script,
   Go race detector, Go vet, shell parsing, and fixture parsing. Coverage output
   is retained for 14 days.
2. `portfolio / container build` cross-compiles all feature commands and builds
   the three runtime images from their pinned bases.
3. `portfolio / kind end-to-end` installs checksum-verified tools, renders every
   active Kustomize tree, creates the disposable multi-node cluster, and runs
   the complete Slice 1-3 bootstrap. That includes Kafka acknowledgement
   ambiguity, consumer redelivery, immutable S3 effects, DuckDB/Parquet queries,
   readiness loss, and exact replay. Failure diagnostics are retained for 14
   days and the cluster is always deleted.
   For local macOS use, `make -C platform/local e2e-ephemeral` applies the same
   disposable principle to a dedicated Colima profile and removes its VM disk
   in an exit trap. Mocked lifecycle tests prove success, failure, confirmation,
   absent-profile, and pre-existing-profile boundaries.
   The acceptance run also posts the source-controlled zero-return contribution
   scenario through the Kubernetes service proxy and requires its deterministic
   $2,200 ending balance, contribution total, and end-of-period timing contract.
4. `portfolio / aws infrastructure` verifies the pinned OpenTofu and AWS
   provider configuration, reusable Kustomize bases, AWS overlays, Kafka
   replication settings, Pod Identity service accounts, and absence of static
   AWS credential references. It never plans or applies against an account.
5. The separate manual `AWS Lab Plan` workflow exchanges GitHub's OIDC token
   for short-lived AWS credentials and creates a real remote-state-backed plan.
   Its role is plan-only, its target account is checked, and no binary plan or
   apply step is retained.
6. `portfolio / continuous delivery` runs only after a successful `main` push.
   It publishes the three images to GHCR with immutable `sha-<commit>` tags and
   a movable `main` tag.

Continuous delivery stops at a trusted registry artifact. It does not deploy to
an unspecified staging or production cluster. Cluster promotion requires a
separate environment contract, workload identity, registry pull policy,
rollback procedure, and GitOps reconciliation target.

## Verification record

The local-equivalent gate completed with 501/501 Python statements, 130/130
Python branches, and 100.0% Go statement coverage in both measured profiles.
All three production images built, and all 43 analytics tests passed again from
inside the non-root production image.

The private-provider package uses local fake WebSocket and historical HTTP
servers to cover authentication, exact subscription acknowledgement, pages,
disconnects, rate limits, backfill, overflow, stable identity, checkpoint
ordering, and privacy. This is contract evidence; no CI or local verification
claim here depends on real provider credentials.

The AWS static gate validated both OpenTofu roots with AWS provider 6.55.0,
passed 2 state-foundation and 5 platform architecture tests, rendered every AWS
overlay without static AWS credentials, and passed Strimzi 1.1 server-side
schema dry-run for the replicated Kafka resources. This is implementation
evidence only; AWS runtime, failure, cost, and teardown evidence remains pending
an explicitly authorized billable apply.

A clean disposable run on 2026-07-30 reproduced the Slice 2 and Slice 3
boundaries with the neutral fixture:

```text
total_market_value=600.00000000
positions=3
input_set_sha256=907e5275a884a446285cdff7023638f3bbc0a66660ee9b0a8bf53c3777c8405f
result_sha256=a3756ba641c48a7518bbf5cfd8338d85338d882bb62648a40a8c0547a0b3bae2
```

Per-head PS2 runs now require lag to remain observable across the HPA sampling
window, KEDA to reach the three-partition consumer ceiling, and three consumers
to remain pinned while a deleted pod is replaced. The replacement must join a
stable three-member group with all partitions assigned; replay must produce
only duplicate effects; lag must drain; and the Deployment must return to one
ready replica. Aggregate samples and a failure snapshot are retained with the
run, while exact measurements remain generated evidence rather than committed
capacity claims.

PS2 also runs one 10,000-event, fixed-three-consumer capacity smoke trial. It
requires exact acknowledged/topic/archive counts, zero unexpected outcomes,
trial-scoped durable-latency observations, per-partition committed lag, and
consumer/Kafka/Garage CPU and memory series. The full five-repeat
10K/50K/100K matrix remains a manual or scheduled disposable benchmark rather
than a merge-blocking workload.

This is `local_kind_synthetic` evidence, not AWS runtime, provider-data,
availability, or production-capacity evidence.

## Repository enforcement

The `main` ruleset requires the single `jenkins / presubmit` result. Jenkins
runs the same repository-owned PS0, PS1, and PS2 commands and publishes success
only after verifying the exact pull-request head and disposable-VM cleanup.

GitHub Actions independently reports coverage, container-build, kind
end-to-end, and AWS infrastructure results. They remain visible review evidence
without duplicating the required-status policy. Workflows use least-privilege
permissions, grant `packages: write` only to delivery, and pin third-party
actions to full commit SHAs.

Authoritative references, retrieved 2026-07-21:

- [GitHub Actions secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [GitHub protected branches and required checks](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [GitHub container image publishing](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)
- [kind CI installation guidance](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [Helm binary installation and verification](https://helm.sh/docs/intro/install/)
- [coverage.py package and supported Python versions](https://pypi.org/project/coverage/)
