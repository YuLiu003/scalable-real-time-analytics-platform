from __future__ import annotations

import json
import os
import re
import tempfile
from pathlib import Path

from .analytics import build_products
from .storage import ObjectStore, S3Settings


def main() -> None:
    holdings_file = Path(os.environ.get("HOLDINGS_FILE", "/config/holdings.json"))
    bronze_prefix = os.environ.get("BRONZE_PREFIX", "bronze/market.price.observed/v1/")
    bronze_source = os.environ.get("BRONZE_SOURCE", "synthetic")
    if re.fullmatch(r"[a-z0-9][a-z0-9._-]{0,31}", bronze_source) is None:
        raise ValueError("BRONZE_SOURCE must be a canonical source slug")
    work_root = Path(os.environ.get("WORK_ROOT", "/work"))
    holdings_data = holdings_file.read_bytes()
    store = ObjectStore(S3Settings.from_environment())
    objects = store.list_objects(bronze_prefix, f"/source={bronze_source}/")

    with tempfile.TemporaryDirectory(prefix="portfolio-analytics-", dir=work_root) as temporary:
        products = build_products(objects, holdings_data, Path(temporary))
        silver_data = products["silver_file"].read_bytes()
        gold_data = products["gold_file"].read_bytes()
        silver_result = store.put_immutable(products["silver_key"], silver_data, "application/vnd.apache.parquet")
        gold_result = store.put_immutable(products["gold_key"], gold_data, "application/vnd.apache.parquet")
        store.put_latest(products["latest_key"], products["latest_bytes"])

    print(
        json.dumps(
            {
                "event": "portfolio analytics build complete",
                "gold_object": products["gold_key"],
                "gold_write": gold_result,
                "input_objects": len(objects),
                "input_set_sha256": products["input_set_sha256"],
                "positions": len(products["result"]["positions"]),
                "result_sha256": products["result_sha256"],
                "silver_object": products["silver_key"],
                "silver_write": silver_result,
                "total_market_value": products["result"]["total_market_value"],
            },
            separators=(",", ":"),
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    main()
