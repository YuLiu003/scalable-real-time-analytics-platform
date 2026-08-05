from __future__ import annotations

import json
import os
import re
import sys
import tempfile
from pathlib import Path

from .analytics import build_products, latest_required_objects
from .storage import ObjectStore, S3Settings

PRIVATE_DATE_PARTITIONS = 2


def _build(private_mode: bool) -> None:
    holdings_file = Path(os.environ.get("HOLDINGS_FILE", "/config/holdings.json"))
    bronze_prefix = os.environ.get("BRONZE_PREFIX", "bronze/market.price.observed/v1/")
    bronze_source = os.environ.get("BRONZE_SOURCE", "synthetic")
    if re.fullmatch(r"[a-z0-9][a-z0-9._-]{0,31}", bronze_source) is None:
        raise ValueError("BRONZE_SOURCE must be a canonical source slug")
    work_root = Path(os.environ.get("WORK_ROOT", "/work"))
    holdings_data = holdings_file.read_bytes()
    store = ObjectStore(S3Settings.from_environment())
    source_segment = f"/source={bronze_source}/"
    if private_mode:
        objects = store.list_latest_source_objects(
            bronze_prefix,
            source_segment,
            PRIVATE_DATE_PARTITIONS,
        )
        objects = latest_required_objects(objects, holdings_data)
    else:
        objects = store.list_objects(bronze_prefix, source_segment)

    with tempfile.TemporaryDirectory(prefix="portfolio-analytics-", dir=work_root) as temporary:
        products = build_products(objects, holdings_data, Path(temporary))
        silver_data = products["silver_file"].read_bytes()
        gold_data = products["gold_file"].read_bytes()
        silver_result = store.put_immutable(products["silver_key"], silver_data, "application/vnd.apache.parquet")
        gold_result = store.put_immutable(products["gold_key"], gold_data, "application/vnd.apache.parquet")
        store.put_latest(products["latest_key"], products["latest_bytes"])

    result = {
        "event": "portfolio analytics build complete",
        "gold_write": gold_result,
        "input_objects": len(objects),
        "positions": len(products["result"]["positions"]),
        "silver_write": silver_result,
    }
    if not private_mode:
        result["result_sha256"] = products["result_sha256"]
    print(json.dumps(result, separators=(",", ":"), sort_keys=True))


def main() -> None:
    raw_private_mode = os.environ.get("ANALYTICS_PRIVATE_MODE", "false")
    bronze_source = os.environ.get("BRONZE_SOURCE", "synthetic")
    try:
        if raw_private_mode not in {"false", "true"}:
            raise ValueError("ANALYTICS_PRIVATE_MODE must be true or false")
        if bronze_source != "synthetic" and raw_private_mode != "true":
            raise ValueError("non-synthetic analytics must use private mode")
        _build(raw_private_mode == "true")
    except Exception:
        print('{"event":"portfolio analytics build failed"}', file=sys.stderr)
        raise SystemExit(1) from None


if __name__ == "__main__":
    main()
