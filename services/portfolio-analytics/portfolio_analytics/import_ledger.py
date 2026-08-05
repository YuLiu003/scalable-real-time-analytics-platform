from __future__ import annotations

import argparse
import json
import os
import tempfile
from pathlib import Path

from .ledger import build_ledger


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Import an offline cash-flow ledger")
    parser.add_argument("--input-format", required=True, choices=("json", "csv"))
    parser.add_argument("--portfolio", required=True)
    parser.add_argument("--currency", required=True)
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    return parser


def _write_atomic(output: Path, data: bytes) -> None:
    descriptor, temporary_name = tempfile.mkstemp(prefix=f".{output.name}.", dir=output.parent)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            os.fchmod(stream.fileno(), 0o600)
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary_name, output)
    except BaseException:
        Path(temporary_name).unlink(missing_ok=True)
        raise


def main() -> None:
    arguments = _parser().parse_args()
    ledger = build_ledger(
        arguments.input.read_bytes(),
        arguments.input_format,
        arguments.portfolio,
        arguments.currency,
    )
    transaction_count = len(json.loads(ledger)["transactions"])
    _write_atomic(arguments.output, ledger)
    print(
        json.dumps(
            {
                "event": "cash-flow ledger imported",
                "transaction_count": transaction_count,
            },
            separators=(",", ":"),
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    main()
