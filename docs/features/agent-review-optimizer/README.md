# Agent Review Optimizer

| Field | Decision |
| --- | --- |
| Status | Local Codex MVP implemented |
| Product objective | Improve code-review quality per unit of agent usage |
| Trust boundary | Read-only routing; allowlisted pseudonymous records; human final decision |
| Cost boundary | Tests and analysis make no model or paid-cloud calls |

## Problem

Broadcasting every change to every specialist consumes tokens and produces
duplicate or irrelevant findings. Token counts alone also do not measure value:
a cheaper review that misses an important defect is worse.

The MVP provides three deterministic operations:

```text
changed paths -> local router -> applicable reviewer categories
Codex JSONL  -> allowlist adapter -> sanitized invocation record
records      -> aggregator -> descriptive reviewer-level cohorts
```

The optimizer is a sibling developer tool under `tools/codex-plugins/`. It does
not change the investment services or claim that a personal review workload
needs Kafka or Kubernetes.

## Run

Run from the repository root after installing the development requirements in
the root README:

```bash
optimizer=tools/codex-plugins/agent-review-optimizer/scripts/review-optimizer
untracked_scope=path/to/in-scope-file-or-dir

{
  git diff --no-renames --name-only -z origin/main...HEAD
  git diff --no-renames --name-only -z
  git diff --cached --no-renames --name-only -z
  git ls-files --others --exclude-standard -z -- "$untracked_scope"
} | "$optimizer" route --null
PYTHON_BIN="$PWD/.venv/bin/python" \
  tools/codex-plugins/agent-review-optimizer/scripts/quality.sh
```

Replace `untracked_scope` with the current task's narrow path; add explicit
pathspecs when more than one untracked root is in scope.

`route` emits only the changed-file count and selected reviewer categories. It
does not emit paths.

To sanitize an existing private `codex exec --json` stream, supply opaque UUIDv4
identifiers and adjudicated outcome counts:

```bash
"$optimizer" codex-run \
  --run-id 11111111-1111-4111-8111-111111111111 \
  --invocation-id 22222222-2222-4222-8222-222222222222 \
  --occurred-at 2026-08-01T17:00:00Z \
  --strategy routed \
  --reviewer general \
  --duration-ms 1000 \
  --changed-file-count 4 \
  --proposed-findings 1 \
  --confirmed-findings 1 \
  --false-positive-findings 0 \
  < /private/path/codex-review.jsonl
```

Raw JSONL can contain source, commands, messages, paths, and reasoning. Keep it
outside Git with restrictive access and retention, or stream it directly to the
adapter. The adapter excludes that content from its output, but it does not
change Codex's normal local session persistence. Use the `--ephemeral` flag with
`codex exec --json` when the source session itself must not be retained.

Pipe newline-delimited sanitized invocation records to `review-optimizer
summarize`. Cohorts are grouped by strategy, reviewer, provider, and model. The
report includes confirmed and false-positive rates, duration, zero-yield calls,
token categories, usage-measured invocation counts, and uncached-plus-output
tokens per confirmed finding. These are descriptive reviewer-level cohorts, not
team-level strategy comparisons: v1 cannot represent duplicate findings across
reviewers or prove that strategies saw the same corpus. A cohort containing
unmeasured failed usage reports its token totals as `null` rather than a
misleading partial sum.

## Record contract

`ReviewInvocation` v1 rejects unknown fields, noncanonical or non-v4 IDs,
negative counters, cached input greater than input, unsupported dimensions,
duplicate invocation IDs during aggregation, incomplete completed adjudication,
and findings attached to failed runs. Its allowlist contains:

- opaque run and invocation UUIDv4 values;
- UTC event time, provider, optional model, strategy, reviewer, and status;
- duration, changed-file count, and adjudicated aggregate finding counts; and
- input, cached-input, and output token counts; nullable reasoning-output tokens;
  or `null` usage when a failed Codex terminal event supplies no counters.

It contains no prompt, response, source, diff, comment, command, path,
repository, identity, session, credential, or financial field. Provider quota
percentages and estimated costs are intentionally not normalized across vendors.
The adapter sanitizes raw JSONL, not caller-supplied CLI metadata. Supply only a
public provider model identifier to `--model`, or omit it, and treat exact
timestamps, opaque IDs, durations, file counts, and token counters as
pseudonymous telemetry rather than anonymous or aggregate-only data.

The Codex adapter follows the documented `codex exec --json` terminal contract
and is tested with Codex CLI 0.146.0. It accepts older streams that omit
`reasoning_output_tokens` without fabricating zero and fails closed when the
three core usage counters or a terminal event are absent.

## Next slices

1. Build a versioned seeded-defect corpus with unique adjudicated finding IDs,
   paired strategy membership, and total-team aggregates before comparing
   deterministic-only, broadcast, and routed strategies.
2. Add a Claude Code adapter that passes the same conformance and privacy tests,
   versioning the record contract when its nullable measurements require it.
3. Publish only the sanitized contract as a separate Kafka producer, then add
   Kubernetes scaling and Grafana views after local measurements justify them.
4. Keep routing advisory until critical/high recall matches broadcast review and
   usage falls without increasing false positives or human review time.

No token savings, defect-recall improvement, production deployment, or cloud
runtime result has been measured yet.
