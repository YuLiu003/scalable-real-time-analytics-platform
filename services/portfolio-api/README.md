# Portfolio API

This stateless Go service reads the latest canonical portfolio-allocation JSON
from object storage, validates it strictly, and exposes it through an API and a
small embedded dashboard.

It also exposes a stateless contribution-projection endpoint. The calculation
contract is documented in the
[`contribution projection contract`](../../docs/features/cloud-native-investment-platform/contribution-projection-contract.md).

The dashboard discovers its configured portfolio and data mode through the
API. Public mode is clearly labeled synthetic. Private mode prompts for a
bearer token kept only in page memory and supports stock/ETF market-price
positions and benchmarks. The shared result contract still distinguishes ETF
market prices, mutual-fund NAV, and index levels for the public fixture.

| Endpoint | Contract |
| --- | --- |
| `/healthz` | Process liveness; independent of object storage |
| `/readyz` | Ready only when the latest allocation can be fetched and validated |
| `/api/v1/config` | Configured portfolio ID, data mode, and token requirement; never holdings |
| `/api/v1/portfolios/{portfolio}/allocation` | Canonical allocation JSON; exact bearer authentication when configured |
| `POST /api/v1/projections/contributions` | Hypothetical monthly/biweekly contribution scenarios |
| `/` | Browser dashboard |

The service holds no portfolio state and uses no SQL or cache database. Derived
state remains rebuildable from bronze objects through the Slice 3 analytics Job.
Projection assumptions and results are calculated per request and are not
stored.

Private deployment configuration reads a 32-to-512-character bearer-safe ASCII
token from the absolute path in `PORTFOLIO_ACCESS_TOKEN_FILE`. It never accepts the token
itself through an environment variable, and startup fails closed when a
non-demo portfolio has no token. Allocation and configuration responses use
`Cache-Control: no-store`; health and readiness reveal no portfolio data. The
API is still a single-user local boundary, not a multi-user identity or
internet-facing authorization system.

The offline cash-flow ledger is intentionally not served here because it is not
yet consumed by transaction-grounded performance analytics.

The repository quality gate runs all internal API packages under the Go race
detector and fails unless their combined statement coverage is exactly 100%:

```bash
PYTHON_BIN=.venv/bin/python make -C platform/local quality
```
