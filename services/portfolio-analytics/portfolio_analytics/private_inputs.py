from __future__ import annotations

import argparse
import json
import re
import stat
import sys
import unicodedata
from pathlib import Path

from .model import INSTRUMENT, parse_portfolio

ENVIRONMENT_KEYS = {
    "APCA_API_KEY_ID",
    "APCA_API_SECRET_KEY",
    "MARKET_TENANT_ID",
    "MARKET_WATCHLIST",
}
PRIVATE_ERROR = '{"event":"input validation failed"}'
TOKEN68 = re.compile(r"^[A-Za-z0-9._~+/-]+=*$")


class _ArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise ValueError("invalid arguments")


def _private_file(raw_path: str, repository_root: Path) -> Path:
    path = Path(raw_path)
    if not path.is_absolute():
        raise ValueError("private file is invalid")
    file_stat = path.lstat()
    if (
        stat.S_ISLNK(file_stat.st_mode)
        or not stat.S_ISREG(file_stat.st_mode)
        or stat.S_IMODE(file_stat.st_mode) != 0o600
    ):
        raise ValueError("private file is invalid")
    resolved = path.resolve()
    if resolved == repository_root or repository_root in resolved.parents:
        raise ValueError("private file is invalid")
    return path


def _environment(data: bytes) -> dict[str, str]:
    text = data.decode("utf-8")
    values: dict[str, str] = {}
    for raw_line in text.splitlines():
        if not raw_line.strip() or raw_line.lstrip().startswith("#"):
            continue
        if "=" not in raw_line:
            raise ValueError("private environment is invalid")
        name, value = raw_line.split("=", 1)
        if name not in ENVIRONMENT_KEYS or name in values:
            raise ValueError("private environment is invalid")
        values[name] = value
    if set(values) != ENVIRONMENT_KEYS:
        raise ValueError("private environment is invalid")
    if not _secret(values["APCA_API_KEY_ID"]) or not _secret(values["APCA_API_SECRET_KEY"]):
        raise ValueError("private environment is invalid")
    return values


def _secret(value: str) -> bool:
    return 0 < len(value) <= 512 and all(
        not character.isspace() and unicodedata.category(character) != "Cc"
        for character in value
    )


def _watchlist(value: str) -> tuple[str, ...]:
    items = tuple(item.strip() for item in value.split(","))
    if not 1 <= len(items) <= 30 or len(set(items)) != len(items):
        raise ValueError("private watchlist is invalid")
    if any(INSTRUMENT.fullmatch(item) is None for item in items):
        raise ValueError("private watchlist is invalid")
    return items


def _access_token(data: bytes) -> None:
    token = data.decode("utf-8").removesuffix("\n")
    if not 32 <= len(token) <= 512 or TOKEN68.fullmatch(token) is None:
        raise ValueError("private access token is invalid")


def _validate(arguments: argparse.Namespace) -> dict[str, int | str]:
    repository_root = Path(arguments.repository_root)
    if not repository_root.is_absolute() or not repository_root.is_dir():
        raise ValueError("repository root is invalid")
    repository_root = repository_root.resolve()
    feed_environment = _private_file(arguments.feed_environment, repository_root)
    holdings_path = _private_file(arguments.holdings, repository_root)
    access_token_path = _private_file(arguments.access_token, repository_root)

    environment = _environment(feed_environment.read_bytes())
    portfolio = parse_portfolio(holdings_path.read_bytes())
    watchlist = _watchlist(environment["MARKET_WATCHLIST"])
    _access_token(access_token_path.read_bytes())

    if (
        portfolio.base_currency != "USD"
        or portfolio.portfolio_id != "private"
        or environment["MARKET_TENANT_ID"] != "private"
    ):
        raise ValueError("private portfolio is invalid")
    definitions = (*portfolio.positions, portfolio.benchmark)
    if any(
        item.asset_type not in {"stock", "etf"} or item.valuation_type != "market_price"
        for item in definitions
    ):
        raise ValueError("private portfolio is invalid")
    if set(watchlist) != {item.instrument for item in definitions}:
        raise ValueError("private portfolio is invalid")
    return {
        "event": "private inputs validated",
        "position_count": len(portfolio.positions),
        "watchlist_count": len(watchlist),
    }


def _arguments() -> argparse.Namespace:
    parser = _ArgumentParser()
    parser.add_argument("--feed-environment", required=True)
    parser.add_argument("--holdings", required=True)
    parser.add_argument("--access-token", required=True)
    parser.add_argument("--repository-root", required=True)
    return parser.parse_args()


def main() -> None:
    try:
        result = _validate(_arguments())
    except Exception:
        print(PRIVATE_ERROR, file=sys.stderr)
        raise SystemExit(1) from None
    print(json.dumps(result, separators=(",", ":"), sort_keys=True))


if __name__ == "__main__":
    main()
