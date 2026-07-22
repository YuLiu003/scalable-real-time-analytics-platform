from __future__ import annotations

import json
import unittest
from decimal import Decimal

from portfolio_analytics.model import (
    BENCHMARK_KEYS,
    EVENT_KEYS,
    POSITION_KEYS,
    BronzeObject,
    _decimal,
    _display_name,
    _instrument,
    _strict_object,
    _timestamp,
    canonical_json,
    parse_portfolio,
    parse_price,
)
from test_analytics import HOLDINGS, event


def event_value() -> dict:
    return json.loads(event("QQQ", "600.0000", 1, "2026-07-21T00:00:00Z"))


def holdings_value() -> dict:
    return json.loads(HOLDINGS)


class ModelHelperTests(unittest.TestCase):
    def test_strict_object_requires_exact_object_shape(self) -> None:
        with self.assertRaisesRegex(ValueError, "JSON object"):
            _strict_object([], {"field"}, "value")
        with self.assertRaisesRegex(ValueError, "missing=.*field.*unknown=.*other"):
            _strict_object({"other": True}, {"field"}, "value")
        self.assertEqual(_strict_object({"field": True}, {"field"}, "value"), {"field": True})

    def test_timestamp_requires_rfc3339_timezone(self) -> None:
        for value, message in [(1, "string"), ("not-a-time", "RFC3339"), ("2026-07-21T00:00:00", "timezone")]:
            with self.subTest(value=value), self.assertRaisesRegex(ValueError, message):
                _timestamp(value, "timestamp")
        self.assertEqual(_timestamp("2026-07-21T00:00:00Z", "timestamp").isoformat(), "2026-07-21T00:00:00+00:00")

    def test_decimal_display_name_and_instrument_validation(self) -> None:
        for value in [1, "-1", "1.123456789"]:
            with self.subTest(decimal=value), self.assertRaisesRegex(ValueError, "fixed-point"):
                _decimal(value, "amount")
        with self.assertRaisesRegex(ValueError, "greater than zero"):
            _decimal("0", "amount", positive=True)
        self.assertEqual(_decimal("1.25", "amount", positive=True), Decimal("1.25"))

        for value in [1, " padded", "control\nname"]:
            with self.subTest(display=value), self.assertRaisesRegex(ValueError, "invalid"):
                _display_name(value, "display")
        self.assertEqual(_display_name("S&P 500 Index", "display"), "S&P 500 Index")

        for value in [1, "lowercase", "TOO-LONG-INSTRUMENT"]:
            with self.subTest(instrument=value), self.assertRaisesRegex(ValueError, "invalid"):
                _instrument(value, "instrument")
        self.assertEqual(_instrument("SP500", "instrument"), "SP500")

    def test_canonical_json_is_sorted_ascii_and_newline_terminated(self) -> None:
        self.assertEqual(canonical_json({"z": "é", "a": 1}), b'{"a":1,"z":"\\u00e9"}\n')


class PriceContractTests(unittest.TestCase):
    def assert_invalid(self, mutate, message: str = "") -> None:
        value = event_value()
        mutate(value)
        context = self.assertRaisesRegex(ValueError, message) if message else self.assertRaises(ValueError)
        with context:
            parse_price(BronzeObject("bronze/event.json", json.dumps(value).encode()))

    def test_parse_price_accepts_canonical_event(self) -> None:
        observed = parse_price(BronzeObject("bronze/event.json", event("QQQ", "600.0000", 1, "2026-07-21T00:00:00Z")))
        self.assertEqual((observed.instrument, observed.price, observed.provider_sequence), ("QQQ", Decimal("600.0000"), 1))

    def test_parse_price_rejects_invalid_json_and_shapes(self) -> None:
        for data in [b"{", b"\xff"]:
            with self.subTest(data=data), self.assertRaisesRegex(ValueError, "invalid JSON"):
                parse_price(BronzeObject("bronze/event.json", data))
        with self.assertRaisesRegex(ValueError, "JSON object"):
            parse_price(BronzeObject("bronze/event.json", b"[]"))
        self.assert_invalid(lambda value: value.update({"unknown": True}), "fields mismatch")
        self.assert_invalid(lambda value: value.__setitem__("payload", []), "JSON object")
        self.assert_invalid(lambda value: value["payload"].update({"unknown": True}), "fields mismatch")

    def test_parse_price_rejects_invalid_envelope_fields(self) -> None:
        mutations = [
            (lambda value: value.__setitem__("event_id", 1), "event_id"),
            (lambda value: value.__setitem__("event_id", "INVALID"), "event_id"),
            (lambda value: value.__setitem__("event_type", "unknown"), "contract"),
            (lambda value: value.__setitem__("schema_version", "1"), "contract"),
            (lambda value: value.__setitem__("schema_version", True), "contract"),
            (lambda value: value.__setitem__("schema_version", 2), "contract"),
            (lambda value: value.__setitem__("source", 1), "source"),
            (lambda value: value.__setitem__("source", "INVALID"), "source"),
            (lambda value: value.__setitem__("tenant_id", 1), "tenant"),
            (lambda value: value.__setitem__("tenant_id", "INVALID"), "tenant"),
            (lambda value: value.__setitem__("trace_id", 1), "trace"),
            (lambda value: value.__setitem__("trace_id", "invalid"), "trace"),
            (lambda value: value.__setitem__("trace_id", "0" * 32), "trace"),
        ]
        for index, (mutate, message) in enumerate(mutations):
            with self.subTest(index=index):
                self.assert_invalid(mutate, message)

    def test_parse_price_rejects_invalid_payload_and_time_fields(self) -> None:
        mutations = [
            (lambda value: value["payload"].__setitem__("instrument", 1), "instrument"),
            (lambda value: value["payload"].__setitem__("instrument", "lower"), "instrument"),
            (lambda value: value.__setitem__("partition_key", "SP500"), "partition_key"),
            (lambda value: value["payload"].__setitem__("currency", 1), "currency"),
            (lambda value: value["payload"].__setitem__("currency", "usd"), "currency"),
            (lambda value: value["payload"].__setitem__("provider_sequence", "1"), "provider_sequence"),
            (lambda value: value["payload"].__setitem__("provider_sequence", True), "provider_sequence"),
            (lambda value: value["payload"].__setitem__("provider_sequence", -1), "provider_sequence"),
            (lambda value: value["payload"].__setitem__("price", "-1"), "fixed-point"),
            (lambda value: value.__setitem__("occurred_at", 1), "string"),
            (lambda value: value.__setitem__("occurred_at", "invalid"), "RFC3339"),
            (lambda value: value.__setitem__("occurred_at", "2026-07-21T00:00:00"), "timezone"),
            (lambda value: value.__setitem__("ingested_at", "2026-07-20T23:59:59Z"), "precedes"),
        ]
        for index, (mutate, message) in enumerate(mutations):
            with self.subTest(index=index):
                self.assert_invalid(mutate, message)


