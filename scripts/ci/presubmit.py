#!/usr/bin/env python3
"""Repository-owned PS0/PS1/PS2 presubmit gates."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Mapping, Sequence


REPO_ROOT = Path(__file__).resolve().parents[2]
STAGES = ("PS0", "PS1", "PS2")
CHECKLIST_CODES = ("SCOPE", "TEST", "SEC", "AUTH", "DATA", "DIST", "INFRA", "OPS")
MAX_SCANNED_BYTES = 2_000_000
REMOTE_ACTION = re.compile(r"^\s*(?:-\s*)?uses:\s*([^\s#]+)")
FULL_COMMIT = re.compile(r"^[0-9a-fA-F]{40}$")
FULL_DIGEST = re.compile(r"^sha256:[0-9a-fA-F]{64}$")
SECRET_MARKERS = (
    re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----"),
    re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    re.compile(r"\bgh[pousr]_[A-Za-z0-9_]{36,}\b"),
)
STAGE_COMMANDS = {
    "PS1": (
        ("make", "-C", "platform/jenkins", "quality"),
        ("make", "-C", "platform/local", "quality"),
        ("tools/codex-plugins/cloud-platform-engineering/scripts/quality.sh",),
        ("scripts/ci/presubmit-quality.sh",),
        ("scripts/ci/validate-workflows.sh",),
    ),
    "PS2": (("scripts/ci/presubmit-ps2.sh",),),
}


class PresubmitError(RuntimeError):
    pass


def run(command: Sequence[str], environ: Mapping[str, str]) -> None:
    subprocess.run(
        list(command),
        cwd=REPO_ROOT,
        env=dict(environ),
        check=True,
    )


def git_output(arguments: Sequence[str], environ: Mapping[str, str]) -> list[str]:
    result = subprocess.run(
        ["git", *arguments],
        cwd=REPO_ROOT,
        env=dict(environ),
        check=True,
        capture_output=True,
        text=True,
    )
    return [value for value in result.stdout.split("\0") if value]


def base_revision(environ: Mapping[str, str]) -> str | None:
    explicit = environ.get("PRESUBMIT_BASE_SHA") or environ.get("PRESUBMIT_BASE_REF")
    if explicit:
        return explicit
    target = environ.get("CHANGE_TARGET")
    return f"origin/{target}" if target else None


def changed_files(environ: Mapping[str, str]) -> list[str]:
    base = base_revision(environ)
    if base:
        return sorted(set(git_output(("diff", "--name-only", "-z", f"{base}...HEAD"), environ)))

    paths: set[str] = set()
    for arguments in (
        ("diff", "--name-only", "-z"),
        ("diff", "--cached", "--name-only", "-z"),
        ("ls-files", "--others", "--exclude-standard", "-z"),
    ):
        paths.update(git_output(arguments, environ))
    return sorted(paths)


def check_diff(environ: Mapping[str, str]) -> None:
    base = base_revision(environ)
    if base:
        run(("git", "diff", "--check", f"{base}...HEAD"), environ)
        return
    run(("git", "diff", "--check"), environ)
    run(("git", "diff", "--cached", "--check"), environ)


def check_workflow_pins(paths: Sequence[str]) -> None:
    failures: list[str] = []
    for relative in paths:
        path = REPO_ROOT / relative
        if (
            not path.is_file()
            or path.parent != REPO_ROOT / ".github" / "workflows"
            or path.suffix not in {".yml", ".yaml"}
        ):
            continue
        for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            match = REMOTE_ACTION.match(line)
            if not match:
                continue
            action = match.group(1)
            if action.startswith("./"):
                continue
            if action.startswith("docker://"):
                _, separator, digest = action.rpartition("@")
                if not separator or not FULL_DIGEST.fullmatch(digest):
                    failures.append(f"{relative}:{line_number}: container action is not digest-pinned")
                continue
            _, separator, revision = action.rpartition("@")
            if not separator or not FULL_COMMIT.fullmatch(revision):
                failures.append(f"{relative}:{line_number}: action is not pinned to a full commit SHA")
    if failures:
        raise PresubmitError("\n".join(failures))


def sensitive_path(path: Path) -> bool:
    lower_name = path.name.lower()
    if lower_name in {".env", "kubeconfig", "id_rsa", "id_ed25519"}:
        return True
    if lower_name.endswith((".pem", ".p12", ".pfx", ".key")):
        return not any(marker in lower_name for marker in ("example", "template", "fixture"))
    if "secret" in lower_name and lower_name.endswith((".yaml", ".yml", ".json")):
        return not any(marker in lower_name for marker in ("example", "template", "fixture"))
    return False


def check_file_safety(paths: Sequence[str]) -> None:
    failures: list[str] = []
    for relative in paths:
        path = REPO_ROOT / relative
        if not path.is_file():
            continue
        if sensitive_path(path):
            failures.append(f"{relative}: sensitive credential filename")
            continue
        if path.stat().st_size > MAX_SCANNED_BYTES:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        if any(pattern.search(text) for pattern in SECRET_MARKERS):
            failures.append(f"{relative}: possible credential material")
    if failures:
        raise PresubmitError("\n".join(failures))


def check_syntax(paths: Sequence[str], environ: Mapping[str, str]) -> None:
    failures: list[str] = []
    for relative in paths:
        path = REPO_ROOT / relative
        if not path.is_file():
            continue
        try:
            if path.suffix == ".json":
                json.loads(path.read_text(encoding="utf-8"))
            elif path.suffix == ".py":
                compile(path.read_text(encoding="utf-8"), str(path), "exec")
            elif path.suffix == ".sh":
                run(("bash", "-n", relative), environ)
        except (OSError, SyntaxError, json.JSONDecodeError, subprocess.CalledProcessError) as exc:
            failures.append(f"{relative}: {exc}")
    if failures:
        raise PresubmitError("\n".join(failures))


def check_jenkins_pipeline(paths: Sequence[str]) -> None:
    if "Jenkinsfile" not in paths:
        return
    text = (REPO_ROOT / "Jenkinsfile").read_text(encoding="utf-8")
    failures: list[str] = []
    for stage in STAGES:
        if f"stage('{stage}')" not in text:
            failures.append(f"Jenkinsfile: missing {stage} stage")
    for forbidden in ("agent any", "withCredentials(", "credentials("):
        if forbidden in text:
            failures.append(f"Jenkinsfile: forbidden untrusted-PR construct: {forbidden}")
    for label in ("jenkins-verify", "jenkins-integration"):
        if label not in text:
            failures.append(f"Jenkinsfile: missing static isolated agent label: {label}")
    for forbidden in ("podTemplate(", "privileged:"):
        if forbidden in text:
            failures.append(f"Jenkinsfile: agent privilege must not be defined by repository code: {forbidden}")
    commit_check = 'test "$(git rev-parse HEAD)" = "$EXPECTED_COMMIT"'
    if text.count(commit_check) != len(STAGES):
        failures.append("Jenkinsfile: every stage must verify the trusted expected commit")
    if failures:
        raise PresubmitError("\n".join(failures))


def pull_request_body(environ: Mapping[str, str]) -> str | None:
    if "PRESUBMIT_PR_BODY" in environ:
        return environ["PRESUBMIT_PR_BODY"]
    event_path = environ.get("GITHUB_EVENT_PATH")
    if not event_path:
        return None
    event = json.loads(Path(event_path).read_text(encoding="utf-8"))
    pull_request = event.get("pull_request")
    if not isinstance(pull_request, dict):
        return None
    body = pull_request.get("body")
    return body if isinstance(body, str) else ""


def check_pr_checklist(environ: Mapping[str, str]) -> None:
    body = pull_request_body(environ)
    required = environ.get("PRESUBMIT_REQUIRE_PR_CHECKLIST", "false").lower() == "true"
    if body is None:
        if required:
            raise PresubmitError("PR checklist is required but no pull-request body was provided")
        return
    missing = [
        code
        for code in CHECKLIST_CODES
        if not re.search(rf"-\s*\[[xX]\]\s*`{code}`(?:\s|—|-)", body)
    ]
    if missing:
        raise PresubmitError("unchecked PR checklist items: " + ", ".join(missing))


def run_ps0(environ: Mapping[str, str]) -> None:
    paths = changed_files(environ)
    workflow_paths = sorted(
        str(path.relative_to(REPO_ROOT))
        for path in (REPO_ROOT / ".github" / "workflows").glob("*.y*ml")
    )
    check_diff(environ)
    check_workflow_pins(workflow_paths)
    check_file_safety(paths)
    check_syntax(paths, environ)
    check_jenkins_pipeline(paths)
    check_pr_checklist(environ)
    print(f"PS0 passed for {len(paths)} changed file(s).")


def run_stage(stage: str, environ: Mapping[str, str]) -> None:
    print(f"Running {stage}...")
    if stage == "PS0":
        run_ps0(environ)
        return
    for command in STAGE_COMMANDS[stage]:
        run(command, environ)
    print(f"{stage} passed.")


def parse_args(arguments: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("stage", choices=(*STAGES, "all"), nargs="?")
    parser.add_argument("--list", action="store_true", dest="list_stages")
    parsed = parser.parse_args(arguments)
    if not parsed.list_stages and parsed.stage is None:
        parser.error("a stage or --list is required")
    return parsed


def main(arguments: Sequence[str] | None = None, environ: Mapping[str, str] | None = None) -> int:
    parsed = parse_args(sys.argv[1:] if arguments is None else arguments)
    if parsed.list_stages:
        print("\n".join(STAGES))
        return 0
    selected = STAGES if parsed.stage == "all" else (parsed.stage,)
    active_environment = os.environ if environ is None else environ
    try:
        for stage in selected:
            run_stage(stage, active_environment)
    except (PresubmitError, subprocess.CalledProcessError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
