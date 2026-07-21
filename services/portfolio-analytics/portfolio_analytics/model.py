from __future__ import annotations

import hashlib
import json
import re
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal, InvalidOperation
from typing import Any, Iterable

EVENT_KEYS = {
    "event_id",
    "event_type",
    "schema_version",
    "source",
    "tenant_id",
    "occurred_at",
    "ingested_at",
    "partition_key",
    "trace_id",
    "payload",
}
PAYLOAD_KEYS = {"instrument", "currency", "price", "provider_sequence"}
HOLDINGS_KEYS = {"schema_version", "portfolio_id", "base_currency", "positions"}
POSITION_KEYS = {"instrument", "quantity"}

EVENT_ID = re.compile(r"^[a-z0-9][a-z0-9._:-]{0,127}$")
SOURCE = re.compile(r"^[a-z0-9][a-z0-9._-]{0,31}$")
TENANT = re.compile(r"^[a-z0-9][a-z0-9._-]{0,63}$")
INSTRUMENT = re.compile(r"^[A-Z0-9][A-Z0-9.-]{0,14}$")
CURRENCY = re.compile(r"^[A-Z]{3}$")
DECIMAL_VALUE = re.compile(r"^(0|[1-9][0-9]*)(\.[0-9]{1,8})?$")
TRACE_ID = re.compile(r"^[0-9a-f]{32}$")


@dataclass(frozen=True)
class BronzeObject:
    key: str
    data: bytes


@dataclass(frozen=True)
class PriceObservation:
    event_id: str
    source: str
    tenant_id: str
    occurred_at: datetime
    ingested_at: datetime
    instrument: str
    currency: str
    price: Decimal
    provider_sequence: int
    object_key: str


@dataclass(frozen=True)
class Portfolio:
    portfolio_id: str
    base_currency: str
    positions: tuple[tuple[str, Decimal], ...]