class PortfolioContractTests(unittest.TestCase):
    def assert_invalid(self, mutate, message: str = "") -> None:
        value = holdings_value()
        mutate(value)
        context = self.assertRaisesRegex(ValueError, message) if message else self.assertRaises(ValueError)
        with context:
            parse_portfolio(json.dumps(value).encode())

    def test_parse_portfolio_accepts_and_sorts_canonical_fixture(self) -> None:
        portfolio = parse_portfolio(HOLDINGS)
        self.assertEqual([position.instrument for position in portfolio.positions], ["FSELX", "QQQ", "QQQM"])
        self.assertEqual(portfolio.benchmark.instrument, "SP500")

    def test_parse_portfolio_rejects_invalid_json_and_shape(self) -> None:
        for data in [b"{", b"\xff"]:
            with self.subTest(data=data), self.assertRaisesRegex(ValueError, "invalid JSON"):
                parse_portfolio(data)
        with self.assertRaisesRegex(ValueError, "JSON object"):
            parse_portfolio(b"[]")
        self.assert_invalid(lambda value: value.update({"unknown": True}), "fields mismatch")

    def test_parse_portfolio_rejects_invalid_header_and_positions(self) -> None:
        mutations = [
            (lambda value: value.__setitem__("schema_version", "2"), "schema_version"),
            (lambda value: value.__setitem__("schema_version", True), "schema_version"),
            (lambda value: value.__setitem__("schema_version", 1), "schema_version"),
            (lambda value: value.__setitem__("portfolio_id", 1), "portfolio_id"),
            (lambda value: value.__setitem__("portfolio_id", "INVALID"), "portfolio_id"),
            (lambda value: value.__setitem__("base_currency", 1), "base_currency"),
            (lambda value: value.__setitem__("base_currency", "usd"), "base_currency"),
            (lambda value: value.__setitem__("display_name", " padded"), "display_name"),
            (lambda value: value.__setitem__("positions", {}), "non-empty"),
            (lambda value: value.__setitem__("positions", []), "non-empty"),
            (lambda value: value["positions"].__setitem__(0, []), "JSON object"),
            (lambda value: value["positions"][0].update({"unknown": True}), "fields mismatch"),
            (lambda value: value["positions"][0].__setitem__("instrument", "lower"), "instrument"),
            (lambda value: value["positions"].append(value["positions"][0]), "duplicate"),
            (lambda value: value["positions"][0].__setitem__("asset_type", "stock"), "asset_type"),
            (lambda value: value["positions"][0].__setitem__("valuation_type", "close"), "valuation_type"),
            (
                lambda value: value["positions"][2].__setitem__("valuation_type", "market_price"),
                "mutual fund",
            ),
            (lambda value: value["positions"][0].__setitem__("valuation_type", "nav"), "ETF"),
            (lambda value: value["positions"][0].__setitem__("display_name", " padded"), "display_name"),
            (lambda value: value["positions"][0].__setitem__("quantity", "0"), "greater than zero"),
        ]
        for index, (mutate, message) in enumerate(mutations):
            with self.subTest(index=index):
                self.assert_invalid(mutate, message)

    def test_parse_portfolio_rejects_invalid_benchmark(self) -> None:
        mutations = [
            (lambda value: value.__setitem__("benchmark", []), "JSON object"),
            (lambda value: value["benchmark"].update({"unknown": True}), "fields mismatch"),
            (lambda value: value["benchmark"].__setitem__("instrument", "lower"), "instrument"),
            (lambda value: value["benchmark"].__setitem__("instrument", "QQQ"), "must not also"),
            (lambda value: value["benchmark"].__setitem__("asset_type", "etf"), "index level"),
            (lambda value: value["benchmark"].__setitem__("valuation_type", "market_price"), "index level"),
            (lambda value: value["benchmark"].__setitem__("display_name", " padded"), "display_name"),
        ]
        for index, (mutate, message) in enumerate(mutations):
            with self.subTest(index=index):
                self.assert_invalid(mutate, message)


if __name__ == "__main__":
    unittest.main()
