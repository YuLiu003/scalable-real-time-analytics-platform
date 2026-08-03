# Private market feed contract

## Scope

The optional `alpaca-market-producer` turns a runtime-selected Alpaca stock or
ETF watchlist into the existing `market.price.observed` v1 Kafka contract. The
default free-compatible feed is IEX. The adapter is not deployed by the normal
local or PS2 workflow because CI has no provider credentials or private
watchlist. Its runtime watchlist is capped at 30 instruments; synthetic traffic,
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
demo holdings. This slice proves live ingestion through private bronze storage;
it does not yet expose private feed data in the unauthenticated dashboard.

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
idempotence within one producer session and records remain ordered per
instrument. An ambiguous acknowledgement across a restart can still replay the
same stable ID; the immutable archive treats that as a duplicate delivery
rather than a second durable effect.

## Gap recovery

1. Connect, authenticate, and require an exact watchlist subscription
   acknowledgement.
2. Start a bounded live-message buffer.
3. Read the last Kafka-acknowledged timestamp per instrument from a private
   mode-`0600` checkpoint.
4. Backfill from the earliest checkpoint minus a two-minute inclusive overlap;
   a new watchlist starts with a bounded seven-day lookup.
5. Publish backfill records in event-time order, then drain and continue the
   live stream.
6. Advance cursors only after Kafka acknowledges each event. Persist one atomic
   checkpoint after a historical batch and after each live bar; a mid-backfill
   crash therefore replays stable IDs instead of losing records.

Disconnects, HTTP `429`, provider `500`, and slow-client failures reconnect with
bounded full-jitter backoff. Invalid credentials, invalid subscriptions, and
unsupported feeds fail closed. Readiness stays false until authentication,
subscription, and backfill succeed. A full live buffer forces reconnect and
backfill instead of silently dropping records.

The delivery boundary is at-least-once with idempotent durable effects, not an
exactly-once claim. Alpaca trade cancellations cannot be represented by the v1
price-observation contract. Late updated bars are ingested, but this stream is
not an authoritative consolidated tape or a tax/performance ledger.

## Privacy and operations

- `APCA_API_KEY_ID`, `APCA_API_SECRET_KEY`, `MARKET_WATCHLIST`, and
  `MARKET_TENANT_ID` come only from the `alpaca-market-feed` runtime Secret.
- The checkpoint lives on the private feed PVC and includes no credential.
- Logs and Prometheus metrics contain aggregate state and counters without
  instrument, tenant, event-ID, or credential labels.
- Canonical Kafka and bronze records necessarily contain the selected
  instrument and tenant. They are private workload data and must not be
  attached to CI artifacts or committed.
- Only approved Alpaca hosts are allowed to receive credentials; plaintext
  endpoints are accepted only on loopback for tests, and provider redirects
  are rejected before authentication data can cross that allowlist.

Tests use fictional symbols and local fake WebSocket/HTTP servers. A real
provider smoke test is optional and must remain local and redacted.