def _strict_object(value: Any, expected: set[str], name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError(f"{name} must be a JSON object")
    actual = set(value)
    if actual != expected:
        missing = sorted(expected - actual)
        unknown = sorted(actual - expected)
        raise ValueError(f"{name} fields mismatch: missing={missing} unknown={unknown}")
    return value


def _timestamp(value: Any, field: str) -> datetime:
    if not isinstance(value, str):
        raise ValueError(f"{field} must be a string")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as error:
        raise ValueError(f"{field} must be RFC3339") from error
    if parsed.tzinfo is None:
        raise ValueError(f"{field} must include a timezone")
    return parsed


def _decimal(value: Any, field: str, *, positive: bool = False) -> Decimal:
    if not isinstance(value, str) or not DECIMAL_VALUE.fullmatch(value):
        raise ValueError(f"{field} must be a non-negative fixed-point decimal string")
    try:
        parsed = Decimal(value)
    except InvalidOperation as error:
        raise ValueError(f"{field} is not a decimal") from error
    if positive and parsed <= 0:
        raise ValueError(f"{field} must be greater than zero")
    return parsed


def parse_price(obj: BronzeObject) -> PriceObservation:
    try:
        value = json.loads(obj.data)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError(f"{obj.key}: invalid JSON") from error
    event = _strict_object(value, EVENT_KEYS, f"{obj.key}: event")
    payload = _strict_object(event["payload"], PAYLOAD_KEYS, f"{obj.key}: payload")

    event_id = event["event_id"]
    if not isinstance(event_id, str) or not EVENT_ID.fullmatch(event_id):
        raise ValueError(f"{obj.key}: invalid event_id")
    schema_version = event["schema_version"]
    if (
        event["event_type"] != "market.price.observed"
        or not isinstance(schema_version, int)
        or isinstance(schema_version, bool)
        or schema_version != 1
    ):
        raise ValueError(f"{obj.key}: unsupported event contract")
    if not isinstance(event["source"], str) or not SOURCE.fullmatch(event["source"]):
        raise ValueError(f"{obj.key}: invalid source")
    if not isinstance(event["tenant_id"], str) or not TENANT.fullmatch(event["tenant_id"]):
        raise ValueError(f"{obj.key}: invalid tenant_id")
    if not isinstance(event["trace_id"], str) or not TRACE_ID.fullmatch(event["trace_id"]) or set(event["trace_id"]) == {"0"}:
        raise ValueError(f"{obj.key}: invalid trace_id")

    instrument = payload["instrument"]
    if not isinstance(instrument, str) or not INSTRUMENT.fullmatch(instrument):
        raise ValueError(f"{obj.key}: invalid instrument")
    if event["partition_key"] != instrument:
        raise ValueError(f"{obj.key}: partition_key does not match instrument")
    currency = payload["currency"]
    if not isinstance(currency, str) or not CURRENCY.fullmatch(currency):
        raise ValueError(f"{obj.key}: invalid currency")
    sequence = payload["provider_sequence"]
    if not isinstance(sequence, int) or isinstance(sequence, bool) or sequence < 0:
        raise ValueError(f"{obj.key}: invalid provider_sequence")
    occurred_at = _timestamp(event["occurred_at"], f"{obj.key}: occurred_at")
    ingested_at = _timestamp(event["ingested_at"], f"{obj.key}: ingested_at")
    if ingested_at < occurred_at:
        raise ValueError(f"{obj.key}: ingested_at precedes occurred_at")

    return PriceObservation(
        event_id=event_id,
        source=event["source"],
        tenant_id=event["tenant_id"],
        occurred_at=occurred_at,
        ingested_at=ingested_at,
        instrument=instrument,
        currency=currency,
        price=_decimal(payload["price"], f"{obj.key}: price"),
        provider_sequence=sequence,
        object_key=obj.key,
    )


def parse_portfolio(data: bytes) -> Portfolio:
    try:
        value = json.loads(data)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError("holdings fixture is invalid JSON") from error
    holdings = _strict_object(value, HOLDINGS_KEYS, "holdings")
    schema_version = holdings["schema_version"]
    if not isinstance(schema_version, int) or isinstance(schema_version, bool) or schema_version != 1:
        raise ValueError("holdings schema_version must be 1")
    portfolio_id = holdings["portfolio_id"]
    if not isinstance(portfolio_id, str) or not TENANT.fullmatch(portfolio_id):
        raise ValueError("holdings portfolio_id is invalid")
    currency = holdings["base_currency"]
    if not isinstance(currency, str) or not CURRENCY.fullmatch(currency):
        raise ValueError("holdings base_currency is invalid")
    raw_positions = holdings["positions"]
    if not isinstance(raw_positions, list) or not raw_positions:
        raise ValueError("holdings positions must be a non-empty array")

    positions: list[tuple[str, Decimal]] = []
    seen: set[str] = set()
    for index, raw in enumerate(raw_positions):
        position = _strict_object(raw, POSITION_KEYS, f"holdings position {index}")
        instrument = position["instrument"]
        if not isinstance(instrument, str) or not INSTRUMENT.fullmatch(instrument):
            raise ValueError(f"holdings position {index} instrument is invalid")
        if instrument in seen:
            raise ValueError(f"holdings contains duplicate instrument {instrument}")
        seen.add(instrument)
        positions.append((instrument, _decimal(position["quantity"], f"holdings {instrument} quantity", positive=True)))
    return Portfolio(portfolio_id, currency, tuple(sorted(positions)))


def input_set_sha256(holdings_data: bytes, objects: Iterable[BronzeObject]) -> str:
    digest = hashlib.sha256()
    digest.update(b"holdings\0")
    digest.update(hashlib.sha256(holdings_data).digest())
    for obj in sorted(objects, key=lambda item: item.key):
        digest.update(b"\0object\0")
        digest.update(obj.key.encode("utf-8"))
        digest.update(b"\0")
        digest.update(hashlib.sha256(obj.data).digest())
    return digest.hexdigest()


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode("utf-8") + b"\n"
