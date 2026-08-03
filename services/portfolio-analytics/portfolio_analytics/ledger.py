from __future__ import annotations

import csv
import io
import json
import re
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, localcontext
from typing import Any

from .model import canonical_json

INPUT_FIELDS = ("transaction_id", "occurred_at", "cash_flow_type", "amount", "currency")
TRANSACTION_ID = re.compile(r"^[a-z0-9][a-z0-9._:-]{0,127}$")
PORTFOLIO_ID = re.compile(r"^[a-z0-9][a-z0-9._-]{0,63}$")
CURRENCY = re.compile(r"^[A-Z]{3}$")
AMOUNT = re.compile(r"^(0|[1-9][0-9]*)(\.[0-9]{1,8})?$")
RFC3339 = re.compile(
    r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}"
    r"(?:\.[0-9]{1,6})?(?:Z|[+-][0-9]{2}:[0-9]{2})$"
)
CASH_FLOW_TYPES = {"deposit", "withdrawal"}


@dataclass(frozen=True)
class CashFlow:
    transaction_id: str
    occurred_at: datetime
    cash_flow_type: str
    amount: Decimal
    currency: str


def _parse_timestamp(value: Any) -> datetime:
    if not isinstance(value, str):
        raise ValueError("occurred_at must be a string")
    if not RFC3339.fullmatch(value):
        raise ValueError("occurred_at must be RFC3339 with at most 6 fractional digits")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as error:
        raise ValueError("occurred_at must be RFC3339") from error
    return parsed


def _parse_row(value: Any, row_number: int) -> CashFlow:
    if not isinstance(value, dict):
        raise ValueError(f"row {row_number} must be an object")
    if set(value) != set(INPUT_FIELDS):
        missing = sorted(set(INPUT_FIELDS) - set(value))
        unknown = sorted(set(value) - set(INPUT_FIELDS))
        raise ValueError(f"row {row_number} fields mismatch: missing={missing} unknown={unknown}")

    transaction_id = value["transaction_id"]
    if not isinstance(transaction_id, str) or not TRANSACTION_ID.fullmatch(transaction_id):
        raise ValueError(f"row {row_number} transaction_id is invalid")
    cash_flow_type = value["cash_flow_type"]
    if cash_flow_type not in CASH_FLOW_TYPES:
        raise ValueError(f"row {row_number} cash_flow_type must be deposit or withdrawal")
    amount_text = value["amount"]
    if not isinstance(amount_text, str) or not AMOUNT.fullmatch(amount_text) or Decimal(amount_text) <= 0:
        raise ValueError(f"row {row_number} amount must be a positive decimal with at most 8 decimal places")
    currency = value["currency"]
    if not isinstance(currency, str) or not CURRENCY.fullmatch(currency):
        raise ValueError(f"row {row_number} currency is invalid")
    return CashFlow(
        transaction_id=transaction_id,
        occurred_at=_parse_timestamp(value["occurred_at"]),
        cash_flow_type=cash_flow_type,
        amount=Decimal(amount_text),
        currency=currency,
    )


def _json_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    value = dict(pairs)
    if len(value) != len(pairs):
        raise ValueError("input JSON contains a duplicate object field")
    return value


def _parse_json(data: bytes) -> list[Any]:
    try:
        value = json.loads(data, object_pairs_hook=_json_object)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError("input is invalid JSON") from error
    if not isinstance(value, list):
        raise ValueError("JSON input must be an array")
    return value


def _parse_csv(data: bytes) -> list[Any]:
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as error:
        raise ValueError("input is invalid UTF-8 CSV") from error
    try:
        reader = csv.DictReader(io.StringIO(text, newline=""), strict=True)
        if reader.fieldnames != list(INPUT_FIELDS):
            raise ValueError(f"CSV headers must be exactly {','.join(INPUT_FIELDS)}")
        return list(reader)
    except csv.Error as error:
        raise ValueError("input is invalid CSV") from error


def _decimal_string(value: Decimal) -> str:
    whole, separator, fraction = format(value, "f").partition(".")
    return f"{whole}.{fraction.ljust(8, '0') if separator else '00000000'}"


def _timestamp_string(value: datetime) -> str:
    return value.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def build_ledger(data: bytes, input_format: str, portfolio_id: str, base_currency: str) -> bytes:
    if not PORTFOLIO_ID.fullmatch(portfolio_id):
        raise ValueError("portfolio_id is invalid")
    if not CURRENCY.fullmatch(base_currency):
        raise ValueError("base_currency is invalid")
    if input_format == "json":
        values = _parse_json(data)
    elif input_format == "csv":
        values = _parse_csv(data)
    else:
        raise ValueError("input_format must be json or csv")

    rows = [_parse_row(value, index) for index, value in enumerate(values, start=1)]
    transaction_ids = [row.transaction_id for row in rows]
    if len(transaction_ids) != len(set(transaction_ids)):
        raise ValueError("transaction_id values must be unique")
    unexpected_currencies = sorted({row.currency for row in rows} - {base_currency})
    if unexpected_currencies:
        raise ValueError(f"input currency must match base_currency {base_currency}")

    ordered = sorted(rows, key=lambda row: (row.occurred_at, row.transaction_id))
    maximum_digits = max((len(row.amount.as_tuple().digits) for row in ordered), default=1)
    with localcontext() as context:
        context.prec = maximum_digits + len(str(max(1, len(ordered)))) + 1
        deposits = sum((row.amount for row in ordered if row.cash_flow_type == "deposit"), Decimal("0"))
        withdrawals = sum((row.amount for row in ordered if row.cash_flow_type == "withdrawal"), Decimal("0"))
        net_external_cash_flow = deposits - withdrawals
    ledger = {
        "schema_version": 1,
        "portfolio_id": portfolio_id,
        "base_currency": base_currency,
        "transactions": [
            {
                "transaction_id": row.transaction_id,
                "occurred_at": _timestamp_string(row.occurred_at),
                "cash_flow_type": row.cash_flow_type,
                "amount": _decimal_string(row.amount),
            }
            for row in ordered
        ],
        "totals": {
            "total_deposits": _decimal_string(deposits),
            "total_withdrawals": _decimal_string(withdrawals),
            "net_external_cash_flow": _decimal_string(net_external_cash_flow),
        },
    }
    return canonical_json(ledger)
