#!/usr/bin/env python3

import argparse
import importlib.util
import io
import json
import os
import subprocess
import tempfile
import unittest
from contextlib import redirect_stderr
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "storage-metrics.py"
SPEC = importlib.util.spec_from_file_location("storage_metrics", SCRIPT)
assert SPEC and SPEC.loader
metrics = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(metrics)


class StorageMetricsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.retained = self.root / "retained"
        self.colima = self.root / "colima"
        self.retained.mkdir()
        self.colima.mkdir()
        self.baseline = self.root / "baseline.json"
        self.peak = self.root / "peak.json"
        self.report = self.root / "report.json"
        self.commit = "a" * 40

    @staticmethod
    def measurement(retained: int = 1024, colima: int = 2048) -> dict[str, int]:
        return {
            "retained_state_allocated_bytes": retained,
            "colima_profile_allocated_bytes": colima,
        }

    def write_measurement(
        self, path: Path, retained: int = 1024, colima: int = 2048
    ) -> None:
        path.write_text(
            json.dumps(self.measurement(retained, colima)),
            encoding="utf-8",
        )

    def test_directory_and_file_path_validation(self) -> None:
        self.assertEqual(
            metrics.validate_directory(self.retained, "directory"), self.retained
        )
        regular_file = self.root / "file"
        regular_file.write_text("data", encoding="utf-8")
        self.assertEqual(
            metrics.validate_input_file(regular_file, "input"), regular_file
        )
        self.assertEqual(
            metrics.validate_output_file(self.report, "output"), self.report
        )
        self.assertEqual(
            metrics.validate_output_file(regular_file, "output"), regular_file
        )

        for invalid in (Path("relative"), self.root / "missing", regular_file):
            with self.subTest(directory=invalid):
                with self.assertRaisesRegex(metrics.MetricError, "absolute directory"):
                    metrics.validate_directory(invalid, "directory")

        for invalid in (Path("relative"), self.root / "missing", self.retained):
            with self.subTest(input_file=invalid):
                with self.assertRaisesRegex(metrics.MetricError, "regular file"):
                    metrics.validate_input_file(invalid, "input")

        directory_link = self.root / "directory-link"
        directory_link.symlink_to(self.retained, target_is_directory=True)
        file_link = self.root / "file-link"
        file_link.symlink_to(regular_file)
        (self.retained / "nested").mkdir()
        nested = directory_link / "nested"
        with self.assertRaisesRegex(metrics.MetricError, "without symlinks"):
            metrics.validate_directory(directory_link, "directory")
        self.assertEqual(metrics.validate_directory(nested, "directory"), nested)
        with self.assertRaisesRegex(metrics.MetricError, "without symlinks"):
            metrics.validate_input_file(file_link, "input")
        nested_file = nested / "file"
        nested_file.write_text("safe aggregate", encoding="utf-8")
        self.assertEqual(metrics.validate_input_file(nested_file, "input"), nested_file)
        self.assertEqual(
            metrics.validate_output_file(nested / "output", "output"),
            nested / "output",
        )
        with self.assertRaisesRegex(metrics.MetricError, "regular file"):
            metrics.validate_output_file(Path("relative"), "output")
        with self.assertRaisesRegex(metrics.MetricError, "regular file"):
            metrics.validate_output_file(file_link, "output")
        with self.assertRaisesRegex(metrics.MetricError, "parent"):
            metrics.validate_output_file(self.root / "missing" / "output", "output")

    def test_numeric_and_identity_validation(self) -> None:
        self.assertEqual(metrics._nonnegative_integer(0, "value"), 0)
        self.assertEqual(
            metrics._nonnegative_integer(metrics.MAX_INTEGER, "value"),
            metrics.MAX_INTEGER,
        )
        for value in (True, "1"):
            with self.subTest(non_integer=value):
                with self.assertRaisesRegex(
                    metrics.MetricError, "non-negative integer"
                ):
                    metrics._nonnegative_integer(value, "value")
        for value in (-1, metrics.MAX_INTEGER + 1):
            with self.subTest(out_of_range=value):
                with self.assertRaisesRegex(metrics.MetricError, "64-bit"):
                    metrics._nonnegative_integer(value, "value")

        self.assertEqual(metrics.parse_nonnegative_integer("0"), 0)
        self.assertEqual(metrics.parse_nonnegative_integer("19"), 19)
        for text in ("-1", "01", "+1", "one"):
            with self.subTest(nonnegative=text):
                with self.assertRaises(argparse.ArgumentTypeError):
                    metrics.parse_nonnegative_integer(text)
        with self.assertRaisesRegex(argparse.ArgumentTypeError, "64-bit"):
            metrics.parse_nonnegative_integer(str(metrics.MAX_INTEGER + 1))
        self.assertEqual(metrics.parse_positive_integer("7"), 7)
        with self.assertRaisesRegex(argparse.ArgumentTypeError, "positive"):
            metrics.parse_positive_integer("0")

        self.assertEqual(metrics.parse_interval("0.25"), 0.25)
        with self.assertRaisesRegex(argparse.ArgumentTypeError, "positive number"):
            metrics.parse_interval("later")
        for text in ("0", "-1", "nan", "inf"):
            with self.subTest(interval=text):
                with self.assertRaisesRegex(argparse.ArgumentTypeError, "finite"):
                    metrics.parse_interval(text)

        self.assertEqual(metrics.validate_commit(self.commit), self.commit)
        for commit in ("a" * 39, "A" * 40, "g" * 40):
            with self.subTest(commit=commit):
                with self.assertRaisesRegex(metrics.MetricError, "lowercase"):
                    metrics.validate_commit(commit)

    @mock.patch.object(metrics.subprocess, "run")
    def test_allocated_byte_sampling(self, run: mock.Mock) -> None:
        run.side_effect = [
            subprocess.CompletedProcess([], 0, "12\tignored-path\n", "secret"),
            subprocess.CompletedProcess([], 0, "3 retained\n", ""),
            subprocess.CompletedProcess([], 0, "9 colima\n", ""),
        ]
        self.assertEqual(metrics.allocated_bytes(self.retained), 12 * 1024)
        self.assertEqual(
            metrics.sample_paths(self.retained, self.colima),
            self.measurement(3 * 1024, 9 * 1024),
        )
        self.assertEqual(run.call_args_list[0].args[0][:3], ("du", "-sk", "--"))
        self.assertEqual(run.call_args_list[0].kwargs["stderr"], subprocess.DEVNULL)

    @mock.patch.object(metrics.subprocess, "run")
    def test_sampling_rejects_du_failures_and_invalid_results(
        self, run: mock.Mock
    ) -> None:
        run.side_effect = OSError("secret executable detail")
        with self.assertRaisesRegex(
            metrics.MetricError, "could not be executed"
        ) as caught:
            metrics.allocated_bytes(self.retained)
        self.assertNotIn("secret", str(caught.exception))

        for completed, message in (
            (subprocess.CompletedProcess([], 1, "", "secret path"), "du failed"),
            (subprocess.CompletedProcess([], 0, "", ""), "numeric"),
            (subprocess.CompletedProcess([], 0, "nope path", ""), "numeric"),
            (
                subprocess.CompletedProcess(
                    [], 0, f"{metrics.MAX_INTEGER // 1024 + 1} path", ""
                ),
                "integer range",
            ),
        ):
            with self.subTest(message=message):
                run.side_effect = None
                run.return_value = completed
                with self.assertRaisesRegex(metrics.MetricError, message) as caught:
                    metrics.allocated_bytes(self.retained)
                self.assertNotIn("path", str(caught.exception))

    def test_measurement_schema_and_json_loading(self) -> None:
        valid = self.measurement()
        self.assertEqual(metrics._validate_measurement(valid, "sample"), valid)
        for invalid in ([], {}, {**valid, "filename": "private"}):
            with self.subTest(schema=invalid):
                with self.assertRaisesRegex(metrics.MetricError, "aggregate schema"):
                    metrics._validate_measurement(invalid, "sample")
        for value in (True, -1):
            with self.subTest(value=value):
                invalid = {**valid, metrics.MEASUREMENT_KEYS[0]: value}
                with self.assertRaises(metrics.MetricError):
                    metrics._validate_measurement(invalid, "sample")

        self.assertEqual(
            metrics._reject_duplicate_keys([("first", 1), ("second", 2)]),
            {"first": 1, "second": 2},
        )
        with self.assertRaisesRegex(ValueError, "duplicate"):
            metrics._reject_duplicate_keys([("same", 1), ("same", 2)])

        self.write_measurement(self.baseline)
        self.assertEqual(
            metrics.read_measurement(self.baseline, "baseline file"), valid
        )
        for payload in (
            "not-json",
            '{"retained_state_allocated_bytes":1,'
            '"retained_state_allocated_bytes":2,'
            '"colima_profile_allocated_bytes":3}',
            '{"retained_state_allocated_bytes":NaN,'
            '"colima_profile_allocated_bytes":3}',
        ):
            with self.subTest(payload=payload):
                self.baseline.write_text(payload, encoding="utf-8")
                with self.assertRaisesRegex(
                    metrics.MetricError, "valid aggregate JSON"
                ):
                    metrics.read_measurement(self.baseline, "baseline file")

        self.write_measurement(self.baseline)
        for failure in (OSError("secret filename"), UnicodeError("secret content")):
            with self.subTest(read_failure=type(failure).__name__):
                with mock.patch.object(Path, "read_text", side_effect=failure):
                    with self.assertRaisesRegex(
                        metrics.MetricError, "valid aggregate JSON"
                    ) as caught:
                        metrics.read_measurement(self.baseline, "baseline file")
                    self.assertNotIn("secret", str(caught.exception))

    def test_atomic_write_success_and_cleanup_failures(self) -> None:
        metrics._atomic_write_json(self.report, {"safe": 1}, "storage report")
        self.assertEqual(
            json.loads(self.report.read_text(encoding="utf-8")), {"safe": 1}
        )
        metrics._atomic_write_json(self.report, {"safe": 2}, "storage report")
        self.assertEqual(
            json.loads(self.report.read_text(encoding="utf-8")), {"safe": 2}
        )

        with mock.patch.object(
            metrics.tempfile, "mkstemp", side_effect=OSError("denied")
        ):
            with self.assertRaisesRegex(metrics.MetricError, "atomically write"):
                metrics._atomic_write_json(self.root / "new.json", {}, "storage report")

        before = set(self.root.iterdir())
        with self.assertRaisesRegex(metrics.MetricError, "atomically write"):
            metrics._atomic_write_json(
                self.root / "bad.json", {"unsafe": object()}, "storage report"
            )
        self.assertEqual(set(self.root.iterdir()), before)

        with mock.patch.object(
            metrics.os, "replace", side_effect=OSError("denied")
        ), mock.patch.object(metrics.os, "unlink", side_effect=FileNotFoundError):
            with self.assertRaisesRegex(metrics.MetricError, "atomically write"):
                metrics._atomic_write_json(
                    self.root / "replace.json", {}, "storage report"
                )

    def test_peak_state_initialization_and_maxima(self) -> None:
        first = self.measurement(100, 400)
        self.assertEqual(metrics.update_peak_state(self.peak, first), first)
        self.assertEqual(metrics.read_measurement(self.peak, "peak"), first)
        second = self.measurement(300, 200)
        expected = self.measurement(300, 400)
        self.assertEqual(metrics.update_peak_state(self.peak, second), expected)
        self.assertEqual(metrics.read_measurement(self.peak, "peak"), expected)
        with self.assertRaisesRegex(metrics.MetricError, "aggregate schema"):
            metrics.update_peak_state(self.peak, {"filename": "private"})

    def test_monitor_samples_until_interrupted(self) -> None:
        samples = [self.measurement(100, 400), self.measurement(300, 200)]
        sleeps: list[float] = []

        def sleeper(interval: float) -> None:
            sleeps.append(interval)
            if len(sleeps) == 2:
                raise KeyboardInterrupt

        with mock.patch.object(metrics, "sample_paths", side_effect=samples):
            with self.assertRaises(KeyboardInterrupt):
                metrics.monitor_paths(
                    self.retained,
                    self.colima,
                    self.peak,
                    0.5,
                    sleeper=sleeper,
                )
        self.assertEqual(sleeps, [0.5, 0.5])
        self.assertEqual(
            metrics.read_measurement(self.peak, "peak"), self.measurement(300, 400)
        )

        for interval in (True, "1", float("inf"), 0, -1):
            with self.subTest(interval=interval):
                with self.assertRaisesRegex(metrics.MetricError, "positive finite"):
                    metrics.monitor_paths(
                        self.retained,
                        self.colima,
                        self.peak,
                        interval,
                        sleeper=sleeper,
                    )

    def test_finalize_writes_only_allowlisted_aggregates(self) -> None:
        self.write_measurement(self.baseline, 100, 500)
        self.write_measurement(self.peak, 300, 400)
        report = metrics.finalize_report(
            self.baseline,
            self.peak,
            self.report,
            commit=self.commit,
            build_number=17,
            result="SUCCESS",
            controller_build_bytes=10,
            console_bytes=20,
        )
        expected = {
            "schema_version": 1,
            "commit": self.commit,
            "build_number": 17,
            "result": "SUCCESS",
            "retained_state_baseline_allocated_bytes": 100,
            "retained_state_peak_allocated_bytes": 300,
            "retained_state_growth_allocated_bytes": 200,
            "colima_profile_baseline_allocated_bytes": 500,
            "colima_profile_peak_allocated_bytes": 500,
            "colima_profile_growth_allocated_bytes": 0,
            "controller_build_bytes": 10,
            "console_bytes": 20,
        }
        self.assertEqual(report, expected)
        self.assertEqual(
            json.loads(self.report.read_text(encoding="utf-8")), expected
        )
        serialized = self.report.read_text(encoding="utf-8")
        self.assertNotIn(str(self.root), serialized)
        self.assertNotIn("filename", serialized)

        minimal_report = self.root / "minimal.json"
        minimal = metrics.finalize_report(
            self.baseline,
            self.peak,
            minimal_report,
            commit=self.commit,
            build_number=1,
            result="UNKNOWN",
        )
        self.assertNotIn("console_bytes", minimal)

    def test_finalize_rejects_invalid_provenance_and_values(self) -> None:
        self.write_measurement(self.baseline)
        self.write_measurement(self.peak)
        arguments = {
            "commit": self.commit,
            "build_number": 1,
            "result": "SUCCESS",
        }
        for replacement, message in (
            ({"commit": "bad"}, "commit"),
            ({"build_number": 0}, "positive"),
            ({"build_number": True}, "integer"),
            ({"result": "UNSTABLE"}, "allowlisted"),
            ({"console_bytes": -1}, "64-bit"),
        ):
            with self.subTest(replacement=replacement):
                with self.assertRaisesRegex(metrics.MetricError, message):
                    metrics.finalize_report(
                        self.baseline,
                        self.peak,
                        self.report,
                        **{**arguments, **replacement},
                    )

    def test_parser_contract(self) -> None:
        parser = metrics.build_parser()
        sample = parser.parse_args(
            (
                "sample",
                "--retained-state",
                str(self.retained),
                "--colima-profile",
                str(self.colima),
            )
        )
        self.assertEqual(sample.command, "sample")
        monitor = parser.parse_args(
            (
                "monitor",
                "--retained-state",
                str(self.retained),
                "--colima-profile",
                str(self.colima),
                "--peak-state",
                str(self.peak),
            )
        )
        self.assertEqual(monitor.interval_seconds, 5.0)
        record_peak = parser.parse_args(
            (
                "record-peak",
                "--retained-state",
                str(self.retained),
                "--colima-profile",
                str(self.colima),
                "--peak-state",
                str(self.peak),
            )
        )
        self.assertEqual(record_peak.command, "record-peak")
        finalize = parser.parse_args(
            (
                "finalize",
                "--baseline",
                str(self.baseline),
                "--peak-state",
                str(self.peak),
                "--report",
                str(self.report),
                "--commit",
                self.commit,
                "--build-number",
                "2",
                "--result",
                "ABORTED",
                "--console-bytes",
                "0",
            )
        )
        self.assertEqual(finalize.console_bytes, 0)

        for arguments in ((), ("monitor", "--interval-seconds", "nan")):
            with self.subTest(arguments=arguments), redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit):
                    parser.parse_args(arguments)

    def test_main_dispatch_and_sanitized_error(self) -> None:
        sample_args = (
            "sample",
            "--retained-state",
            str(self.retained),
            "--colima-profile",
            str(self.colima),
        )
        with mock.patch.object(
            metrics, "sample_paths", return_value=self.measurement()
        ), mock.patch("builtins.print") as output:
            self.assertEqual(metrics.main(sample_args), 0)
            self.assertEqual(json.loads(output.call_args.args[0]), self.measurement())

        monitor_args = (
            "monitor",
            "--retained-state",
            str(self.retained),
            "--colima-profile",
            str(self.colima),
            "--peak-state",
            str(self.peak),
        )
        with mock.patch.object(metrics, "monitor_paths") as monitor:
            self.assertEqual(metrics.main(monitor_args), 0)
            monitor.assert_called_once()
        with mock.patch.object(metrics, "monitor_paths", side_effect=KeyboardInterrupt):
            self.assertEqual(metrics.main(monitor_args), 0)

        record_peak_args = (
            "record-peak",
            "--retained-state",
            str(self.retained),
            "--colima-profile",
            str(self.colima),
            "--peak-state",
            str(self.peak),
        )
        with mock.patch.object(
            metrics, "sample_paths", return_value=self.measurement()
        ), mock.patch.object(metrics, "update_peak_state") as update:
            self.assertEqual(metrics.main(record_peak_args), 0)
            update.assert_called_once_with(self.peak, self.measurement())

        finalize_args = (
            "finalize",
            "--baseline",
            str(self.baseline),
            "--peak-state",
            str(self.peak),
            "--report",
            str(self.report),
            "--commit",
            self.commit,
            "--build-number",
            "3",
            "--result",
            "TIMEOUT",
            "--controller-build-bytes",
            "10",
            "--console-bytes",
            "20",
        )
        with mock.patch.object(metrics, "finalize_report") as finalize:
            self.assertEqual(metrics.main(finalize_args), 0)
            self.assertEqual(finalize.call_args.kwargs["console_bytes"], 20)

        with mock.patch.object(
            metrics, "sample_paths", side_effect=metrics.MetricError("safe failure")
        ), mock.patch("builtins.print") as output:
            self.assertEqual(metrics.main(sample_args), 1)
            self.assertEqual(output.call_args.kwargs["file"], metrics.sys.stderr)
            self.assertIn("safe failure", output.call_args.args[0])


if __name__ == "__main__":
    unittest.main()
