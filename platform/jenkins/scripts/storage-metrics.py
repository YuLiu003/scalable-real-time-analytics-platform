#!/usr/bin/env python3

import argparse
import json
import math
import os
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from typing import Callable, Mapping, Sequence


MEASUREMENT_KEYS = (
    "retained_state_allocated_bytes",
    "colima_profile_allocated_bytes",
)
RESULTS = ("SUCCESS", "FAILURE", "ABORTED", "TIMEOUT", "UNKNOWN")
MAX_INTEGER = (1 << 63) - 1


class MetricError(RuntimeError):
    pass


def validate_directory(path: Path, label: str) -> Path:
    if not path.is_absolute() or path.is_symlink() or not path.is_dir():
        raise MetricError(f"{label} must be an absolute directory without symlinks")
    return path


def validate_input_file(path: Path, label: str) -> Path:
    if not path.is_absolute() or path.is_symlink() or not path.is_file():
        raise MetricError(f"{label} must be an absolute regular file without symlinks")
    return path


def validate_output_file(path: Path, label: str) -> Path:
    if not path.is_absolute() or path.is_symlink():
        raise MetricError(f"{label} must be an absolute regular file without symlinks")
    if not path.parent.is_dir():
        raise MetricError(f"{label} parent must be an existing directory")
    if path.exists():
        validate_input_file(path, label)
    return path


