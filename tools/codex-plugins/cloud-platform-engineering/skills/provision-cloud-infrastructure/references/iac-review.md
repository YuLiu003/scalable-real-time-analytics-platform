# Infrastructure as Code Review

## State and lifecycle

- Use a remote, encrypted backend with locking and restricted access for shared environments.
- Keep state scopes small enough to limit blast radius and ownership ambiguity.
- Pin tool, provider, module, and artifact versions; commit dependency locks.
- Model imports, moves, replacements, and removals explicitly.
- Protect critical data and control-plane resources from accidental destruction.

## Cloud foundation

- Spread subnets and dependencies across required failure domains.
- Distinguish public ingress, private workloads, controlled egress, and service endpoints.
- Prefer federated CI identity and workload identity over stored access keys.
- Encrypt in transit and at rest with defined key ownership and rotation.
- Define DNS, certificates, logs, metrics, audit trails, backup, restore, and retention.

## Module quality

- Make invalid states difficult with typed variables, validation, and safe defaults.
- Expose intentional interfaces; avoid pass-through modules that hide no complexity.
- Include outputs needed by downstream scopes without leaking secrets.
- Test invariants and important conditional paths.
- Document operational consequences in code comments only where names and types cannot.

## Plan review

Classify every change as create, in-place update, replacement, destroy, state-only, or unknown. Call out security exposure, downtime, data risk, dependency order, quota, rollback, and recurring-cost changes.
