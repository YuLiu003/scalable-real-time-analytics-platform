"""Canonical provider-neutral review records."""

from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Mapping
from uuid import UUID


SCHEMA_VERSION = 1
STRATEGIES = ("deterministic-only", "routed", "broadcast")
REVIEWERS = (
    "deterministic",
    "general",
    "go",
    "python",
    "event-driven",
    "kubernetes",
    "cloud-iac",
    "ci-security",
    "privacy-finance",
)
STATUSES = ("completed", "failed")
PROVIDERS = ("none", "codex")
_MODEL = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$")
_USAGE_FIELDS = {
    "input_tokens",
    "cached_input_tokens",
    "output_tokens",
    "reasoning_output_tokens",
}
_RECORD_FIELDS = {
    "schema_version",
    "run_id",
    "invocation_id",
    "occurred_at",
    "provider",
    "model",
    "strategy",
    "reviewer",
    "status",
    "duration_ms",
    "changed_file_count",
    "proposed_findings",
    "confirmed_findings",
    "false_positive_findings",
    "usage",
}


def _nonnegative_int(value: Any, field: str) -> int:
    if type(value) is not int or value < 0:
        raise ValueError(f"{field} must be a non-negative integer")
    return value


def _canonical_uuid(value: Any, field: str) -> str:
    if not isinstance(value, str):
        raise ValueError(f"{field} must be a canonical UUID")
    try:
        parsed = UUID(value)
    except ValueError:
        raise ValueError(f"{field} must be a canonical UUID") from None
    if str(parsed) != value or parsed.version != 4:
        raise ValueError(f"{field} must be a canonical UUIDv4")
    return value


