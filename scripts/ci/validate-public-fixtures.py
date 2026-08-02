#!/usr/bin/env python3
"""Fail when committed portfolio or scale fixtures are not explicitly fictional."""

from __future__ import annotations

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
DEMO_ID = re.compile(r"^DEMO-(?:ASSET|BENCH)-[A-Z]$")
LOAD_ID = re.compile(r"^LOAD-[A-Z]$")


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

    print("Committed portfolio and scale fixtures are explicitly fictional.")


if __name__ == "__main__":
    main()
