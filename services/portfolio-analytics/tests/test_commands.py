from __future__ import annotations

import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest.mock import Mock, patch

from portfolio_analytics import builder, import_ledger, query_products
from portfolio_analytics.analytics import build_products
from test_analytics import HOLDINGS, bronze

FIXTURE_OVERRIDE = os.environ.get("PORTFOLIO_FIXTURE_ROOT")
FIXTURES = Path(FIXTURE_OVERRIDE) if FIXTURE_OVERRIDE else Path(__file__).parents[3] / "contracts" / "fixtures"


class FakeStore:
    def __init__(self, objects: dict[str, bytes] | None = None):
        self.objects = objects or {}
        self.immutable_writes: list[str] = []
        self.latest_writes: list[str] = []

    def list_objects(self, prefix: str, required_key_segment: str = ""):
        self.prefix = prefix
        self.required_key_segment = required_key_segment
        return bronze()

    def list_latest_source_objects(
        self,
        prefix: str,
        required_key_segment: str,
        maximum_date_partitions: int,
    ):
        self.prefix = prefix
        self.required_key_segment = required_key_segment
        self.maximum_date_partitions = maximum_date_partitions
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
        self.assertEqual(result["result_sha256"], products["result_sha256"])
        self.assertEqual(store.prefix, "bronze/custom/")
        self.assertEqual(store.required_key_segment, "/source=synthetic/")
        self.assertEqual(store.immutable_writes, [products["silver_key"], products["gold_key"]])
        self.assertEqual(store.latest_writes, [products["latest_key"]])

    def test_main_rejects_invalid_bronze_source(self) -> None:
        with (
            patch.dict("os.environ", {"BRONZE_SOURCE": "PRIVATE/SOURCE"}, clear=True),
            self.assertRaisesRegex(ValueError, "BRONZE_SOURCE"),
        ):
            builder._build(False)

    def test_private_mode_reports_only_aggregate_outcomes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            holdings = root / "holdings.json"
            holdings.write_bytes(HOLDINGS)
            products_dir = root / "products"
            products_dir.mkdir()
            products = build_products(bronze(), HOLDINGS, products_dir)
            store = FakeStore()
            output = io.StringIO()
            environment = {
                "ANALYTICS_PRIVATE_MODE": "true",
                "HOLDINGS_FILE": str(holdings),
                "WORK_ROOT": str(root),
            }
            with (
                patch.dict("os.environ", environment, clear=True),
                patch.object(builder.S3Settings, "from_environment", return_value=Mock()),
                patch.object(builder, "ObjectStore", return_value=store),
                patch.object(builder, "build_products", return_value=products),
                redirect_stdout(output),
            ):
                builder.main()
        self.assertEqual(
            json.loads(output.getvalue()),
            {
                "event": "portfolio analytics build complete",
                "gold_write": "created",
                "input_objects": 4,
                "positions": 3,
                "silver_write": "created",
            },
        )
        self.assertEqual(store.maximum_date_partitions, 2)
        self.assertNotIn("result_sha256", json.loads(output.getvalue()))

    def test_private_mode_redacts_every_exception(self) -> None:
        private_value = "PRIVATE/SOURCE"
        for environment in [
            {"ANALYTICS_PRIVATE_MODE": "true", "BRONZE_SOURCE": private_value},
            {"BRONZE_SOURCE": "alpaca-iex"},
            {"ANALYTICS_PRIVATE_MODE": "typo", "BRONZE_SOURCE": "alpaca-iex"},
            {"ANALYTICS_PRIVATE_MODE": "typo"},
        ]:
            with self.subTest(environment=environment):
                stdout = io.StringIO()
                stderr = io.StringIO()
                with (
                    patch.dict("os.environ", environment, clear=True),
                    redirect_stdout(stdout),
                    redirect_stderr(stderr),
                    self.assertRaisesRegex(SystemExit, "1"),
                ):
                    builder.main()
                self.assertEqual(stdout.getvalue(), "")
                self.assertEqual(stderr.getvalue(), '{"event":"portfolio analytics build failed"}\n')
                self.assertNotIn(private_value, stderr.getvalue())

        stdout = io.StringIO()
        stderr = io.StringIO()
        with (
            patch.dict("os.environ", {}, clear=True),
            patch.object(builder, "_build", side_effect=ValueError("public failure")),
            redirect_stdout(stdout),
            redirect_stderr(stderr),
            self.assertRaisesRegex(SystemExit, "1"),
        ):
            builder.main()
        self.assertEqual(stdout.getvalue(), "")
        self.assertEqual(stderr.getvalue(), '{"event":"portfolio analytics build failed"}\n')


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

    def run_query(self, *, result: dict | None = None, expected_total: str = "600.00000000") -> str:
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
        self.assertEqual(output, {"event": "DuckDB Parquet query passed", "gold_rows": 3, "silver_rows": 4, "total_market_value": "600.00000000"})

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


class ImportLedgerCommandTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.fixture_root = FIXTURES

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def run_import(self, input_format: str, input_path: Path, output: Path) -> str:
        stdout = io.StringIO()
        arguments = [
            "import-ledger",
            "--input-format",
            input_format,
            "--portfolio",
            "demo",
            "--currency",
            "USD",
            "--input",
            str(input_path),
            "--output",
            str(output),
        ]
        with patch("sys.argv", arguments), redirect_stdout(stdout):
            import_ledger.main()
        return stdout.getvalue()

    def test_main_imports_both_formats_without_logging_row_values(self) -> None:
        outputs: list[bytes] = []
        for input_format in ["json", "csv"]:
            with self.subTest(input_format=input_format):
                output = self.root / f"ledger-{input_format}.json"
                log = self.run_import(
                    input_format,
                    self.fixture_root / f"demo-cash-flows.v1.{input_format}",
                    output,
                )
                outputs.append(output.read_bytes())
                self.assertEqual(output.stat().st_mode & 0o777, 0o600)
                self.assertEqual(
                    json.loads(log),
                    {
                        "event": "cash-flow ledger imported",
                        "transaction_count": 4,
                    },
                )
                self.assertNotIn("demo", log)
                self.assertNotIn("demo-cash-001", log)
                self.assertNotIn("1000", log)
        self.assertEqual(outputs[0], outputs[1])

    def test_main_requires_explicit_arguments(self) -> None:
        with patch("sys.argv", ["import-ledger"]), redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
            import_ledger.main()
        self.assertEqual(raised.exception.code, 2)

    def test_invalid_input_neither_creates_nor_replaces_output_and_logs_nothing(self) -> None:
        invalid = self.root / "invalid.json"
        invalid.write_text("[{}]")
        for exists in [False, True]:
            with self.subTest(existing_output=exists):
                output = self.root / f"output-{exists}.json"
                if exists:
                    output.write_bytes(b"protected")
                stdout = io.StringIO()
                with redirect_stdout(stdout), self.assertRaisesRegex(ValueError, "fields mismatch"):
                    self.run_import("json", invalid, output)
                self.assertEqual(stdout.getvalue(), "")
                if exists:
                    self.assertEqual(output.read_bytes(), b"protected")
                else:
                    self.assertFalse(output.exists())

    def test_atomic_replace_failure_preserves_output_and_removes_temporary_file(self) -> None:
        output = self.root / "ledger.json"
        output.write_bytes(b"protected")
        with patch.object(import_ledger.os, "replace", side_effect=OSError("replace failed")):
            with self.assertRaisesRegex(OSError, "replace failed"):
                import_ledger._write_atomic(output, b"new ledger")
        self.assertEqual(output.read_bytes(), b"protected")
        self.assertEqual(list(self.root.glob(".ledger.json.*")), [])


if __name__ == "__main__":
    unittest.main()
