# Evidence Standard

## Authority order

1. Normative specifications and upstream project documentation.
2. Cloud-provider service documentation, API references, and architecture centers.
3. Official project release notes, security advisories, and maintained examples.
4. Official engineering blogs when documentation does not cover the behavior.
5. Secondary sources only for discovery or clearly labeled operational experience.

## Required distinctions

- Fact: directly supported by a retrieved source.
- Inference: follows from multiple facts or from applying them to the stated context.
- Recommendation: a tradeoff-driven choice; state the constraints that make it appropriate.
- Unknown: evidence is absent, stale, contradictory, or outside the documented scope.

## Freshness checks

- Verify the product version and documentation channel (`latest`, stable, or versioned).
- Check deprecations, end-of-support dates, and migration notes.
- Treat prices, quotas, regions, service availability, APIs, and security guidance as time-sensitive.
- Prefer exact service pages over broad marketing pages.
- Fetch each page on the current turn; do not cite a search result snippet.

## Comparison discipline

- Compare equivalent responsibility boundaries: managed control plane versus self-managed, regional versus zonal, and service tier versus service tier.
- Normalize dimensions: availability, scaling, consistency, networking, identity, encryption, observability, operations, lock-in, and cost model.
- Do not invent a winner without workload constraints.
