#!/usr/bin/env python3

import argparse
import hashlib
import json
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Sequence


ROOT = Path(__file__).resolve().parents[3]
REPOSITORY = "YuLiu003/scalable-real-time-analytics-platform"
CONTEXT = "jenkins / presubmit"
MARKER = "<!-- jenkins-presubmit-evidence -->"


class ReportError(RuntimeError):
    pass


def command(args: Sequence[str]) -> str:
    try:
        result = subprocess.run(args, capture_output=True, text=True, check=False)
    except OSError as exc:
        raise ReportError(f"{args[0]}: {exc}") from exc
    if result.returncode:
        detail = result.stderr.strip() or "command failed"
        raise ReportError(f"{args[0]}: {detail}")
    return result.stdout.strip()


def command_json(args: Sequence[str]) -> object:
    try:
        return json.loads(command(args))
    except json.JSONDecodeError as exc:
        raise ReportError(f"{args[0]} returned invalid JSON") from exc


def validate_evidence(path: Path, commit: str) -> str:
    if not path.is_absolute() or path.is_symlink() or not path.is_file():
        raise ReportError("evidence log must be an absolute, regular, non-symlink file")
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        raise ReportError(f"cannot read evidence log: {exc}") from exc
    markers = (
        f"Jenkins PS0, PS1, and PS2 pipeline passed for {commit}.",
        "Disposable Jenkins production-like validation passed; deleting ",
        "Deleted `colima-investment-platform-jenkins-ephemeral`",
        "msg=done",
    )
    position = -1
    for marker in markers:
        position = text.find(marker, position + 1)
        if position < 0:
            raise ReportError(f"evidence log is missing ordered marker: {marker}")
    segment = text[: position + len(markers[-1])]
    if "ERROR: Jenkins" in segment or "make: ***" in segment:
        raise ReportError("evidence log contains a Jenkins or Make failure before cleanup")
    return hashlib.sha256(segment.encode()).hexdigest()


def validate_pull_request(number: int, commit: str) -> dict[str, object]:
    data = command_json(
        (
            "gh",
            "pr",
            "view",
            str(number),
            "--repo",
            REPOSITORY,
            "--json",
            "baseRefName,headRefName,headRefOid,state,url",
        )
    )
    if not isinstance(data, dict):
        raise ReportError("GitHub returned an invalid pull-request object")
    if data.get("state") != "OPEN" or data.get("baseRefName") != "main":
        raise ReportError("status target must be an open pull request into main")
    if data.get("headRefOid") != commit:
        raise ReportError("local HEAD does not match the pull-request head")
    return data


def evidence_body(commit: str, digest: str, timestamp: str) -> str:
    return "\n".join(
        (
            MARKER,
            "### Jenkins presubmit evidence",
            "",
            "- Result: `SUCCESS`",
            f"- Commit: [`{commit[:12]}`](https://github.com/{REPOSITORY}/commit/{commit})",
            "- Gates: `PS0`, `PS1`, `PS2`",
            "- Executor: disposable Jenkins controller with isolated Kubernetes agents",
            "- Cleanup: disposable Colima VM and all container data deleted",
            "- Retention: bounded history and Garage artifacts remain only on the trusted local host",
            f"- Retained operator-log SHA-256: `{digest}`",
            f"- Reported: `{timestamp}`",
            "",
            "The live lab runtime is intentionally deleted after the run; retained local "
            "state is not published or an off-host backup. A production installation "
            "must use durable external storage and a GitHub App.",
        )
    )


def publish_comment(number: int, body: str) -> str:
    login = command(("gh", "api", "user", "--jq", ".login"))
    comments = command_json(
        ("gh", "api", f"repos/{REPOSITORY}/issues/{number}/comments?per_page=100")
    )
    if not isinstance(comments, list):
        raise ReportError("GitHub returned an invalid issue-comment list")
    comment_id = next(
        (
            comment.get("id")
            for comment in comments
            if isinstance(comment, dict)
            and MARKER in str(comment.get("body", ""))
            and isinstance(comment.get("user"), dict)
            and comment["user"].get("login") == login
        ),
        None,
    )
    endpoint = (
        f"repos/{REPOSITORY}/issues/comments/{comment_id}"
        if comment_id is not None
        else f"repos/{REPOSITORY}/issues/{number}/comments"
    )
    response = command_json(
        (
            "gh",
            "api",
            "--method",
            "PATCH" if comment_id is not None else "POST",
            endpoint,
            "-f",
            f"body={body}",
        )
    )
    if not isinstance(response, dict) or not isinstance(response.get("html_url"), str):
        raise ReportError("GitHub did not return the evidence-comment URL")
    return response["html_url"]


def publish_status(commit: str, target_url: str) -> None:
    command(
        (
            "gh",
            "api",
            "--method",
            "POST",
            f"repos/{REPOSITORY}/statuses/{commit}",
            "-f",
            "state=success",
            "-f",
            f"context={CONTEXT}",
            "-f",
            "description=Jenkins PS0, PS1, and PS2 passed",
            "-f",
            f"target_url={target_url}",
        )
    )


def report(number: int, log_path: Path) -> str:
    commit = command(("git", "-C", str(ROOT), "rev-parse", "HEAD"))
    if len(commit) != 40 or any(character not in "0123456789abcdef" for character in commit):
        raise ReportError("local HEAD is not a full lowercase commit SHA")
    validate_pull_request(number, commit)
    digest = validate_evidence(log_path, commit)
    timestamp = datetime.now(UTC).replace(microsecond=0).isoformat()
    comment_url = publish_comment(number, evidence_body(commit, digest, timestamp))
    publish_status(commit, comment_url)
    return f"Published {CONTEXT} success for {commit}."


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Publish trusted Jenkins evidence after a disposable run succeeds."
    )
    parser.add_argument("--pr", type=int, required=True)
    parser.add_argument("--log", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        print(report(args.pr, args.log))
    except ReportError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