def _canonical_timestamp(value: Any) -> str:
    if not isinstance(value, str):
        raise ValueError("occurred_at must be an RFC3339 timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        raise ValueError("occurred_at must be an RFC3339 timestamp") from None
    if parsed.tzinfo is None:
        raise ValueError("occurred_at must include a timezone")
    return parsed.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


@dataclass(frozen=True)
class TokenUsage:
    input_tokens: int
    cached_input_tokens: int
    output_tokens: int
    reasoning_output_tokens: int | None

    def __post_init__(self) -> None:
        for field in _USAGE_FIELDS:
            value = getattr(self, field)
            if field == "reasoning_output_tokens" and value is None:
                continue
            _nonnegative_int(value, field)
        if self.cached_input_tokens > self.input_tokens:
            raise ValueError("cached_input_tokens cannot exceed input_tokens")

    @classmethod
    def from_mapping(cls, value: Mapping[str, Any]) -> TokenUsage:
        if set(value) != _USAGE_FIELDS:
            raise ValueError("usage must contain only the canonical token fields")
        return cls(**{field: value[field] for field in _USAGE_FIELDS})

    def to_mapping(self) -> dict[str, int | None]:
        return {
            "input_tokens": self.input_tokens,
            "cached_input_tokens": self.cached_input_tokens,
            "output_tokens": self.output_tokens,
            "reasoning_output_tokens": self.reasoning_output_tokens,
        }

    def __add__(self, other: TokenUsage) -> TokenUsage:
        reasoning_output_tokens = (
            self.reasoning_output_tokens + other.reasoning_output_tokens
            if self.reasoning_output_tokens is not None
            and other.reasoning_output_tokens is not None
            else None
        )
        return TokenUsage(
            input_tokens=self.input_tokens + other.input_tokens,
            cached_input_tokens=self.cached_input_tokens + other.cached_input_tokens,
            output_tokens=self.output_tokens + other.output_tokens,
            reasoning_output_tokens=reasoning_output_tokens,
        )


@dataclass(frozen=True)
class ReviewInvocation:
    run_id: str
    invocation_id: str
    occurred_at: str
    provider: str
    model: str | None
    strategy: str
    reviewer: str
    status: str
    duration_ms: int
    changed_file_count: int
    proposed_findings: int
    confirmed_findings: int
    false_positive_findings: int
    usage: TokenUsage | None

    def __post_init__(self) -> None:
        _canonical_uuid(self.run_id, "run_id")
        _canonical_uuid(self.invocation_id, "invocation_id")
        object.__setattr__(self, "occurred_at", _canonical_timestamp(self.occurred_at))
        if self.provider not in PROVIDERS:
            raise ValueError("unsupported provider")
        if self.model is not None and (
            not isinstance(self.model, str) or not _MODEL.fullmatch(self.model)
        ):
            raise ValueError("model must be null or a provider model identifier")
        if self.strategy not in STRATEGIES:
            raise ValueError("unsupported review strategy")
        if self.reviewer not in REVIEWERS:
            raise ValueError("unsupported reviewer")
        if self.status not in STATUSES:
            raise ValueError("unsupported review status")
        _nonnegative_int(self.duration_ms, "duration_ms")
        if type(self.changed_file_count) is not int or self.changed_file_count < 1:
            raise ValueError("changed_file_count must be a positive integer")
        _nonnegative_int(self.proposed_findings, "proposed_findings")
        _nonnegative_int(self.confirmed_findings, "confirmed_findings")
        _nonnegative_int(self.false_positive_findings, "false_positive_findings")
        adjudicated = self.confirmed_findings + self.false_positive_findings
        if self.status == "completed" and adjudicated != self.proposed_findings:
            raise ValueError("completed records require full finding adjudication")
        if self.status == "failed" and (
            self.proposed_findings or self.confirmed_findings or self.false_positive_findings
        ):
            raise ValueError("failed records cannot declare findings")
        if self.usage is not None and not isinstance(self.usage, TokenUsage):
            raise ValueError("usage must be null or canonical token usage")
        if self.status == "completed" and self.usage is None:
            raise ValueError("completed records require token usage")
        if self.strategy == "deterministic-only":
            if self.provider != "none":
                raise ValueError("deterministic-only records require provider none")
            if self.model is not None:
                raise ValueError("deterministic-only records cannot declare a model")
            if self.reviewer != "deterministic":
                raise ValueError("deterministic-only records require deterministic reviewer")
            if self.usage is None or self.usage.to_mapping() != {
                field: 0 for field in _USAGE_FIELDS
            }:
                raise ValueError("deterministic-only records require zero token usage")
        else:
            if self.provider == "none":
                raise ValueError("agent review records require a provider")
            if self.reviewer == "deterministic":
                raise ValueError("agent review records require an agent reviewer")

    @classmethod
    def from_mapping(cls, value: Mapping[str, Any]) -> ReviewInvocation:
        if set(value) != _RECORD_FIELDS:
            raise ValueError("review record contains missing or unsupported fields")
        if type(value["schema_version"]) is not int or value["schema_version"] != SCHEMA_VERSION:
            raise ValueError("unsupported review record schema_version")
        usage = value["usage"]
        if usage is not None and not isinstance(usage, Mapping):
            raise ValueError("usage must be null or an object")
        return cls(
            run_id=value["run_id"],
            invocation_id=value["invocation_id"],
            occurred_at=value["occurred_at"],
            provider=value["provider"],
            model=value["model"],
            strategy=value["strategy"],
            reviewer=value["reviewer"],
            status=value["status"],
            duration_ms=value["duration_ms"],
            changed_file_count=value["changed_file_count"],
            proposed_findings=value["proposed_findings"],
            confirmed_findings=value["confirmed_findings"],
            false_positive_findings=value["false_positive_findings"],
            usage=TokenUsage.from_mapping(usage) if usage is not None else None,
        )

    def to_mapping(self) -> dict[str, Any]:
        return {
            "schema_version": SCHEMA_VERSION,
            "run_id": self.run_id,
            "invocation_id": self.invocation_id,
            "occurred_at": self.occurred_at,
            "provider": self.provider,
            "model": self.model,
            "strategy": self.strategy,
            "reviewer": self.reviewer,
            "status": self.status,
            "duration_ms": self.duration_ms,
            "changed_file_count": self.changed_file_count,
            "proposed_findings": self.proposed_findings,
            "confirmed_findings": self.confirmed_findings,
            "false_positive_findings": self.false_positive_findings,
            "usage": self.usage.to_mapping() if self.usage is not None else None,
        }
