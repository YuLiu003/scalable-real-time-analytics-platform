from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import duckdb

from portfolio_analytics.analytics import build_products
from portfolio_analytics.model import BronzeObject, input_set_sha256, parse_portfolio


def event(instrument: str, price: str, sequence: int, occurred_at: str) -> bytes:
    value = {
        "event_id": f"synthetic:price:{instrument.lower()}:{sequence}",
        "event_type": "market.price.observed",
        "schema_version": 1,
        "source": "synthetic",
        "tenant_id": "demo",
        "occurred_at": occurred_at,
        "ingested_at": occurred_at,
        "partition_key": instrument,
        "trace_id": f"{sequence:032x}",
        "payload": {
            "instrument": instrument,
            "currency": "USD",
            "price": price,
            "provider_sequence": sequence,
        },
    }
    return json.dumps(value, separators=(",", ":"), sort_keys=True).encode()


HOLDINGS = json.dumps(
    {
        "schema_version": 2,
        "portfolio_id": "demo",
        "display_name": "Synthetic Fund Portfolio",
        "base_currency": "USD",
        "positions": [
            {
                "instrument": "QQQ",
                "display_name": "Invesco QQQ",
                "asset_type": "etf",
                "valuation_type": "market_price",
                "quantity": "4.00000000",
            },
            {
                "instrument": "QQQM",
                "display_name": "Invesco NASDAQ 100 ETF",
                "asset_type": "etf",
                "valuation_type": "market_price",
                "quantity": "8.00000000",
            },
            {
                "instrument": "FSELX",
                "display_name": "Fidelity Select Semiconductors Portfolio",
                "asset_type": "mutual_fund",
                "valuation_type": "nav",
                "quantity": "30.00000000",
            },
        ],
        "benchmark": {
            "instrument": "SP500",
            "display_name": "S&P 500 Index",
            "asset_type": "index",
            "valuation_type": "index_level",
        },
    },
    separators=(",", ":"),
    sort_keys=True,
).encode()


def bronze() -> list[BronzeObject]:
    return [
        BronzeObject("bronze/qqq.json", event("QQQ", "600.0000", 1, "2026-07-21T00:00:00Z")),
        BronzeObject("bronze/qqqm.json", event("QQQM", "250.0000", 2, "2026-07-21T00:00:02Z")),
        BronzeObject("bronze/fselx.json", event("FSELX", "60.0000", 100, "2026-07-21T00:01:00Z")),
        BronzeObject("bronze/sp500.json", event("SP500", "6500.0000", 101, "2026-07-21T00:01:30Z")),
    ]


class AnalyticsTests(unittest.TestCase):
    def test_builds_deterministic_parquet_and_result(self) -> None:
        with tempfile.TemporaryDirectory() as first_directory, tempfile.TemporaryDirectory() as second_directory:
            products = build_products(bronze(), HOLDINGS, Path(first_directory))
            replayed = build_products(list(reversed(bronze())), HOLDINGS, Path(second_directory))
            self.assertEqual(products["result"]["total_market_value"], "6200.00000000")
            self.assertEqual(len(products["result"]["positions"]), 3)
            allocations = {item["instrument"]: item["allocation_pct"] for item in products["result"]["positions"]}
            self.assertEqual(allocations, {"FSELX": "29.0323", "QQQ": "38.7097", "QQQM": "32.2581"})
            self.assertEqual(products["result"]["benchmark"]["instrument"], "SP500")
            self.assertEqual(products["result"]["benchmark"]["valuation_type"], "index_level")
            self.assertEqual(products["latest_bytes"], replayed["latest_bytes"])
            self.assertEqual(products["silver_file"].read_bytes(), replayed["silver_file"].read_bytes())
            self.assertEqual(products["gold_file"].read_bytes(), replayed["gold_file"].read_bytes())
            self.assertEqual(
                duckdb.sql(f"SELECT count(*) FROM read_parquet('{products['silver_file']}')").fetchone()[0],
                4,
            )
            self.assertEqual(
                duckdb.sql(f"SELECT count(*) FROM read_parquet('{products['gold_file']}')").fetchone()[0],
                3,
            )

    def test_input_identity_is_order_independent(self) -> None:
        objects = bronze()
        self.assertEqual(input_set_sha256(HOLDINGS, objects), input_set_sha256(HOLDINGS, reversed(objects)))

    def test_missing_price_fails_without_a_partial_result(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "SP500"):
                build_products(bronze()[:-1], HOLDINGS, Path(directory))

    def test_holdings_reject_duplicate_instrument(self) -> None:
        value = json.loads(HOLDINGS)
        value["positions"].append(value["positions"][0])
        with self.assertRaisesRegex(ValueError, "duplicate instrument QQQ"):
            parse_portfolio(json.dumps(value).encode())

    def test_cross_tenant_bronze_fails_closed(self) -> None:
        objects = bronze()
        value = json.loads(objects[0].data)
        value["tenant_id"] = "another-portfolio"
        objects[0] = BronzeObject(objects[0].key, json.dumps(value).encode())
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "unexpected tenant_id"):
                build_products(objects, HOLDINGS, Path(directory))

    def test_empty_duplicate_and_currency_mismatch_inputs_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "no bronze"):
                build_products([], HOLDINGS, Path(directory))

        objects = bronze()
        objects[1] = BronzeObject(objects[1].key, objects[0].data)
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "duplicate event_id"):
                build_products(objects, HOLDINGS, Path(directory))

        objects = bronze()
        value = json.loads(objects[0].data)
        value["payload"]["currency"] = "CAD"
        objects[0] = BronzeObject(objects[0].key, json.dumps(value).encode())
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "FX conversion"):
                build_products(objects, HOLDINGS, Path(directory))


if __name__ == "__main__":
    unittest.main()
