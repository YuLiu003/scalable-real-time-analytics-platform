---
name: engineer-software-changes
description: Implement and review software changes for correctness, readability, maintainability, and proportionate design. Use when adding or modifying code, fixing bugs, refactoring, reviewing a diff or pull request, reducing unnecessary complexity, defining tests, or checking that a solution uses the smallest clear and correct change.
---

# Engineer Software Changes

Optimize for the smallest coherent change, not the fewest lines. Preserve clarity, behavior, and evidence.

## Workflow

1. State the requested behavior, constraints, invariants, and observable acceptance criteria.
2. Inspect the affected code, tests, repository guidance, and current diff before proposing a design.
3. Trace callers, data ownership, failure paths, and compatibility boundaries. Do not infer safety from one file.
4. Compare viable designs. Choose the lower-diff design when correctness and clarity are equal.
5. Implement only the required behavior. Reuse a fitting existing abstraction; do not force reuse through an awkward abstraction.
6. Test public behavior, the regression, meaningful boundaries, and realistic failures. Honor repository coverage requirements and measure them rather than assuming them.
7. Run focused checks first, then the broadest relevant repository gates.
8. Re-read the final diff as a reviewer. Remove accidental churn, redundant comments, dead paths, speculative options, and unrelated refactors.

## Design standard

- Make invariants and ownership explicit.
- Prefer cohesive units, narrow interfaces, one-way dependencies, and hidden implementation details.
- Keep policy separate from mechanisms only when they vary independently.
- Add a helper, interface, layer, configuration option, or dependency only when a concrete need justifies its cost.
- Prefer direct code over premature abstraction. Small duplication can be cheaper than the wrong shared abstraction.
- Use names to explain what; reserve comments for why, constraints, or non-obvious tradeoffs.
- Handle errors at the boundary that has enough context to act. Do not silently discard failures.
- Preserve compatibility unless the requested change explicitly alters it.
- Treat generated code, formatting churn, and incidental cleanup as separate changes when practical.
- Delete obsolete code when tests, callers, and repository search establish that it is safe.

Use patterns as tools, not goals. Apply principles such as information hiding, high cohesion, low coupling, dependency inversion, and single responsibility only when they make the current change easier to understand, test, or evolve.

## Review standard

Prioritize findings that can change behavior or operational outcomes:

1. Correctness, data loss, security, concurrency, and failure recovery
2. Contract, compatibility, migration, and distributed-systems semantics
3. Missing or misleading tests and observability
4. Unnecessary complexity, coupling, duplication, and readability problems with a concrete maintenance cost

For each finding, identify the affected file and line, explain the failure scenario, and propose the smallest safe correction. Distinguish defects from optional preferences. Do not request a rewrite merely to match personal style.

If there are no findings, say so and list any material verification gaps or residual risks. Do not invent issues to make a review appear thorough.

## Completion check

Before handoff, confirm that the diff:

- changes only what the request requires;
- has no simpler equally correct design;
- reads clearly without narration;
- covers important behavior and failure paths;
- passes the relevant checks; and
- documents any unverified assumption instead of hiding it.
