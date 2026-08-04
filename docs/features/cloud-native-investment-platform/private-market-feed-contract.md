# Private market feed contract

## Scope

The optional `alpaca-market-producer` turns a runtime-selected Alpaca stock or
ETF watchlist into the existing `market.price.observed` v1 Kafka contract. The
default free-compatible feed is IEX. The real adapter is not deployed by the
credential-free PS2 workflow because CI has no provider credentials or private
watchlist; PS2 instead uses a fictional producer through the identical event
contract. The runtime watchlist is capped at 30 instruments. Synthetic traffic,
not a provider subscription, remains the capacity-test input.

Alpaca documents the WebSocket authentication, subscription, batching, and
error protocol in its
[streaming guide](https://docs.alpaca.markets/us/docs/streaming-market-data).
The adapter subscribes to one-minute `bars` and `updatedBars`, whose fields and
late-update behavior are documented in the
[real-time stock-data guide](https://docs.alpaca.markets/us/docs/real-time-stock-pricing-data).
It uses the paginated
[historical bars API](https://docs.alpaca.markets/us/v1.4.2/reference/stockbars)
to close bounded disconnect gaps.

This provider covers supported US stocks and ETFs. It does not provide mutual-
fund NAVs or direct index levels, so those inputs remain outside this adapter.
The default portfolio analytics Job remains pinned to the synthetic source and
demo holdings. The opt-in private workflow separately joins source
`alpaca-iex` with runtime-only stock/ETF holdings and serves the result through
a bearer-protected API. The workflow and its cleanup boundary are defined in
the [`private portfolio runbook`](private-portfolio-workflow.md).

## Event identity and ordering

Each provider bar maps to one canonical envelope:

| Canonical field | Value |
| --- | --- |
| Kafka key / `partition_key` | Instrument symbol |
| `event_id` | SHA-256 identity over source, tenant, instrument, bar start, trade count, and canonical close |
| `occurred_at` | End of the provider's one-minute interval |
| `ingested_at` | The same deterministic interval end |
| `payload.price` | Bar close as a canonical fixed-point decimal string |
| `payload.provider_sequence` | Provider trade count for the bar |

Using the interval end for both timestamps makes a live delivery and a later
historical backfill byte-identical. The Kafka producer enables protocol-level
idempotence within one producer session, and Kafka preserves append order for
each instrument key. Event time can regress when a reconnect republishes the
two-minute overlap after newer appended records; `provider_sequence` is the
bar's trade count, not a globally increasing stream sequence. An ambiguous
acknowledgement across a restart can still replay the same stable ID; the
immutable archive treats that as a duplicate delivery rather than a second
durable effect.

## Gap recovery

1. Connect, authenticate, and require an exact watchlist subscription
   acknowledgement.
2. Start a bounded live-message buffer.
3. Read the last Kafka-acknowledged timestamp per instrument from a private
   mode-`0600` checkpoint. Checkpoint schema v2 binds a non-reversible SHA-256
   scope to tenant, source, and feed. A schema-v1 checkpoint is atomically
   replaced with an empty scoped checkpoint and triggers the bounded seven-day
   replay; stable event IDs make already archived records duplicates rather
   than second durable effects. Other schema or scope mismatches fail closed.
4. Backfill from the earliest checkpoint minus a two-minute inclusive overlap;
   a new watchlist starts with a bounded seven-day lookup.
5. Publish backfill records in event-time order, then drain and continue the
   live stream.
6. Prune cursors outside the active watchlist, then advance remaining cursors
   only after Kafka acknowledges each event. Persist one atomic checkpoint
   after pruning, after a historical batch, and after each live bar; a
   mid-backfill crash therefore replays stable IDs instead of losing records.

Disconnects, HTTP `429`, provider `500`, and slow-client failures reconnect with
bounded full-jitter backoff. Invalid credentials, invalid subscriptions, and
unsupported feeds fail closed. Readiness stays false until authentication,
subscription, and backfill succeed. A full live buffer forces reconnect and
backfill instead of silently dropping records.

The delivery boundary is at-least-once with idempotent durable effects, not an
exactly-once claim. Alpaca trade cancellations cannot be represented by the v1
price-observation contract. Late updated bars are ingested, but this stream is
not an authoritative consolidated tape or a tax/performance ledger.

Private current-allocation analytics is bounded separately from archive
retention. It reads the two latest UTC date partitions containing its source,
then keeps only the latest required observation per holding and benchmark.
Older bronze records remain available for replay, but are not copied into every
five-minute private silver product.

## Privacy and operations

- `APCA_API_KEY_ID`, `APCA_API_SECRET_KEY`, `MARKET_WATCHLIST`, and
  `MARKET_TENANT_ID` come only from the `alpaca-market-feed` runtime Secret.
- Private holdings and the dashboard token come from separate runtime Secrets;
  the private API allocation endpoint requires an exact bearer token and sends
  `no-store` responses.
- The checkpoint lives on the private feed PVC and includes no credential.
- Logs and Prometheus metrics contain aggregate state and counters without
  instrument, tenant, event-ID, or credential labels.
- Canonical Kafka and bronze records necessarily contain the selected
  instrument and tenant. They are private workload data and must not be
  attached to CI artifacts or committed.
- Only approved Alpaca hosts are allowed to receive credentials; plaintext
  endpoints are accepted only on loopback for tests, and provider redirects
  are rejected before authentication data can cross that allowlist.

Tests use fictional symbols and local fake WebSocket/HTTP servers. The
credential-free Kubernetes acceptance proves wiring, not provider behavior. A
real provider smoke test is optional and must remain local and redacted.
