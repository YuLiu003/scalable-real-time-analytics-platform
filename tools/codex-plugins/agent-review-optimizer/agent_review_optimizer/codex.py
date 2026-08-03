"""Codex JSONL adapter that retains usage counters only."""

from __future__ import annotations

import json
from collections.abc import Iterable
from dataclasses import dataclass
from typing import Any

from .model import TokenUsage


_CODEX_USAGE_FIELDS = {
    "input_tokens",
    "cached_input_tokens",
    "output_tokens",
    "reasoning_output_tokens",
}
_CODEX_REQUIRED_USAGE_FIELDS = _CODEX_USAGE_FIELDS - {"reasoning_output_tokens"}


@dataclass(frozen=True)
class CodexTerminal:
    status: str
    usage: TokenUsage | None


def _usage(event: dict[str, Any], line_number: int, required: bool) -> TokenUsage | None:
    raw_usage = event.get("usage")
    if raw_usage is None and not required:
        return None
    if not isinstance(raw_usage, dict) or not _CODEX_REQUIRED_USAGE_FIELDS.issubset(
        raw_usage
    ):
        raise ValueError(f"missing Codex usage at line {line_number}")
    return TokenUsage.from_mapping(
        {field: raw_usage.get(field) for field in _CODEX_USAGE_FIELDS}
    )


def parse_codex_jsonl(lines: Iterable[str]) -> CodexTerminal:
    terminal: CodexTerminal | None = None
    for line_number, line in enumerate(lines, start=1):
        if not line.strip():
            continue
        try:
            event: Any = json.loads(line)
        except json.JSONDecodeError:
            raise ValueError(f"invalid Codex JSONL at line {line_number}") from None
        if not isinstance(event, dict) or not isinstance(event.get("type"), str):
            raise ValueError(f"invalid Codex event at line {line_number}")
        event_type = event["type"]
        if event_type not in {"turn.completed", "turn.failed"}:
            continue
        if terminal is not None:
            raise ValueError("Codex JSONL contains multiple terminal turn events")
        if event_type == "turn.completed":
            terminal = CodexTerminal("completed", _usage(event, line_number, True))
        else:
            terminal = CodexTerminal("failed", _usage(event, line_number, False))
    if terminal is None:
        raise ValueError("Codex JSONL contains no terminal turn event")
    return terminal
