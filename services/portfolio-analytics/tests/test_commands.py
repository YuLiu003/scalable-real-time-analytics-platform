from __future__ import annotations

import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import Mock, patch

from portfolio_analytics import builder, query_products
from portfolio_analytics.analytics import build_products
from test_analytics import HOLDINGS, bronze


class FakeStore:
    def __init__(self, objects: dict[str, bytes] | None = None):
        self.objects = objects or {}
        self.immutable_writes: list[str] = []
        self.latest_writes: list[str] = []

    def list_objects(self, prefix: str):
        self.prefix = prefix
        return bronze()

    def get(self, key: str) -> bytes:
        return self.objects[key]

    def put_immutable(self, key: str, data: bytes, content_type: str) -> str:
        self.immutable_writes.append(key)
        return "created"

    def put_latest(self, key: str, data: bytes) -> None:
        self.latest_writes.append(key)


class BuilderCommandTests(unittest.TestCase):
    def test_main_reads_builds_and_publishes_products(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            holdings = root / "holdings.json"
            holdings.write_bytes(HOLDINGS)
            products_dir = root / "products"
            products_dir.mkdir()
            products = build_products(bronze(), HOLDINGS, products_dir)
            store = FakeStore()
            environment = {
                "HOLDINGS_FILE": str(holdings),
                "BRONZE_PREFIX": "bronze/custom/",
                "WORK_ROOT": str(root),
            }
            output = io.StringIO()
            with (
                patch.dict("os.environ", environment, clear=True),
                patch.object(builder.S3Settings, "from_environment", return_value=Mock()),
                patch.object(builder, "ObjectStore", return_value=store),
                patch.object(builder, "build_products", return_value=products),
                redirect_stdout(output),
            ):
                builder.main()
        result = json.loads(output.getvalue())
        self.assertEqual(result["event"], "portfolio analytics build complete")
        self.assertEqual(store.prefix, "bronze/custom/")
        self.assertEqual(store.immutable_writes, [products["silver_key"], products["gold_key"]])
        self.assertEqual(store.latest_writes, [products["latest_key"]])


class QueryCommandTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        products = build_products(bronze(), HOLDINGS, self.root / "source")
        self.result = products["result"]
        self.objects = {
            products["latest_key"]: products["latest_bytes"],
            products["silver_key"]: products["silver_file"].read_bytes(),
            products["gold_key"]: products["gold_file"].read_bytes(),
        }

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def run_query(self, *, result: dict | None = None, expected_total: str = "6200.00000000") -> str:
        objects = dict(self.objects)
        if result is not None:
            objects["gold/portfolio_allocations/v2/portfolio=demo/latest.json"] = json.dumps(result).encode()
        store = FakeStore(objects)
        output = io.StringIO()
        environment = {"WORK_ROOT": str(self.root), "EXPECTED_TOTAL": expected_total}
        with (
            patch.dict("os.environ", environment, clear=True),
            patch.object(query_products.S3Settings, "from_environment", return_value=Mock()),
            patch.object(query_products, "ObjectStore", return_value=store),
            redirect_stdout(output),
        ):
            query_products.main()
        return output.getvalue()

    def test_main_queries_both_parquet_products(self) -> None:
        output = json.loads(self.run_query())
        self.assertEqual(output, {"event": "DuckDB Parquet query passed", "gold_rows": 3, "silver_rows": 4, "total_market_value": "6200.00000000"})

    def test_main_rejects_each_row_count_mismatch(self) -> None:
        for field in ["input_object_count", "positions"]:
            with self.subTest(field=field):
                result = json.loads(json.dumps(self.result))
                if field == "positions":
                    result[field].append(result[field][0])
                else:
                    result[field] += 1
                with self.assertRaisesRegex(ValueError, "row mismatch"):
                    self.run_query(result=result)

    def test_main_rejects_expected_and_snapshot_total_mismatch(self) -> None:
        with self.assertRaisesRegex(ValueError, "does not match"):
            self.run_query(expected_total="1.00000000")
        result = json.loads(json.dumps(self.result))
        result["total_market_value"] = "1.00000000"
        with self.assertRaisesRegex(ValueError, "does not match"):
            self.run_query(result=result)


if __name__ == "__main__":
    unittest.main()
