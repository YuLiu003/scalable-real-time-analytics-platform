from __future__ import annotations

import json
import os
import tempfile
from decimal import Decimal
from pathlib import Path

import duckdb

from .storage import ObjectStore, S3Settings


def main() -> None:
    portfolio = os.environ.get("PORTFOLIO_ID", "demo")
    expected_total = os.environ.get("EXPECTED_TOTAL", "6200.00000000")
    store = ObjectStore(S3Settings.from_environment())
    latest_key = f"gold/portfolio_allocations/v2/portfolio={portfolio}/latest.json"
    result = json.loads(store.get(latest_key))

    with tempfile.TemporaryDirectory(prefix="portfolio-query-", dir=os.environ.get("WORK_ROOT", "/work")) as temporary:
        work = Path(temporary)
        silver = work / "silver.parquet"
        gold = work / "gold.parquet"
        silver.write_bytes(store.get(result["silver_parquet_object"]))
        gold.write_bytes(store.get(result["gold_parquet_object"]))
        connection = duckdb.connect()
        try:
            silver_rows = connection.execute("SELECT count(*) FROM read_parquet(?)", [str(silver)]).fetchone()[0]
            gold_rows, total = connection.execute(
                "SELECT count(*), sum(market_value) FROM read_parquet(?)", [str(gold)]
            ).fetchone()
        finally:
            connection.close()

    formatted_total = format(Decimal(total).quantize(Decimal("0.00000001")), "f")
    if silver_rows != result["input_object_count"] or gold_rows != len(result["positions"]):
        raise ValueError(f"Parquet row mismatch: silver={silver_rows} gold={gold_rows}")
    if formatted_total != expected_total or formatted_total != result["total_market_value"]:
        raise ValueError(f"Parquet total {formatted_total} does not match {expected_total}")
    print(
        json.dumps(
            {
                "event": "DuckDB Parquet query passed",
                "gold_rows": gold_rows,
                "silver_rows": silver_rows,
                "total_market_value": formatted_total,
            },
            separators=(",", ":"),
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    main()
