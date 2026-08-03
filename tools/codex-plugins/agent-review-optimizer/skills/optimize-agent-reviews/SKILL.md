---
name: optimize-agent-reviews
description: Route code changes to applicable specialist reviewers and produce privacy-minimized reviewer-level usage summaries. Use for Codex CLI code reviews, reviewer selection, sanitized Codex JSONL usage extraction, false-positive analysis, or descriptive review-efficiency cohorts.
---

# Optimize Agent Reviews

Use the deterministic analyzer at `<plugin-root>/scripts/review-optimizer`, where
`<plugin-root>` is three parent directories above this file.

## Workflow

1. Establish the review base. Follow the repository's documented gate policy;
   otherwise run the smallest affected test or static check before spending model
   tokens. The optimizer itself has no runtime dependency beyond Python.
2. Route committed, staged, unstaged, and intended untracked paths. Check
   `git status --short` first. Name task-relevant untracked pathspecs explicitly;
   do not include or exclude an untracked directory solely because of its name.
   Then pipe this NUL-delimited stream to `review-optimizer route --null`;
   `--no-renames` exposes both sides of a rename and duplicate paths are removed:

   ```bash
   {
     git diff --no-renames --name-only -z "$base"...HEAD
     git diff --no-renames --name-only -z
     git diff --cached --no-renames --name-only -z
     git ls-files --others --exclude-standard -z -- path/to/in-scope-file-or-dir
   } | "$optimizer" route --null
   ```

   Do not save the path list in telemetry. When modifying this plugin, install
   `requirements-dev.txt` into an isolated environment and run `scripts/quality.sh`;
   ordinary routing and analysis do not require `coverage`.
3. Always use one general correctness reviewer. Add only the specialists returned
   by the router. Keep reviewers read-only and give each a bounded question.
4. Ask reviewers for evidence-backed findings with file and line references.
   Deduplicate candidates, independently verify material findings, and leave the
   author or another human as the final decision maker.
5. Record one allowlisted invocation per reviewer. For `codex exec --json`, pipe
   JSONL through `review-optimizer codex-run`; use `codex exec --ephemeral --json`
   when Codex must not persist its normal local session files. The adapter derives
   completion or failure from the terminal event, copies only documented token
   counts when present, and excludes messages, commands, reasoning, and paths
   from its output. It does not control Codex session persistence.
6. Pipe allowlisted invocation records to `review-optimizer summarize`. Treat the
   reviewer-level cohorts as descriptive observations. Do not sum findings across
   reviewers or compare strategies because duplicate defects and paired-corpus
   membership are not represented by v1.
7. Do not claim the routed strategy is better until a versioned benchmark adds
   unique seeded-defect adjudication, matches defect recall, and reduces total
   team usage on the same corpus.

## Routing policy

- `general`: every change.
- `go` or `python`: matching application language.
- `event-driven`: Kafka, Strimzi, or event-contract changes.
- `kubernetes`: GitOps, Kubernetes, and workload manifests.
- `cloud-iac`: OpenTofu and Terraform changes.
- `ci-security`: Jenkins, CI scripts, and workflow changes.
- `privacy-finance`: portfolio, investment, market, or public-fixture changes.

Run specialists in parallel only when their scopes are independent. Do not send
every change to every specialist.

## Privacy boundary

- Keep prompts, responses, source, diffs, comments, commands, paths, repository
  names, emails, account IDs, session IDs, credentials, and financial data out of
  sanitized records.
- Use opaque canonical UUIDs for run and invocation IDs.
- Keep raw review artifacts local, access-controlled, ignored by Git, and subject
  to explicit retention. Never publish them to Kafka or object storage.
- Keep provider cost and quota fields provider-specific. Missing fields are
  absent or `null`, never fabricated as zero.
- Treat CLI metadata as trusted input. Use only a public provider model ID in
  `--model`, or omit it; allowlisting field names cannot detect sensitive text
  placed inside an accepted value.

The v1 record accepts only Codex and the token-free deterministic baseline.
Reasoning-token usage is nullable for Codex streams that do not emit it. Claude
Code is not plug-and-play in v1; add it as a separate adapter and version the
contract if its nullable fields cannot be represented without ambiguity.
