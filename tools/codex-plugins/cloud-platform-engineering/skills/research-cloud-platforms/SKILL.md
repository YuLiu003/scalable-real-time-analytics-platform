---
name: research-cloud-platforms
description: Research current Kubernetes, AWS, GCP, Azure, Terraform, cloud-native, observability, and streaming behavior from authoritative documentation. Use when answering version-sensitive technical questions, comparing cloud services, validating architecture claims, finding exact configuration guidance, or producing evidence-backed recommendations with links.
---

# Research Cloud Platforms

Use the `cloud-docs` MCP tools to discover and retrieve current official pages. Do not rely on search snippets as evidence.

## Research workflow

1. State the decision or question precisely, including provider, version, region, workload, and constraints when known.
2. Call `catalog_sources` when the relevant authority is unclear.
3. Call `search_official_docs` with a narrow query and explicit `source_ids` for provider-specific questions.
4. Call `fetch_official_doc` for every page used to support a claim. Use `build_evidence_pack` only for a compact initial survey.
5. Separate documented facts, architectural inference, and recommendations.
6. Record version, publication/update date when visible, retrieval time, and unresolved ambiguity.
7. Link the exact supporting pages near the claims they support.

Prefer upstream project documentation for portable behavior and cloud-provider documentation for managed-service behavior. When they disagree, explain the product/version boundary instead of blending the guidance.

Read [evidence-standard.md](references/evidence-standard.md) before producing a comparison, architecture decision, security recommendation, or production runbook.
