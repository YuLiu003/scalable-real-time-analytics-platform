from __future__ import annotations

import hashlib
from datetime import timezone
from decimal import Decimal
from pathlib import Path

import duckdb

from .model import BronzeObject, canonical_json, input_set_sha256, parse_portfolio, parse_price


def _decimal_string(value: Decimal, scale: str) -> str:
    return format(value.quantize(Decimal(scale)), "f")


def _rfc3339(value) -> str:
    return value.astimezone(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")


def build_products(objects: list[BronzeObject], holdings_data: bytes, output_dir: Path) -> dict:
    if not objects:
        raise ValueError("no bronze market-price objects found")
    portfolio = parse_portfolio(holdings_data)
    observations = [parse_price(obj) for obj in sorted(objects, key=lambda item: item.key)]
    if len({item.event_id for item in observations}) != len(observations):
        raise ValueError("bronze input contains duplicate event_id values")
    unexpected_tenants = sorted({item.tenant_id for item in observations} - {portfolio.portfolio_id})
    if unexpected_tenants:
        raise ValueError(f"bronze input contains unexpected tenant_id values: {','.join(unexpected_tenants)}")

    input_hash = input_set_sha256(holdings_data, objects)
    output_dir.mkdir(parents=True, exist_ok=True)
    silver_file = output_dir / "market-prices.parquet"
    gold_file = output_dir / "portfolio-allocation.parquet"

    connection = duckdb.connect()
    try:
        connection.execute(
            """
            CREATE TABLE silver_market_prices (
                event_id VARCHAR,
                source VARCHAR,
                tenant_id VARCHAR,
                occurred_at TIMESTAMPTZ,
                ingested_at TIMESTAMPTZ,
                instrument VARCHAR,
                currency VARCHAR,
                price DECIMAL(38, 8),
                provider_sequence BIGINT,
                object_key VARCHAR
            )
            """
        )
        connection.executemany(
            "INSERT INTO silver_market_prices VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
            [
                (
                    item.event_id,
                    item.source,
                    item.tenant_id,
                    item.occurred_at,
                    item.ingested_at,
                    item.instrument,
                    item.currency,
                    item.price,
                    item.provider_sequence,
                    item.object_key,
                )
                for item in observations
            ],
        )
        connection.execute(
            """
            CREATE TABLE holdings (
                instrument VARCHAR PRIMARY KEY,
                display_name VARCHAR,
                asset_type VARCHAR,
                valuation_type VARCHAR,
                quantity DECIMAL(38, 8)
            )
            """
        )
        connection.executemany(
            "INSERT INTO holdings VALUES (?, ?, ?, ?, ?)",
            [
                (
                    item.instrument,
                    item.display_name,
                    item.asset_type,
                    item.valuation_type,
                    item.quantity,
                )
                for item in portfolio.positions
            ],
        )
        connection.execute("CREATE TABLE required_instruments (instrument VARCHAR PRIMARY KEY)")
        connection.executemany(
            "INSERT INTO required_instruments VALUES (?)",
            [(item.instrument,) for item in portfolio.positions] + [(portfolio.benchmark.instrument,)],
        )

        missing = [
            row[0]
            for row in connection.execute(
                """
                WITH latest AS (
                    SELECT *, row_number() OVER (
                        PARTITION BY instrument
                        ORDER BY occurred_at DESC, provider_sequence DESC, event_id DESC
                    ) AS rank
                    FROM silver_market_prices
                )
                SELECT required_instruments.instrument
                FROM required_instruments
                LEFT JOIN latest ON latest.instrument = required_instruments.instrument AND latest.rank = 1
                WHERE latest.instrument IS NULL
                ORDER BY required_instruments.instrument
                """
            ).fetchall()
        ]
        if missing:
            raise ValueError(f"holdings lack a market price: {','.join(missing)}")

        mismatched = [
            row[0]
            for row in connection.execute(
                """
                WITH latest AS (
                    SELECT *, row_number() OVER (
                        PARTITION BY instrument
                        ORDER BY occurred_at DESC, provider_sequence DESC, event_id DESC
                    ) AS rank
                    FROM silver_market_prices
                )
                SELECT required_instruments.instrument
                FROM required_instruments
                JOIN latest ON latest.instrument = required_instruments.instrument AND latest.rank = 1
                WHERE latest.currency <> ?
                ORDER BY required_instruments.instrument
                """,
                [portfolio.base_currency],
            ).fetchall()
        ]
        if mismatched:
            raise ValueError(f"holdings require FX conversion: {','.join(mismatched)}")

        connection.execute(
            """
            CREATE TABLE gold_allocation AS
            WITH latest AS (
                SELECT *, row_number() OVER (
                    PARTITION BY instrument
                    ORDER BY occurred_at DESC, provider_sequence DESC, event_id DESC
                ) AS rank
                FROM silver_market_prices
            ), valued AS (
                SELECT
                    ?::VARCHAR AS portfolio_id,
                    ?::VARCHAR AS portfolio_display_name,
                    ?::VARCHAR AS base_currency,
                    holdings.instrument,
                    holdings.display_name,
                    holdings.asset_type,
                    holdings.valuation_type,
                    holdings.quantity,
                    latest.price,
                    (holdings.quantity * latest.price)::DECIMAL(38, 8) AS market_value,
                    latest.occurred_at AS price_as_of,
                    ?::VARCHAR AS input_set_sha256
                FROM holdings
                JOIN latest ON latest.instrument = holdings.instrument AND latest.rank = 1
            )
            SELECT
                portfolio_id,
                portfolio_display_name,
                base_currency,
                instrument,
                display_name,
                asset_type,
                valuation_type,
                quantity,
                price,
                market_value,
                round(market_value / sum(market_value) OVER () * 100, 4)::DECIMAL(9, 4) AS allocation_pct,
                price_as_of,
                input_set_sha256
            FROM valued
            ORDER BY instrument
            """,
            [portfolio.portfolio_id, portfolio.display_name, portfolio.base_currency, input_hash],
        )
        silver_path = str(silver_file).replace("'", "''")
        gold_path = str(gold_file).replace("'", "''")
        connection.execute(
            f"COPY (SELECT * FROM silver_market_prices ORDER BY occurred_at, event_id) TO '{silver_path}' (FORMAT PARQUET, COMPRESSION ZSTD)"
        )
        connection.execute(f"COPY gold_allocation TO '{gold_path}' (FORMAT PARQUET, COMPRESSION ZSTD)")
        rows = connection.execute(
            """
            SELECT instrument, display_name, asset_type, valuation_type, quantity, price, market_value, allocation_pct, price_as_of
            FROM gold_allocation ORDER BY instrument
            """
        ).fetchall()
        benchmark_row = connection.execute(
            """
            WITH latest AS (
                SELECT *, row_number() OVER (
                    PARTITION BY instrument
                    ORDER BY occurred_at DESC, provider_sequence DESC, event_id DESC
                ) AS rank
                FROM silver_market_prices
            )
            SELECT instrument, ?, ?, ?, price, occurred_at
            FROM latest
            WHERE instrument = ? AND rank = 1
            """,
            [
                portfolio.benchmark.display_name,
                portfolio.benchmark.asset_type,
                portfolio.benchmark.valuation_type,
                portfolio.benchmark.instrument,
            ],
        ).fetchone()
    finally:
        connection.close()

    as_of = max([row[8] for row in rows] + [benchmark_row[5]])
    total = sum((row[6] for row in rows), Decimal("0"))
    silver_key = f"silver/market_prices/v1/run={input_hash}/part-00000.parquet"
    gold_key = f"gold/portfolio_allocations/v2/portfolio={portfolio.portfolio_id}/run={input_hash}/allocation.parquet"
    latest_key = f"gold/portfolio_allocations/v2/portfolio={portfolio.portfolio_id}/latest.json"
    result = {
        "as_of": _rfc3339(as_of),
        "base_currency": portfolio.base_currency,
        "benchmark": {
            "asset_type": benchmark_row[2],
            "display_name": benchmark_row[1],
            "instrument": benchmark_row[0],
            "price": _decimal_string(benchmark_row[4], "0.00000001"),
            "price_as_of": _rfc3339(benchmark_row[5]),
            "valuation_type": benchmark_row[3],
        },
        "display_name": portfolio.display_name,
        "gold_parquet_object": gold_key,
        "input_object_count": len(objects),
        "input_set_sha256": input_hash,
        "portfolio_id": portfolio.portfolio_id,
        "positions": [
            {
                "allocation_pct": _decimal_string(row[7], "0.0001"),
                "asset_type": row[2],
                "display_name": row[1],
                "instrument": row[0],
                "market_value": _decimal_string(row[6], "0.00000001"),
                "price": _decimal_string(row[5], "0.00000001"),
                "price_as_of": _rfc3339(row[8]),
                "quantity": _decimal_string(row[4], "0.00000001"),
                "valuation_type": row[3],
            }
            for row in rows
        ],
        "schema_version": 2,
        "silver_parquet_object": silver_key,
        "total_market_value": _decimal_string(total, "0.00000001"),
    }
    result_bytes = canonical_json(result)
    return {
        "gold_file": gold_file,
        "gold_key": gold_key,
        "input_set_sha256": input_hash,
        "latest_bytes": result_bytes,
        "latest_key": latest_key,
        "result": result,
        "result_sha256": hashlib.sha256(result_bytes).hexdigest(),
        "silver_file": silver_file,
        "silver_key": silver_key,
    }
