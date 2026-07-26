## Change

Describe the user-visible behavior, problem being solved, and intentionally excluded work.

## Risk and evidence

List important failure modes, test evidence, rollout or migration effects, and rollback steps.

## Required presubmit review

Check every item after reviewing it. Check an item when it is not applicable only after confirming why.

- [ ] `SCOPE` The diff is the smallest coherent change and contains no unrelated refactor.
- [ ] `TEST` New and changed behavior, boundaries, and failure paths have automated evidence.
- [ ] `SEC` Inputs, secrets, dependencies, injection paths, and least privilege were reviewed.
- [ ] `AUTH` Authentication, authorization, tenant boundaries, denied paths, and auditability were reviewed.
- [ ] `DATA` Financial or personal data handling, retention, encryption, and migrations were reviewed.
- [ ] `DIST` Ordering, retries, idempotency, concurrency, replay, and partial failure were reviewed.
- [ ] `INFRA` Kubernetes, IAM, network, state, image, resource, and cost impacts were reviewed.
- [ ] `OPS` Observability, capacity, rollout, rollback, cleanup, and incident recovery were reviewed.

## Evidence

- PS0:
- PS1:
- PS2:
