# Presubmit Gates

`PS` means presubmit. These gates validate a proposed change before it reaches
`main`; deployment and production credentials are outside this trust boundary.
The former CI, Go-lint, pre-merge, and deploy workflows were retired because
they targeted removed services and included floating or fail-open checks.

## Gate contract

| Gate | Target | Blocking evidence |
| --- | --- | --- |
| `PS0` | Fast local and PR policy | Clean diff, changed-file syntax, credential-material guard, SHA-pinned workflows, secure Jenkinsfile, completed PR checklist |
| `PS1` | Deterministic correctness | Unit and race tests, static checks, application coverage, plugin coverage, presubmit-runner coverage |
| `PS2` | System and infrastructure behavior | OpenTofu validation/tests, Kubernetes render, disposable kind workflow, scale/recovery acceptance, bounded capacity smoke, diagnostics, automatic cleanup |

All three gates fail closed. Warnings are informational only when the check is
explicitly outside the gate contract.

Run them with:

```bash
make presubmit-ps0
make presubmit-ps1
make presubmit-ps2
```

`PS0` requires only Git, Bash, and Python. `PS1` requires the pinned Python and
Go test dependencies. `PS2` requires Docker, kind, kubectl, Helm, and OpenTofu.
On a local Mac, `PS2` uses the disposable Colima path and deletes its dedicated
VM and data. Linux CI/Jenkins agents delete the kind cluster and should
themselves be ephemeral.

The bounded capacity gate is one 10,000-event trial. The repeated
10K/50K/100K five-run matrix is separate because shared-runner contention and
merge-gate timeouts would make capacity comparisons misleading.

## PR review checklist

The pull-request template makes these reviews explicit:

- `SCOPE`: smallest coherent diff and no unrelated refactor.
- `TEST`: behavior, boundaries, regression, and failure evidence.
- `SEC`: inputs, secret exposure, dependencies, injection, and least privilege.
- `AUTH`: authentication, authorization, tenant isolation, denied paths, token
  lifecycle, and audit events.
- `DATA`: personal/financial data classification, encryption, retention,
  deletion, backup, and migration.
- `DIST`: ordering, retries, idempotency, concurrency, replay, and partial
  failure.
- `INFRA`: Kubernetes, IAM, networking, state, images, resources, and cost.
- `OPS`: telemetry, SLO impact, capacity, rollout, rollback, cleanup, and
  recovery.

Checking an item means it was considered; it may be not applicable. The PR
description should state why when the reason is not obvious. `PS0` blocks a
GitHub pull request while any item remains unchecked. Automation cannot prove
the truth of the attestation, so review and CODEOWNERS remain separate controls.

## Required `main` protection

Require the Jenkins aggregate status:

- `jenkins / presubmit`

GitHub Actions continues to run `presubmit / PS0`, `presubmit / PS1`, and
`presubmit / PS2` as independent portability evidence, but Jenkins is the
primary merge gate.

Also require a pull request, conversation resolution, strict up-to-date checks
or a merge queue, no force pushes or deletions, and no administrator bypass.
For a team repository, require at least one approval, dismiss stale approvals,
require the latest push to be approved by someone else, and require CODEOWNER
review. A solo repository cannot honestly satisfy independent approval; use
zero approvals until a second trusted reviewer exists while retaining all
automated gates and the PR checklist.

The setup helper defaults to the honest solo configuration. For a team, set
`REQUIRED_APPROVALS=1`, `REQUIRE_CODEOWNER_REVIEWS=true`, and
`REQUIRE_LAST_PUSH_APPROVAL=true`.

Repository files declare the checks, but they do not activate GitHub branch
protection. An administrator must apply and verify the rule.

## Jenkins trust boundary

The `Jenkinsfile` is a credential-free adapter to the same repository commands.
Configure Jenkins as follows:

- Set controller executors to zero. Run PRs only on agents labeled
  `linux && ephemeral && untrusted`.
- Give agents no access to `JENKINS_HOME`, no `sudo`, and no shared workspace
  with trusted jobs.
- Require authentication, use narrow matrix/folder authorization, keep CSRF
  and agent-to-controller protection enabled, and restrict credential creation.
- Do not attach deployment, registry-write, cloud, kubeconfig, or signing
  credentials to multibranch PR jobs. A read-only SCM checkout credential is
  the maximum needed.
- Scope any later post-merge credential to the lowest folder/job and bind it
  only inside the trusted stage that needs it.
- Use an LTS controller, pinned plugins, bounded build retention, ephemeral
  agents, and workspace deletion after every run.
- Bind each build to the PR head SHA in trusted job input, and make every stage
  reject a checkout that differs from it.

The free lab uses `platform/jenkins/scripts/report-github-status.py` after the
disposable run exits successfully. It verifies the exact open PR head, ordered
PS0/PS1/PS2 success, and VM deletion before publishing
`jenkins / presubmit`. This operator workflow is production-like evidence, not
a substitute for a continuously available controller. Production should use
GitHub Branch Source with a GitHub App held by the controller plugin, retain
logs externally, and never expose that credential to PR-controlled Pipeline
steps.

Jenkins documents that masking only reduces accidental disclosure and that an
untrusted Pipeline can still capture credentials. It also recommends
controller/agent isolation and lowest-scope credentials:

- [Using a Jenkinsfile](https://www.jenkins.io/doc/book/pipeline/jenkinsfile/)
- [Jenkins credentials security](https://www.jenkins.io/doc/book/security/credentials/)
- [Jenkins controller isolation](https://www.jenkins.io/doc/book/security/controller-isolation/)

GitHub required checks and review rules are the merge authority:

- [Protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [CODEOWNERS and branch protection](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)
- [Secure use of GitHub Actions](https://docs.github.com/en/actions/reference/security/secure-use)
