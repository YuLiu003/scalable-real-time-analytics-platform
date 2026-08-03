#!/usr/bin/env python3
"""Fail when committed portfolio or scale fixtures are not explicitly fictional."""

from __future__ import annotations

import csv
import json
import re
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURE = REPO_ROOT / "contracts" / "fixtures" / "demo-fund-portfolio.v2.json"
SCALE_MANIFEST = (
    REPO_ROOT
    / "platform"
    / "gitops"
    / "apps"
    / "local"
    / "market-pipeline"
    / "scale-lab.yaml"
)
SCALE_VERIFY = REPO_ROOT / "platform" / "local" / "scripts" / "verify-scale-lab.sh"
CAPACITY_VERIFY = (
    REPO_ROOT / "platform" / "local" / "scripts" / "verify-capacity-benchmark.sh"
)
LEDGER_JSON = REPO_ROOT / "contracts" / "fixtures" / "demo-cash-flows.v1.json"
LEDGER_CSV = REPO_ROOT / "contracts" / "fixtures" / "demo-cash-flows.v1.csv"
PRIVATE_FEED_MANIFEST = (
    REPO_ROOT
    / "platform"
    / "gitops"
    / "apps"
    / "private"
    / "market-feed"
    / "market-feed.yaml"
)
DEMO_ID = re.compile(r"^DEMO-(?:ASSET|BENCH)-[A-Z]$")
LOAD_ID = re.compile(r"^LOAD-[A-Z]$")
DEMO_TRANSACTION_ID = re.compile(r"^demo-cash-[0-9]{3}$")
LEDGER_FIELDS = {
    "transaction_id",
    "occurred_at",
    "cash_flow_type",
    "amount",
    "currency",
}


def main() -> None:
    fixture = json.loads(FIXTURE.read_text(encoding="utf-8"))
    instruments = [
        (position["instrument"], position["display_name"])
        for position in fixture["positions"]
    ]
    instruments.append(
        (fixture["benchmark"]["instrument"], fixture["benchmark"]["display_name"])
    )
    if not fixture["display_name"].startswith("Synthetic "):
        raise SystemExit("public portfolio display name must be explicitly synthetic")
    for instrument, display_name in instruments:
        if not DEMO_ID.fullmatch(instrument) or not display_name.startswith("Synthetic "):
            raise SystemExit(
                "committed portfolio instruments and display names must be fictional"
            )

    manifest = SCALE_MANIFEST.read_text(encoding="utf-8")
    match = re.search(r"name: LOAD_INSTRUMENTS\s+value: ([A-Z0-9,-]+)", manifest)
    if match is None:
        raise SystemExit("scale manifest must declare a public LOAD_INSTRUMENTS list")
    load_instruments = match.group(1).split(",")
    if len(load_instruments) < 3 or any(
        not LOAD_ID.fullmatch(instrument) for instrument in load_instruments
    ):
        raise SystemExit("committed scale instruments must be fictional LOAD-* keys")

    verify_script = SCALE_VERIFY.read_text(encoding="utf-8")
    event_default = re.search(
        r'event_count="\$\{SCALE_EVENT_COUNT:-(\d+)\}"', verify_script
    )
    delay_default = re.search(
        r'archiver_delay_ms="\$\{SCALE_ARCHIVER_DELAY_MS:-(\d+)\}"',
        verify_script,
    )
    if event_default is None or delay_default is None:
        raise SystemExit("scale verification must declare numeric load defaults")
    minimum_work_ms = (
        int(event_default.group(1)) // 12 * 4 * int(delay_default.group(1))
    )
    if minimum_work_ms < 35_000:
        raise SystemExit(
            "scale defaults must retain lag across the HPA observation window"
        )

    runtime_lists = re.findall(
        r"^\s*LOAD_INSTRUMENTS=([A-Z0-9,-]+)", verify_script, re.MULTILINE
    )
    if not runtime_lists or any(
        runtime_list.split(",") != load_instruments for runtime_list in runtime_lists
    ):
        raise SystemExit("scale verification must pin the committed fictional instruments")

    capacity_script = CAPACITY_VERIFY.read_text(encoding="utf-8")
    capacity_lists = re.findall(
        r"^\s*LOAD_INSTRUMENTS=([A-Z0-9,-]+)", capacity_script, re.MULTILINE
    )
    if not capacity_lists or any(
        runtime_list.split(",") != load_instruments
        for runtime_list in capacity_lists
    ):
        raise SystemExit("capacity benchmark must pin the committed fictional instruments")
    if not re.search(
        r'event_counts="\$\{CAPACITY_EVENT_COUNTS:-10000,50000,100000\}"',
        capacity_script,
    ) or not re.search(
        r'repetitions="\$\{CAPACITY_REPETITIONS:-5\}"', capacity_script
    ):
        raise SystemExit("capacity benchmark must retain the reviewed default matrix")

    json_rows = json.loads(LEDGER_JSON.read_text(encoding="utf-8"))
    with LEDGER_CSV.open(encoding="utf-8", newline="") as stream:
        csv_rows = list(csv.DictReader(stream))
    for rows in (json_rows, csv_rows):
        if not isinstance(rows, list) or any(set(row) != LEDGER_FIELDS for row in rows):
            raise SystemExit("committed ledger rows must use only the reviewed cash-flow fields")
        if any(
            not DEMO_TRANSACTION_ID.fullmatch(row["transaction_id"])
            or row["cash_flow_type"] not in {"deposit", "withdrawal"}
            or row["currency"] != "USD"
            for row in rows
        ):
            raise SystemExit("committed ledger rows must be fictional single-currency cash flows")
    if sorted(json_rows, key=lambda row: row["transaction_id"]) != sorted(
        csv_rows, key=lambda row: row["transaction_id"]
    ):
        raise SystemExit("committed JSON and CSV ledger fixtures must be equivalent")

    private_manifest = PRIVATE_FEED_MANIFEST.read_text(encoding="utf-8")
    for private_name in (
        "APCA_API_KEY_ID",
        "APCA_API_SECRET_KEY",
        "MARKET_WATCHLIST",
        "MARKET_TENANT_ID",
    ):
        secret_reference = (
            f"            - name: {private_name}\n"
            "              valueFrom:\n"
            "                secretKeyRef:\n"
            "                  name: alpaca-market-feed\n"
            f"                  key: {private_name}\n"
        )
        if private_manifest.count(secret_reference) != 1:
            raise SystemExit(f"{private_name} must remain a runtime Secret reference")

    print("Committed portfolio, ledger, scale, and capacity fixtures are explicitly fictional; private feed inputs remain Secret-backed.")


if __name__ == "__main__":
    main()
