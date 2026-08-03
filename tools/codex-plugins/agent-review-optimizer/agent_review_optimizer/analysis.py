"""Observed aggregate review-efficiency summaries."""

from __future__ import annotations

import json
import math
import statistics
from collections import defaultdict
from collections.abc import Iterable
from typing import Any

from .model import ReviewInvocation, SCHEMA_VERSION


def parse_review_jsonl(lines: Iterable[str]) -> list[ReviewInvocation]:
    records: list[ReviewInvocation] = []
    for line_number, line in enumerate(lines, start=1):
        if not line.strip():
            continue
        try:
            value: Any = json.loads(line)
        except json.JSONDecodeError:
            raise ValueError(f"invalid review JSONL at line {line_number}") from None
        if not isinstance(value, dict):
            raise ValueError(f"review record at line {line_number} must be an object")
        records.append(ReviewInvocation.from_mapping(value))
    if not records:
        raise ValueError("at least one review record is required")
    return records


def _ratio(numerator: int, denominator: int) -> float | None:
    return round(numerator / denominator, 4) if denominator else None


def _p95(values: list[int]) -> int:
    return sorted(values)[math.ceil(len(values) * 0.95) - 1]


def summarize(records: list[ReviewInvocation]) -> dict[str, Any]:
    invocation_ids = {record.invocation_id for record in records}
    if len(invocation_ids) != len(records):
        raise ValueError("duplicate invocation_id")

    groups: dict[tuple[str, str, str, str | None], list[ReviewInvocation]] = (
        defaultdict(list)
    )
    for record in records:
        groups[(record.strategy, record.reviewer, record.provider, record.model)].append(
            record
        )

    cohorts: list[dict[str, Any]] = []
    for (strategy, reviewer, provider, model), cohort in sorted(
        groups.items(), key=lambda item: (*item[0][:3], item[0][3] or "")
    ):
        proposed = sum(record.proposed_findings for record in cohort)
        confirmed = sum(record.confirmed_findings for record in cohort)
        false_positive = sum(record.false_positive_findings for record in cohort)
        usages = [record.usage for record in cohort if record.usage is not None]
        usage_complete = len(usages) == len(cohort)
        input_tokens = sum(usage.input_tokens for usage in usages)
        cached_tokens = sum(usage.cached_input_tokens for usage in usages)
        output_tokens = sum(usage.output_tokens for usage in usages)
        reasoning_tokens = [usage.reasoning_output_tokens for usage in usages]
        reasoning_complete = usage_complete and all(
            value is not None for value in reasoning_tokens
        )
        measured_tokens = input_tokens - cached_tokens + output_tokens
        durations = [record.duration_ms for record in cohort]
        cohorts.append(
            {
                "strategy": strategy,
                "reviewer": reviewer,
                "provider": provider,
                "model": model,
                "invocations": len(cohort),
                "completed_invocations": sum(
                    record.status == "completed" for record in cohort
                ),
                "zero_yield_invocations": sum(
                    record.status == "completed" and record.confirmed_findings == 0
                    for record in cohort
                ),
                "median_duration_ms": statistics.median(durations),
                "p95_duration_ms": _p95(durations),
                "usage_measured_invocations": len(usages),
                "input_tokens": input_tokens if usage_complete else None,
                "cached_input_tokens": cached_tokens if usage_complete else None,
                "uncached_input_tokens": (
                    input_tokens - cached_tokens if usage_complete else None
                ),
                "output_tokens": output_tokens if usage_complete else None,
                "reasoning_output_tokens": (
                    sum(value for value in reasoning_tokens if value is not None)
                    if reasoning_complete
                    else None
                ),
                "proposed_findings": proposed,
                "confirmed_findings": confirmed,
                "false_positive_findings": false_positive,
                "confirmed_finding_rate": _ratio(confirmed, proposed),
                "false_positive_rate": _ratio(false_positive, proposed),
                "uncached_plus_output_tokens_per_confirmed_finding": _ratio(
                    measured_tokens, confirmed
                ) if usage_complete else None,
            }
        )

    return {
        "schema_version": SCHEMA_VERSION,
        "evidence_scope": "sanitized_local_review_records",
        "run_count": len({record.run_id for record in records}),
        "invocation_count": len(records),
        "cohorts": cohorts,
    }
