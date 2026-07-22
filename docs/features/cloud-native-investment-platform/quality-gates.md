# Portfolio Platform Quality Gates

| Field | Value |
| --- | --- |
| Status | Implemented on stacked quality branch |
| Branch | `feature/portfolio-analytics-quality-gates` |
| Parent | `feature/portfolio-analytics-slice3` at `e4920ae` |
| Coverage threshold | 100% in every measured application scope |
| CI workflow | `Portfolio Platform Quality` |
| Last verified | 2026-07-21 |

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
| `portfolio_analytics` Python package | Statements and branches | 100% | 352/352 statements; 92/92 branches |
| Portfolio API `internal/...` packages | Go statements with `-race` | 100% | 100.0% |
| Market `internal/event` and `internal/synthetic` | Go statements with `-race` | 100% | 100.0% |

The Python denominator contains the analytical model, transformation, S3
adapter, build command, and independent Parquet query command. Only structural
`if __name__ == "__main__"` launch guards are excluded; their `main()` functions
are called by tests and their real module entrypoints run in Kubernetes.

The Go denominator contains the portfolio API's HTTP, result-validation, and S3
packages plus the market event and synthetic-fixture domain packages. Thin Go
process entrypoints, Kafka/S3 SDK wiring from the prior slice, generated
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
PYTHON_BIN=.venv/bin/python make -C platform/local quality
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

The local-equivalent gate completed with 352/352 Python statements, 92/92
Python branches, and 100.0% Go statement coverage in both measured profiles.
All three production images built, and all 27 analytics tests passed again from
inside the non-root production image.

The AWS static gate validated both OpenTofu roots with AWS provider 6.55.0,
passed 2 state-foundation and 5 platform architecture tests, rendered every AWS
overlay without static AWS credentials, and passed Strimzi 1.1 server-side
schema dry-run for the replicated Kafka resources. This is implementation
evidence only; AWS runtime, failure, cost, and teardown evidence remains pending
an explicitly authorized billable apply.

A clean scoped kind rebuild then reproduced the exact Slice 2 terminal state of
7 market records, 1 quarantine record, and 4 immutable bronze objects. The
first run exposed a race in the test itself: its deletion trigger accepted any
event's post-write delay marker. The verifier was corrected to match the exact
SP500 event before deletion and its exact durable `duplicate` record after
restart. A second clean rebuild passed the producer-acknowledgement and consumer
redelivery injections.

Slice 3 then passed its independent DuckDB query, readiness-loss boundary, and
bronze-only replay with the same deterministic result:

```text
total_market_value=6200.00000000
positions=3
input_set_sha256=4faafb293f811f8475712b858e6c22109dca5a7dfccdf2fe9ddcacd63a1799e1
result_sha256=ee62de5a28a25c34b67cf9df59deaf810a657a2994d78c416ff28f6cb08c99d6
```

## Repository enforcement

After this workflow has run on GitHub, protect `main` and require these unique
checks:

- `portfolio / 100% application coverage`
- `portfolio / container build`
- `portfolio / kind end-to-end`
- `portfolio / aws infrastructure`

GitHub documents that required checks must pass before a protected branch can
merge. The workflow uses least-privilege default permissions and grants
`packages: write` only to the delivery job. All referenced actions are pinned
to full commit SHAs, following GitHub's secure-use guidance.

Authoritative references, retrieved 2026-07-21:

- [GitHub Actions secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [GitHub protected branches and required checks](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [GitHub container image publishing](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)
- [kind CI installation guidance](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [Helm binary installation and verification](https://helm.sh/docs/intro/install/)
- [coverage.py package and supported Python versions](https://pypi.org/project/coverage/)