def _nonnegative_integer(value: object, label: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise MetricError(f"{label} must be a non-negative integer")
    if value < 0 or value > MAX_INTEGER:
        raise MetricError(f"{label} must be a non-negative 64-bit integer")
    return value


def parse_nonnegative_integer(text: str) -> int:
    if re.fullmatch(r"0|[1-9][0-9]*", text) is None:
        raise argparse.ArgumentTypeError("value must be a non-negative decimal integer")
    value = int(text)
    if value > MAX_INTEGER:
        raise argparse.ArgumentTypeError("value must fit in a signed 64-bit integer")
    return value


def parse_positive_integer(text: str) -> int:
    value = parse_nonnegative_integer(text)
    if value == 0:
        raise argparse.ArgumentTypeError("value must be positive")
    return value


def parse_interval(text: str) -> float:
    try:
        value = float(text)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("interval must be a positive number") from exc
    if not math.isfinite(value) or value <= 0:
        raise argparse.ArgumentTypeError("interval must be a positive finite number")
    return value


def validate_commit(commit: str) -> str:
    if re.fullmatch(r"[0-9a-f]{40}", commit) is None:
        raise MetricError("commit must be a full lowercase hexadecimal SHA")
    return commit


def _allocated_bytes_for_validated_directory(path: Path) -> int:
    try:
        completed = subprocess.run(
            ("du", "-sk", "--", str(path)),
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            check=False,
        )
    except OSError as exc:
        raise MetricError("du could not be executed") from exc
    if completed.returncode:
        raise MetricError("du failed while sampling allocated storage")
    fields = completed.stdout.split(maxsplit=1)
    if not fields or re.fullmatch(r"[0-9]+", fields[0]) is None:
        raise MetricError("du did not return a numeric block count")
    kibibytes = int(fields[0])
    if kibibytes > MAX_INTEGER // 1024:
        raise MetricError("du result exceeds the supported integer range")
    return kibibytes * 1024


def allocated_bytes(path: Path, label: str = "storage path") -> int:
    return _allocated_bytes_for_validated_directory(validate_directory(path, label))


def sample_paths(retained_state: Path, colima_profile: Path) -> dict[str, int]:
    retained_state = validate_directory(retained_state, "retained-state path")
    colima_profile = validate_directory(colima_profile, "Colima profile path")
    return {
        MEASUREMENT_KEYS[0]: _allocated_bytes_for_validated_directory(retained_state),
        MEASUREMENT_KEYS[1]: _allocated_bytes_for_validated_directory(colima_profile),
    }


def _validate_measurement(data: object, label: str) -> dict[str, int]:
    if not isinstance(data, dict) or set(data) != set(MEASUREMENT_KEYS):
        raise MetricError(f"{label} has an invalid aggregate schema")
    return {
        key: _nonnegative_integer(data[key], f"{label} aggregate")
        for key in MEASUREMENT_KEYS
    }


def _reject_duplicate_keys(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result: dict[str, object] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON key")
        result[key] = value
    return result


def read_measurement(path: Path, label: str) -> dict[str, int]:
    validate_input_file(path, label)
    try:
        text = path.read_text(encoding="utf-8")
        data = json.loads(
            text,
            object_pairs_hook=_reject_duplicate_keys,
            parse_constant=lambda _value: (_ for _ in ()).throw(
                ValueError("non-finite JSON number")
            ),
        )
    except (OSError, UnicodeError, ValueError) as exc:
        raise MetricError(f"{label} is not valid aggregate JSON") from exc
    return _validate_measurement(data, label)


def _atomic_write_json(path: Path, data: Mapping[str, object], label: str) -> None:
    validate_output_file(path, label)
    temporary_name: str | None = None
    try:
        descriptor, temporary_name = tempfile.mkstemp(
            dir=path.parent,
            prefix=f".{path.name}.",
        )
        with os.fdopen(descriptor, "w", encoding="utf-8") as output:
            json.dump(data, output, sort_keys=True, separators=(",", ":"))
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary_name, path)
        temporary_name = None
    except (OSError, TypeError, ValueError) as exc:
        raise MetricError(f"cannot atomically write {label}") from exc
    finally:
        if temporary_name is not None:
            try:
                os.unlink(temporary_name)
            except FileNotFoundError:
                pass


def update_peak_state(path: Path, measurement: object) -> dict[str, int]:
    current = _validate_measurement(measurement, "sample")
    validate_output_file(path, "peak-state file")
    if path.exists():
        previous = read_measurement(path, "peak-state file")
        current = {key: max(previous[key], current[key]) for key in MEASUREMENT_KEYS}
    _atomic_write_json(path, current, "peak-state file")
    return current


def monitor_paths(
    retained_state: Path,
    colima_profile: Path,
    peak_state: Path,
    interval_seconds: float,
    *,
    sleeper: Callable[[float], None] = time.sleep,
) -> None:
    if (
        isinstance(interval_seconds, bool)
        or not isinstance(interval_seconds, (int, float))
        or not math.isfinite(interval_seconds)
        or interval_seconds <= 0
    ):
        raise MetricError("monitor interval must be a positive finite number")
    validate_directory(retained_state, "retained-state path")
    validate_directory(colima_profile, "Colima profile path")
    validate_output_file(peak_state, "peak-state file")
    while True:
        update_peak_state(peak_state, sample_paths(retained_state, colima_profile))
        sleeper(float(interval_seconds))


def finalize_report(
    baseline_path: Path,
    peak_state_path: Path,
    report_path: Path,
    *,
    commit: str,
    build_number: int,
    result: str,
    controller_build_bytes: int | None = None,
    console_bytes: int | None = None,
) -> dict[str, object]:
    validate_commit(commit)
    build_number = _nonnegative_integer(build_number, "build number")
    if build_number == 0:
        raise MetricError("build number must be positive")
    if result not in RESULTS:
        raise MetricError("result is not allowlisted")

    baseline = read_measurement(baseline_path, "baseline file")
    sampled_peak = read_measurement(peak_state_path, "peak-state file")
    peak = {
        key: max(baseline[key], sampled_peak[key]) for key in MEASUREMENT_KEYS
    }
    report: dict[str, object] = {
        "schema_version": 1,
        "commit": commit,
        "build_number": build_number,
        "result": result,
    }
    for key in MEASUREMENT_KEYS:
        stem = key.removesuffix("_allocated_bytes")
        report[f"{stem}_baseline_allocated_bytes"] = baseline[key]
        report[f"{stem}_peak_allocated_bytes"] = peak[key]
        report[f"{stem}_growth_allocated_bytes"] = peak[key] - baseline[key]

    optional_values = {
        "controller_build_bytes": controller_build_bytes,
        "console_bytes": console_bytes,
    }
    for key, value in optional_values.items():
        if value is not None:
            report[key] = _nonnegative_integer(value, key.replace("_", " "))

    _atomic_write_json(report_path, report, "storage report")
    return report


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Collect privacy-safe aggregate storage metrics on a trusted host."
    )
    commands = parser.add_subparsers(dest="command", required=True)

    sample = commands.add_parser("sample", help="print one aggregate sample")
    sample.add_argument("--retained-state", type=Path, required=True)
    sample.add_argument("--colima-profile", type=Path, required=True)

    monitor = commands.add_parser("monitor", help="atomically retain peak samples")
    monitor.add_argument("--retained-state", type=Path, required=True)
    monitor.add_argument("--colima-profile", type=Path, required=True)
    monitor.add_argument("--peak-state", type=Path, required=True)
    monitor.add_argument("--interval-seconds", type=parse_interval, default=5.0)

    record_peak = commands.add_parser(
        "record-peak", help="merge one current sample into the peak state"
    )
    record_peak.add_argument("--retained-state", type=Path, required=True)
    record_peak.add_argument("--colima-profile", type=Path, required=True)
    record_peak.add_argument("--peak-state", type=Path, required=True)

    finalize = commands.add_parser("finalize", help="write a sanitized report")
    finalize.add_argument("--baseline", type=Path, required=True)
    finalize.add_argument("--peak-state", type=Path, required=True)
    finalize.add_argument("--report", type=Path, required=True)
    finalize.add_argument("--commit", required=True)
    finalize.add_argument("--build-number", type=parse_positive_integer, required=True)
    finalize.add_argument("--result", required=True)
    finalize.add_argument("--controller-build-bytes", type=parse_nonnegative_integer)
    finalize.add_argument("--console-bytes", type=parse_nonnegative_integer)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "sample":
            print(
                json.dumps(
                    sample_paths(args.retained_state, args.colima_profile),
                    sort_keys=True,
                    separators=(",", ":"),
                )
            )
        elif args.command == "monitor":
            try:
                monitor_paths(
                    args.retained_state,
                    args.colima_profile,
                    args.peak_state,
                    args.interval_seconds,
                )
            except KeyboardInterrupt:
                pass
        elif args.command == "record-peak":
            update_peak_state(
                args.peak_state,
                sample_paths(args.retained_state, args.colima_profile),
            )
        else:
            finalize_report(
                args.baseline,
                args.peak_state,
                args.report,
                commit=args.commit,
                build_number=args.build_number,
                result=args.result,
                controller_build_bytes=args.controller_build_bytes,
                console_bytes=args.console_bytes,
            )
    except MetricError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
