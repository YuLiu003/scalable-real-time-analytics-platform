"""Deterministic local-only specialist routing."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import PurePosixPath

from .model import REVIEWERS, SCHEMA_VERSION


_ORDER = tuple(reviewer for reviewer in REVIEWERS if reviewer != "deterministic")


@dataclass(frozen=True)
class ReviewPlan:
    changed_file_count: int
    reviewers: tuple[str, ...]

    def to_mapping(self) -> dict[str, object]:
        return {
            "schema_version": SCHEMA_VERSION,
            "changed_file_count": self.changed_file_count,
            "reviewers": list(self.reviewers),
        }


def _validated_paths(paths: list[str]) -> list[str]:
    unique: list[str] = []
    seen: set[str] = set()
    for path in paths:
        if not path:
            continue
        parsed = PurePosixPath(path)
        if "\0" in path or parsed.is_absolute() or ".." in parsed.parts:
            raise ValueError("changed paths must be relative repository paths")
        if path not in seen:
            seen.add(path)
            unique.append(path)
    if not unique:
        raise ValueError("at least one changed path is required")
    return unique


def route_changed_files(paths: list[str]) -> ReviewPlan:
    validated = _validated_paths(paths)
    selected = {"general"}
    for path in validated:
        lowered = path.lower()
        parsed = PurePosixPath(lowered)
        name = parsed.name
        suffix = parsed.suffix
        if suffix == ".go":
            selected.add("go")
        if suffix == ".py":
            selected.add("python")
        if (
            lowered.startswith("contracts/events/")
            or lowered.startswith("services/market-pipeline/")
            or "kafka" in lowered
            or "keda" in lowered
            or "strimzi" in lowered
        ):
            selected.add("event-driven")
        if (
            lowered.startswith("platform/gitops/")
            or lowered.startswith("k8s/")
            or name in {"kustomization.yaml", "kustomization.yml"}
        ):
            selected.add("kubernetes")
        if lowered.startswith("infra/opentofu/") or suffix in {".tf", ".tofu"}:
            selected.add("cloud-iac")
        if (
            name == "jenkinsfile"
            or lowered.startswith(".github/workflows/")
            or lowered.startswith("platform/jenkins/")
            or lowered.startswith("scripts/ci/")
        ):
            selected.add("ci-security")
        if (
            lowered.startswith("services/portfolio-")
            or lowered.startswith("contracts/fixtures/")
            or any(term in lowered for term in ("investment", "market", "portfolio"))
        ):
            selected.add("privacy-finance")
    return ReviewPlan(
        changed_file_count=len(validated),
        reviewers=tuple(reviewer for reviewer in _ORDER if reviewer in selected),
    )
