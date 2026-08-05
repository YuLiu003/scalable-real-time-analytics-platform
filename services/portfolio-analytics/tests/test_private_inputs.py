from __future__ import annotations

import io
import json
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest.mock import patch

from portfolio_analytics import private_inputs


def private_holdings() -> dict:
    return {
        "schema_version": 2,
        "portfolio_id": "private",
        "display_name": "Private portfolio",
        "base_currency": "USD",
        "positions": [
            {
                "instrument": "ASSET-A",
                "display_name": "Asset A",
                "asset_type": "stock",
                "valuation_type": "market_price",
                "quantity": "2",
            },
            {
                "instrument": "ASSET-B",
                "display_name": "Asset B",
                "asset_type": "etf",
                "valuation_type": "market_price",
                "quantity": "1.5",
            },
        ],
        "benchmark": {
            "instrument": "BENCH-C",
            "display_name": "Benchmark C",
            "asset_type": "etf",
            "valuation_type": "market_price",
        },
    }


def environment(**overrides: str) -> bytes:
    values = {
        "APCA_API_KEY_ID": "key-id",
        "APCA_API_SECRET_KEY": "secret-key",
        "MARKET_WATCHLIST": "ASSET-B, BENCH-C,ASSET-A",
        "MARKET_TENANT_ID": "private",
    }
    values.update(overrides)
    return ("\n# private feed\n" + "\n".join(f"{name}={value}" for name, value in values.items()) + "\n").encode()


class PrivateInputCommandTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.repository = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.feed = self.write("feed.env", environment())
        self.holdings = self.write("holdings.json", json.dumps(private_holdings()).encode())
        self.token = self.write("access.token", b"t" * 32 + b"\n")
        self.repository_root = Path(self.repository.name).resolve()

    def tearDown(self) -> None:
        self.repository.cleanup()
        self.temporary.cleanup()

    def write(self, name: str, data: bytes, mode: int = 0o600) -> Path:
        path = self.root / name
        path.write_bytes(data)
        path.chmod(mode)
        return path

    def arguments(self, **overrides: str) -> list[str]:
        values = {
            "feed_environment": str(self.feed),
            "holdings": str(self.holdings),
            "access_token": str(self.token),
            "repository_root": str(self.repository_root),
        }
        values.update(overrides)
        return [
            "private-inputs",
            "--feed-environment",
            values["feed_environment"],
            "--holdings",
            values["holdings"],
            "--access-token",
            values["access_token"],
            "--repository-root",
            values["repository_root"],
        ]

    def run_success(self) -> dict:
        stdout = io.StringIO()
        stderr = io.StringIO()
        with patch("sys.argv", self.arguments()), redirect_stdout(stdout), redirect_stderr(stderr):
            private_inputs.main()
        self.assertEqual(stderr.getvalue(), "")
        return json.loads(stdout.getvalue())

    def assert_private_failure(self, arguments: list[str] | None = None) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()
        with (
            patch("sys.argv", arguments or self.arguments()),
            redirect_stdout(stdout),
            redirect_stderr(stderr),
            self.assertRaisesRegex(SystemExit, "1"),
        ):
            private_inputs.main()
        self.assertEqual(stdout.getvalue(), "")
        self.assertEqual(stderr.getvalue(), private_inputs.PRIVATE_ERROR + "\n")
        for private_value in [str(self.root), "ASSET-A", "private", "key-id", "secret-key", "t" * 32]:
            self.assertNotIn(private_value, stderr.getvalue())

    def test_main_accepts_matching_private_inputs_and_logs_only_counts(self) -> None:
        self.assertEqual(
            self.run_success(),
            {
                "event": "private inputs validated",
                "position_count": 2,
                "watchlist_count": 3,
            },
        )
        self.token.write_bytes(b"u" * 30 + b"==")
        self.assertEqual(self.run_success()["watchlist_count"], 3)
        self.token.write_bytes(b"u" * 512)
        self.assertEqual(self.run_success()["watchlist_count"], 3)

    def test_main_redacts_argument_and_repository_root_failures(self) -> None:
        self.assert_private_failure(["private-inputs", "--feed-environment", "secret-value"])
        self.assert_private_failure(self.arguments(repository_root="relative/root"))
        missing_root = self.root / "missing-root"
        self.assert_private_failure(self.arguments(repository_root=str(missing_root)))

    def test_main_rejects_unsafe_private_file_paths(self) -> None:
        relative = self.arguments(feed_environment="feed.env")
        self.assert_private_failure(relative)

        symlink = self.root / "feed-link"
        symlink.symlink_to(self.feed)
        self.assert_private_failure(self.arguments(feed_environment=str(symlink)))

        directory = self.root / "not-a-file"
        directory.mkdir()
        self.assert_private_failure(self.arguments(feed_environment=str(directory)))

        exposed = self.write("exposed.env", environment(), 0o640)
        self.assert_private_failure(self.arguments(feed_environment=str(exposed)))
        owner_read_only = self.write("read-only.env", environment(), 0o400)
        self.assert_private_failure(self.arguments(feed_environment=str(owner_read_only)))

        inside_repository = self.repository_root / "private-input-test.env"
        try:
            inside_repository.write_bytes(environment())
            inside_repository.chmod(0o600)
            self.assert_private_failure(self.arguments(feed_environment=str(inside_repository)))
        finally:
            inside_repository.unlink(missing_ok=True)

    def test_main_rejects_invalid_environment_files(self) -> None:
        invalid_values = [
            b"\xff",
            b"APCA_API_KEY_ID",
            environment(EXTRA="value"),
            environment() + b"APCA_API_KEY_ID=duplicate\n",
            b"APCA_API_KEY_ID=key-id\n",
            environment(APCA_API_KEY_ID=""),
            environment(APCA_API_SECRET_KEY="bad secret"),
            environment(APCA_API_KEY_ID="x" * 513),
            environment(APCA_API_KEY_ID="bad\x7fkey"),
            environment(APCA_API_KEY_ID="bad\u0080key"),
        ]
        for index, data in enumerate(invalid_values):
            with self.subTest(index=index):
                self.feed.write_bytes(data)
                self.assert_private_failure()

    def test_main_rejects_invalid_watchlists(self) -> None:
        invalid_values = [
            "ASSET-A,ASSET-A,BENCH-C",
            "asset-a,ASSET-B,BENCH-C",
            ",ASSET-B,BENCH-C",
            ",".join(f"S{index}" for index in range(31)),
        ]
        for value in invalid_values:
            with self.subTest(value=value):
                self.feed.write_bytes(environment(MARKET_WATCHLIST=value))
                self.assert_private_failure()

    def test_main_rejects_invalid_access_tokens(self) -> None:
        for data in [
            b"\xff",
            b"short",
            b"t" * 513,
            b"t" * 31 + b" ",
            b"t" * 32 + b"\r\n",
            "é".encode() * 32,
            b"!" * 32,
            b"t" * 31 + b"=t",
        ]:
            with self.subTest(length=len(data)):
                self.token.write_bytes(data)
                self.assert_private_failure()

    def test_main_rejects_non_private_portfolio_semantics(self) -> None:
        mutations = [
            lambda value: value.__setitem__("base_currency", "EUR"),
            lambda value: value.__setitem__("portfolio_id", "other"),
            lambda value: value["positions"][0].__setitem__("asset_type", "mutual_fund"),
            lambda value: value["benchmark"].update({"asset_type": "index", "valuation_type": "index_level"}),
        ]
        for index, mutate in enumerate(mutations):
            with self.subTest(index=index):
                holdings = private_holdings()
                mutate(holdings)
                if holdings["positions"][0]["asset_type"] == "mutual_fund":
                    holdings["positions"][0]["valuation_type"] = "nav"
                self.holdings.write_text(json.dumps(holdings))
                self.assert_private_failure()

    def test_main_rejects_invalid_holdings_and_watchlist_mismatch(self) -> None:
        self.holdings.write_bytes(b"private holdings value")
        self.assert_private_failure()
        self.holdings.write_text(json.dumps(private_holdings()))
        self.feed.write_bytes(environment(MARKET_WATCHLIST="ASSET-A,ASSET-B,OTHER"))
        self.assert_private_failure()

        holdings = private_holdings()
        holdings["portfolio_id"] = "other"
        self.holdings.write_text(json.dumps(holdings))
        self.feed.write_bytes(environment(MARKET_TENANT_ID="other"))
        self.assert_private_failure()


if __name__ == "__main__":
    unittest.main()
