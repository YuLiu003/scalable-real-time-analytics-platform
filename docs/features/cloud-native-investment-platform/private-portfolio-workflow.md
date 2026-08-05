# Private stock/ETF portfolio workflow

## Supported product

This workflow turns a user-selected Alpaca stock/ETF watchlist and matching
holdings into a private current-allocation snapshot and contribution
projections. It exercises the same live adapter, Kafka, immutable object
archive, DuckDB analytics, Kubernetes Jobs, and Go API used by the public
fictional acceptance path.

It is a single-user local workflow. It does not place trades, recommend
investments, calculate transaction-grounded historical returns, or ingest
mutual-fund NAVs or direct index levels. The contribution calculator is a
hypothetical model whose return, fee, inflation, amount, frequency, and horizon
assumptions remain visible to the user.

## Private inputs

Create three absolute-path regular files outside the repository. Each file must
be readable only by its owner (`chmod 600`). Never commit these files or attach
their contents to build artifacts.

The feed environment file contains exactly four keys:

```text
APCA_API_KEY_ID=<provider key>
APCA_API_SECRET_KEY=<provider secret>
MARKET_WATCHLIST=<comma-separated stock/ETF symbols>
MARKET_TENANT_ID=private
```

The holdings file uses the v2 portfolio schema. Every position and the
benchmark must use a symbol in `MARKET_WATCHLIST`, `asset_type` must be `stock`
or `etf`, and `valuation_type` must be `market_price`. The portfolio ID is
literally `private` and the base currency is USD. Use the fictional
[`demo-private-portfolio.v2.json`](../../../contracts/fixtures/demo-private-portfolio.v2.json)
only as a structural example; replace its symbols, quantities, and labels in
the private file outside Git.

Generate a separate dashboard token containing 32 to 512 bearer-safe ASCII
characters (`A-Z`, `a-z`, `0-9`, `-._~+/`, with optional trailing `=`), for
example:

```bash
umask 077
openssl rand -hex 32 > /absolute/path/outside/repository/portfolio.token
```

The preflight validator rejects symlinks, permissive file modes, files inside
the repository, mismatched watchlists, unsupported asset types, and malformed
tokens. Its output contains only aggregate counts or a generic error.

## Start

Use persistent local mode because a live feed continues until explicitly
stopped:

```bash
PRIVATE_FEED_ENV_FILE=/absolute/path/outside/repository/alpaca-market-feed.env \
PRIVATE_HOLDINGS_FILE=/absolute/path/outside/repository/portfolio.json \
PRIVATE_ACCESS_TOKEN_FILE=/absolute/path/outside/repository/portfolio.token \
  make -C platform/local bootstrap-private-portfolio
```

The command creates or reuses the local kind data path, injects the three files
as Kubernetes Secrets, starts the live adapter, waits for Kafka archive lag to
drain, publishes the first private allocation, enables five-minute refreshes,
and waits for the protected API. Secret values and selected instruments are not
printed. On a rerun, periodic analytics is suspended and existing private
analytics Jobs are removed before input rotation, so an old build cannot
overwrite the newly published `latest.json` result.

Each private refresh discovers date partitions through the S3 index, downloads
only the two most recent UTC partitions that contain the selected source, and
publishes only the latest required observation per position and benchmark. The
bound prevents an every-five-minute current-allocation job from rescanning and
rewriting all retained history. It is intentionally not a historical-return
dataset.

Open a loopback-only dashboard connection in a separate terminal:

```bash
make -C platform/local private-portfolio-dashboard
```

Visit `http://127.0.0.1:8080`, then paste the dashboard token when prompted.
The browser keeps the token only in page memory; it is not placed in local
storage, a cookie, or the URL. Closing the page clears it.

Changes to credentials, watchlist, holdings, or token require rerunning the
bootstrap command. The adapter checkpoint is scoped to the tenant, source, and
feed, and contains only active-watchlist cursors. Existing unscoped schema-v1
state is safely reset to a scoped checkpoint and rebuilt through the bounded
replay on the first upgraded start.

## Stop and data retention

Remove the private workloads, private Secrets, and adapter checkpoint with an
exact confirmation:

```bash
CONFIRM_DESTROY_PRIVATE_PORTFOLIO=private-portfolio \
  make -C platform/local destroy-private-portfolio
```

Kafka and Garage are shared with the public local data path. After the normal
destroy command, Kafka market events can retain the private tenant and selected
symbols. Garage can also retain immutable and latest analytical products with
private quantities, per-position values and allocations, the benchmark, and
the total portfolio valuation. Purging that shared storage also deletes all
local market and analytics data and therefore requires a second explicit
confirmation:

```bash
PURGE_ALL_LOCAL_MARKET_DATA=true \
CONFIRM_PURGE_ALL_LOCAL_MARKET_DATA=all-local-market-data \
CONFIRM_DESTROY_PRIVATE_PORTFOLIO=private-portfolio \
  make -C platform/local destroy-private-portfolio
```

To reclaim all Docker disk used by the persistent development environment,
follow the confirmed Colima cleanup procedure in the
[`local platform runbook`](../../../platform/local/README.md#persistent-development-workflow).

## Verification and evidence boundary

`make presubmit-ps2` runs a credential-free Kubernetes acceptance with
fictional symbols through the canonical event contract. It proves the private
analytics and authorization wiring without contacting Alpaca or exposing a
real watchlist. Synthetic acceptance, fake-provider tests, and local kind
results are not evidence of a live provider subscription, AWS deployment, or
production use. A real credentialed smoke remains a private operator action.
