from __future__ import annotations

import json
import os
import unittest
from pathlib import Path

from portfolio_analytics.ledger import INPUT_FIELDS, build_ledger

FIXTURE_OVERRIDE = os.environ.get("PORTFOLIO_FIXTURE_ROOT")
FIXTURES = Path(FIXTURE_OVERRIDE) if FIXTURE_OVERRIDE else Path(__file__).parents[3] / "contracts" / "fixtures"


def row(**changes) -> dict:
    value = {
        "transaction_id": "cash-001",
        "occurred_at": "2026-01-01T12:00:00Z",
        "cash_flow_type": "deposit",
        "amount": "10.25",
        "currency": "USD",
    }
    value.update(changes)
    return value


def json_input(*rows: object) -> bytes:
    return json.dumps(list(rows)).encode()


class LedgerTests(unittest.TestCase):
    def build_json(self, *rows: object, portfolio: str = "demo", currency: str = "USD") -> dict:
        return json.loads(build_ledger(json_input(*rows), "json", portfolio, currency))

    def assert_invalid_row(self, changes: dict, message: str) -> None:
        with self.assertRaisesRegex(ValueError, message):
            self.build_json(row(**changes))

    def test_fictional_json_and_csv_are_canonically_equivalent(self) -> None:
        json_bytes = build_ledger(
            (FIXTURES / "demo-cash-flows.v1.json").read_bytes(),
            "json",
            "demo",
            "USD",
        )
        csv_bytes = build_ledger(
            (FIXTURES / "demo-cash-flows.v1.csv").read_bytes(),
            "csv",
            "demo",
            "USD",
        )

        self.assertEqual(json_bytes, csv_bytes)
        self.assertTrue(json_bytes.endswith(b"\n"))
        ledger = json.loads(json_bytes)
        self.assertEqual(
            ledger["totals"],
            {
                "net_external_cash_flow": "1024.50000000",
                "total_deposits": "1125.12500000",
                "total_withdrawals": "100.62500000",
            },
        )
        self.assertEqual(
            [item["transaction_id"] for item in ledger["transactions"]],
            ["demo-cash-001", "demo-cash-002", "demo-cash-003", "demo-cash-004"],
        )
        self.assertEqual(
            set(ledger),
            {"schema_version", "portfolio_id", "base_currency", "transactions", "totals"},
        )
        self.assertEqual(set(ledger["transactions"][0]), set(INPUT_FIELDS) - {"currency"})
        self.assertNotIn(b"account", json_bytes)
        self.assertNotIn(b"broker", json_bytes)

    def test_timestamp_normalization_and_secondary_id_order_are_deterministic(self) -> None:
        ledger = self.build_json(
            row(transaction_id="cash-b", occurred_at="2026-01-01T13:00:00+01:00", amount="1"),
            row(transaction_id="cash-a", occurred_at="2026-01-01T12:00:00Z", amount="2.00000001"),
        )
        self.assertEqual([item["transaction_id"] for item in ledger["transactions"]], ["cash-a", "cash-b"])
        self.assertEqual([item["occurred_at"] for item in ledger["transactions"]], ["2026-01-01T12:00:00Z"] * 2)
        self.assertEqual([item["amount"] for item in ledger["transactions"]], ["2.00000001", "1.00000000"])

    def test_empty_ledger_has_explicit_zero_totals(self) -> None:
        expected = {
            "net_external_cash_flow": "0.00000000",
            "total_deposits": "0.00000000",
            "total_withdrawals": "0.00000000",
        }
        self.assertEqual(self.build_json()["totals"], expected)
        csv_data = (",".join(INPUT_FIELDS) + "\n").encode()
        self.assertEqual(json.loads(build_ledger(csv_data, "csv", "demo", "USD"))["totals"], expected)

    def test_rejects_invalid_json_encoding_shape_and_rows(self) -> None:
        for data in [b"{", b"\xff"]:
            with self.subTest(data=data), self.assertRaisesRegex(ValueError, "invalid JSON"):
                build_ledger(data, "json", "demo", "USD")
        with self.assertRaisesRegex(ValueError, "must be an array"):
            build_ledger(b"{}", "json", "demo", "USD")
        with self.assertRaisesRegex(ValueError, "must be an object"):
            self.build_json([])
        duplicate_field = json_input(row()).replace(
            b'"amount": "10.25"',
            b'"amount": "10.25", "amount": "20"',
        )
        with self.assertRaisesRegex(ValueError, "duplicate object field"):
            build_ledger(duplicate_field, "json", "demo", "USD")
        value = row()
        del value["amount"]
        value["unknown"] = "value"
        with self.assertRaisesRegex(ValueError, "missing=.*amount.*unknown=.*unknown"):
            self.build_json(value)

    def test_rejects_invalid_csv_encoding_header_and_syntax(self) -> None:
        with self.assertRaisesRegex(ValueError, "UTF-8"):
            build_ledger(b"\xff", "csv", "demo", "USD")
        with self.assertRaisesRegex(ValueError, "headers"):
            build_ledger(b"occurred_at,transaction_id,cash_flow_type,amount,currency\n", "csv", "demo", "USD")
        malformed = (",".join(INPUT_FIELDS) + '\n"unterminated').encode()
        with self.assertRaisesRegex(ValueError, "invalid CSV"):
            build_ledger(malformed, "csv", "demo", "USD")
        extra_value = (",".join(INPUT_FIELDS) + "\ncash-1,2026-01-01T00:00:00Z,deposit,1,USD,extra\n").encode()
        with self.assertRaisesRegex(ValueError, "fields mismatch"):
            build_ledger(extra_value, "csv", "demo", "USD")

    def test_rejects_invalid_row_values(self) -> None:
        cases = [
            ({"transaction_id": 1}, "transaction_id"),
            ({"transaction_id": "INVALID"}, "transaction_id"),
            ({"occurred_at": 1}, "string"),
            ({"occurred_at": "not-a-time"}, "RFC3339"),
            ({"occurred_at": "2026-01-01T00:00:00"}, "RFC3339"),
            ({"occurred_at": "2026-01-01 00:00:00Z"}, "RFC3339"),
            ({"occurred_at": "2026-W01-4T00:00:00Z"}, "RFC3339"),
            ({"occurred_at": "2026-01-01T00:00:00.1234567Z"}, "6 fractional"),
            ({"occurred_at": "2026-02-30T00:00:00Z"}, "RFC3339"),
            ({"cash_flow_type": "transfer"}, "deposit or withdrawal"),
            ({"amount": 1}, "positive decimal"),
            ({"amount": "0"}, "positive decimal"),
            ({"amount": "1.123456789"}, "positive decimal"),
            ({"currency": 1}, "currency"),
            ({"currency": "usd"}, "currency"),
        ]
        for changes, message in cases:
            with self.subTest(changes=changes):
                self.assert_invalid_row(changes, message)

    def test_preserves_supported_timestamp_precision(self) -> None:
        ledger = self.build_json(row(occurred_at="2026-01-01T00:00:00.123456Z"))
        self.assertEqual(ledger["transactions"][0]["occurred_at"], "2026-01-01T00:00:00.123456Z")

    def test_rejects_duplicate_ids_and_mixed_or_unexpected_currency(self) -> None:
        with self.assertRaisesRegex(ValueError, "unique"):
            self.build_json(row(), row(amount="20"))
        with self.assertRaisesRegex(ValueError, "must match"):
            self.build_json(row(currency="EUR"))
        with self.assertRaisesRegex(ValueError, "must match"):
            self.build_json(row(), row(transaction_id="cash-002", currency="EUR"))

    def test_rejects_invalid_import_options(self) -> None:
        with self.assertRaisesRegex(ValueError, "portfolio_id"):
            build_ledger(b"[]", "json", "INVALID", "USD")
        with self.assertRaisesRegex(ValueError, "base_currency"):
            build_ledger(b"[]", "json", "demo", "usd")
        with self.assertRaisesRegex(ValueError, "input_format"):
            build_ledger(b"[]", "yaml", "demo", "USD")

    def test_negative_net_remains_separate_from_withdrawal_total(self) -> None:
        ledger = self.build_json(row(cash_flow_type="withdrawal", amount="10"))
        self.assertEqual(
            ledger["totals"],
            {
                "net_external_cash_flow": "-10.00000000",
                "total_deposits": "0.00000000",
                "total_withdrawals": "10.00000000",
            },
        )

    def test_totals_remain_exact_beyond_the_default_decimal_precision(self) -> None:
        large = "9" * 40
        ledger = self.build_json(row(amount=large), row(transaction_id="cash-002", amount="1"))
        self.assertEqual(ledger["totals"]["total_deposits"], "1" + ("0" * 40) + ".00000000")


if __name__ == "__main__":
    unittest.main()
