# Portfolio API

This stateless Go service reads the latest canonical portfolio-allocation JSON
from object storage, validates it strictly, and exposes it through an API and a
small embedded dashboard.

The current dashboard is API-driven and clearly labeled as synthetic. It shows
QQQ, QQQM, and FSELX as illustrative holdings and SP500 as a benchmark. The
result contract distinguishes ETF market prices, mutual-fund NAV, and an index
level instead of presenting all four values as equivalent tradable positions.

| Endpoint | Contract |
| --- | --- |
| `/healthz` | Process liveness; independent of object storage |
| `/readyz` | Ready only when the latest allocation can be fetched and validated |
| `/api/v1/portfolios/demo/allocation` | Canonical deterministic allocation JSON |
| `/` | Browser dashboard |

The service holds no portfolio state and uses no SQL or cache database. Derived
state remains rebuildable from bronze objects through the Slice 3 analytics Job.

The repository quality gate runs all internal API packages under the Go race
detector and fails unless their combined statement coverage is exactly 100%:

```bash
PYTHON_BIN=.venv/bin/python make -C platform/local quality
```
