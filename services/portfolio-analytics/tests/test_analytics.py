from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import duckdb

from portfolio_analytics.analytics import build_products, latest_required_objects
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
                "instrument": "DEMO-ASSET-A",
                "display_name": "Synthetic Asset A",
                "asset_type": "etf",
                "valuation_type": "market_price",
                "quantity": "1.00000000",
            },
            {
                "instrument": "DEMO-ASSET-B",
                "display_name": "Synthetic Asset B",
                "asset_type": "etf",
                "valuation_type": "market_price",
                "quantity": "2.00000000",
            },
            {
                "instrument": "DEMO-ASSET-C",
                "display_name": "Synthetic Asset C",
                "asset_type": "mutual_fund",
                "valuation_type": "nav",
                "quantity": "3.00000000",
            },
        ],
        "benchmark": {
            "instrument": "DEMO-BENCH-D",
            "display_name": "Synthetic Benchmark D",
            "asset_type": "index",
            "valuation_type": "index_level",
        },
    },
    separators=(",", ":"),
    sort_keys=True,
).encode()


def bronze() -> list[BronzeObject]:
    return [
        BronzeObject("bronze/demo-asset-a.json", event("DEMO-ASSET-A", "100.0000", 1, "2026-07-21T00:00:00Z")),
        BronzeObject("bronze/demo-asset-b.json", event("DEMO-ASSET-B", "100.0000", 2, "2026-07-21T00:00:02Z")),
        BronzeObject("bronze/demo-asset-c.json", event("DEMO-ASSET-C", "100.0000", 100, "2026-07-21T00:01:00Z")),
        BronzeObject("bronze/demo-bench-d.json", event("DEMO-BENCH-D", "1000.0000", 101, "2026-07-21T00:01:30Z")),
    ]


class AnalyticsTests(unittest.TestCase):
    def test_builds_deterministic_parquet_and_result(self) -> None:
        with tempfile.TemporaryDirectory() as first_directory, tempfile.TemporaryDirectory() as second_directory:
            products = build_products(bronze(), HOLDINGS, Path(first_directory))
            replayed = build_products(list(reversed(bronze())), HOLDINGS, Path(second_directory))
            self.assertEqual(products["result"]["total_market_value"], "600.00000000")
            self.assertEqual(len(products["result"]["positions"]), 3)
            allocations = {item["instrument"]: item["allocation_pct"] for item in products["result"]["positions"]}
            self.assertEqual(allocations, {"DEMO-ASSET-C": "50.0000", "DEMO-ASSET-A": "16.6667", "DEMO-ASSET-B": "33.3333"})
            self.assertEqual(products["result"]["benchmark"]["instrument"], "DEMO-BENCH-D")
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

    def test_private_selection_keeps_only_latest_required_observations(self) -> None:
        objects = bronze()
        objects.extend(
            [
                BronzeObject(
                    "bronze/newer-a.json",
                    event("DEMO-ASSET-A", "125.0000", 200, "2026-07-22T00:00:00Z"),
                ),
                BronzeObject(
                    "bronze/unrelated.json",
                    event("UNRELATED", "10.0000", 201, "2026-07-22T00:00:00Z"),
                ),
                BronzeObject(
                    "bronze/zz-older-a.json",
                    event("DEMO-ASSET-A", "90.0000", 202, "2026-07-20T00:00:00Z"),
                ),
            ]
        )
        selected = latest_required_objects(list(reversed(objects)), HOLDINGS)
        self.assertEqual(len(selected), 4)
        self.assertIn("bronze/newer-a.json", {item.key for item in selected})
        self.assertNotIn("bronze/demo-asset-a.json", {item.key for item in selected})

        duplicate = [*objects, BronzeObject("bronze/duplicate.json", objects[0].data)]
        with self.assertRaisesRegex(ValueError, "duplicate event_id"):
            latest_required_objects(duplicate, HOLDINGS)

        cross_tenant = list(objects)
        value = json.loads(cross_tenant[0].data)
        value["tenant_id"] = "another-portfolio"
        cross_tenant[0] = BronzeObject(cross_tenant[0].key, json.dumps(value).encode())
        with self.assertRaisesRegex(ValueError, "unexpected tenant_id"):
            latest_required_objects(cross_tenant, HOLDINGS)

    def test_missing_price_fails_without_a_partial_result(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "DEMO-BENCH-D"):
                build_products(bronze()[:-1], HOLDINGS, Path(directory))

    def test_holdings_reject_duplicate_instrument(self) -> None:
        value = json.loads(HOLDINGS)
        value["positions"].append(value["positions"][0])
        with self.assertRaisesRegex(ValueError, "duplicate instrument DEMO-ASSET-A"):
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
