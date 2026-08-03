"""Command-line interface for local review routing and analysis."""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Callable, Sequence
from typing import TextIO

from .analysis import parse_review_jsonl, summarize
from .codex import parse_codex_jsonl
from .model import REVIEWERS, STRATEGIES, ReviewInvocation
from .routing import route_changed_files


def _nonnegative(value: str) -> int:
    parsed = int(value)
    if parsed < 0:
        raise argparse.ArgumentTypeError("value must be non-negative")
    return parsed


def _positive(value: str) -> int:
    parsed = int(value)
    if parsed < 1:
        raise argparse.ArgumentTypeError("value must be positive")
    return parsed


def _write(value: object, output: TextIO) -> None:
    json.dump(value, output, sort_keys=True, separators=(",", ":"))
    output.write("\n")


def _route(args: argparse.Namespace, input_stream: TextIO, output: TextIO) -> None:
    separator = "\0" if args.null else "\n"
    plan = route_changed_files(input_stream.read().split(separator))
    _write(plan.to_mapping(), output)


def _codex_run(
    args: argparse.Namespace, input_stream: TextIO, output: TextIO
) -> None:
    terminal = parse_codex_jsonl(input_stream)
    record = ReviewInvocation(
        run_id=args.run_id,
        invocation_id=args.invocation_id,
        occurred_at=args.occurred_at,
        provider="codex",
        model=args.model,
        strategy=args.strategy,
        reviewer=args.reviewer,
        status=terminal.status,
        duration_ms=args.duration_ms,
        changed_file_count=args.changed_file_count,
        proposed_findings=args.proposed_findings,
        confirmed_findings=args.confirmed_findings,
        false_positive_findings=args.false_positive_findings,
        usage=terminal.usage,
    )
    _write(record.to_mapping(), output)


def _summarize(
    _args: argparse.Namespace, input_stream: TextIO, output: TextIO
) -> None:
    _write(summarize(parse_review_jsonl(input_stream)), output)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="review-optimizer")
    subparsers = parser.add_subparsers(required=True)

    route = subparsers.add_parser("route", help="route changed paths from stdin")
    route.add_argument("--null", action="store_true", help="read NUL-delimited paths")
    route.set_defaults(handler=_route)

    codex_run = subparsers.add_parser(
        "codex-run", help="extract one sanitized Codex review invocation"
    )
    codex_run.add_argument("--run-id", required=True)
    codex_run.add_argument("--invocation-id", required=True)
    codex_run.add_argument("--occurred-at", required=True)
    codex_run.add_argument(
        "--model",
        help="optional public provider model identifier; omit sensitive values",
    )
    codex_run.add_argument("--strategy", choices=STRATEGIES[1:], required=True)
    codex_run.add_argument("--reviewer", choices=REVIEWERS, required=True)
    codex_run.add_argument("--duration-ms", type=_nonnegative, required=True)
    codex_run.add_argument("--changed-file-count", type=_positive, required=True)
    codex_run.add_argument("--proposed-findings", type=_nonnegative, required=True)
    codex_run.add_argument("--confirmed-findings", type=_nonnegative, required=True)
    codex_run.add_argument(
        "--false-positive-findings", type=_nonnegative, required=True
    )
    codex_run.set_defaults(handler=_codex_run)

    report = subparsers.add_parser(
        "summarize", help="aggregate sanitized review invocation JSONL"
    )
    report.set_defaults(handler=_summarize)
    return parser


def main(
    argv: Sequence[str] | None = None,
    input_stream: TextIO | None = None,
    output: TextIO | None = None,
    error: TextIO | None = None,
) -> int:
    args = build_parser().parse_args(argv)
    input_stream = input_stream or sys.stdin
    output = output or sys.stdout
    error = error or sys.stderr
    handler: Callable[[argparse.Namespace, TextIO, TextIO], None] = args.handler
    try:
        handler(args, input_stream, output)
    except ValueError as exc:
        print(f"error: {exc}", file=error)
        return 2
    return 0
