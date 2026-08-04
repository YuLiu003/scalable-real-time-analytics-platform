# Security boundaries

Security review requirements for every change are defined in the
[presubmit contract](engineering/presubmit-gates.md).

## Repository data policy

- Do not commit brokerage credentials, account identifiers, personal exports,
  cloud credentials, kubeconfigs, private keys, Terraform state, or secrets.
- Use only synthetic or explicitly redacted portfolio fixtures.
- Keep paid cloud applies opt-in and separate from routine CI.
- Keep credentials out of untrusted pull-request pipelines and container logs.

## Implemented controls

- GitHub workflows use least-privilege permissions and commit-pinned actions.
- Jenkins validates the exact pull-request commit on isolated Kubernetes agents.
- Application pods run as non-root with restricted security contexts.
- Kubernetes namespaces, service accounts, RBAC, quotas, and network policies
  separate application, data, observability, and CI responsibilities.
- Kafka clients use TLS and Strimzi-managed identities.
- AWS desired state uses workload identity, KMS encryption, private networking,
  bounded budgets, and no static access keys.
- Failure diagnostics exclude Kubernetes Secret objects, secret values, and
  archived portfolio contents.
- The single-user private portfolio workflow validates owner-only inputs
  outside Git, mounts them as separate runtime Secrets, redacts analytics logs,
  and protects allocation responses with an exact bearer token and `no-store`.
- The private verifier refuses to replace an existing private runtime and uses
  fictional records only. Its resources are deleted after the acceptance.

## Unsupported claims

The local environment does not prove public-edge TLS, enterprise SSO, cloud IAM
propagation, managed-service recovery, multi-region disaster recovery, or
continuous production operations. Those require separately authorized
environments and evidence.

Bearer protection on a loopback port-forward is not enterprise identity or a
public-edge security boundary. Kafka and Garage are shared local storage. After
private workloads are removed, Kafka can retain private tenant, symbol, and
price events; Garage can retain derived quantities, position values and
allocations, benchmark data, and total portfolio valuation. The private
workflow documents the separate destructive confirmation required to purge all
local market and analytical data.

## Reporting

Report suspected vulnerabilities privately to `yuuliu03@gmail.com`. Do not open
a public issue containing credentials, personal financial data, or exploit
details.
