from __future__ import annotations

import argparse
import io
import json
import sys
import unittest
from unittest import mock

from agent_review_optimizer.cli import _nonnegative, _positive, main


RUN_ID = "11111111-1111-4111-8111-111111111111"
INVOCATION_ID = "22222222-2222-4222-8222-222222222222"


def codex_event() -> str:
    return json.dumps(
        {
            "type": "turn.completed",
            "usage": {
                "input_tokens": 100,
                "cached_input_tokens": 80,
                "output_tokens": 20,
                "reasoning_output_tokens": 5,
            },
        }
    )


def codex_arguments() -> list[str]:
    return [
        "codex-run",
        "--run-id",
        RUN_ID,
        "--invocation-id",
        INVOCATION_ID,
        "--occurred-at",
        "2026-08-01T17:00:00Z",
        "--strategy",
        "routed",
        "--reviewer",
        "general",
        "--duration-ms",
        "100",
        "--changed-file-count",
        "2",
        "--proposed-findings",
        "1",
        "--confirmed-findings",
        "1",
        "--false-positive-findings",
        "0",
    ]


class CliTests(unittest.TestCase):
    def test_routes_newline_and_null_delimited_paths(self) -> None:
        for arguments, value in (
            (["route"], "main.go\n"),
            (["route", "--null"], "main.go\0worker.py\0"),
        ):
            output = io.StringIO()
            self.assertEqual(main(arguments, io.StringIO(value), output), 0)
            result = json.loads(output.getvalue())
            self.assertNotIn("paths", result)
            self.assertEqual(result["reviewers"][0], "general")

    def test_extracts_codex_run_and_summarizes_it(self) -> None:
        invocation_output = io.StringIO()
        self.assertEqual(
            main(
                codex_arguments(),
                io.StringIO(codex_event()),
                invocation_output,
            ),
            0,
        )
        invocation = json.loads(invocation_output.getvalue())
        self.assertEqual(invocation["provider"], "codex")
        self.assertIsNone(invocation["model"])

        summary_output = io.StringIO()
        self.assertEqual(
            main(
                ["summarize"],
                io.StringIO(invocation_output.getvalue()),
                summary_output,
            ),
            0,
        )
        self.assertEqual(json.loads(summary_output.getvalue())["invocation_count"], 1)

    def test_derives_failed_status_and_preserves_missing_usage(self) -> None:
        arguments = codex_arguments()
        for flag in ("--proposed-findings", "--confirmed-findings"):
            arguments[arguments.index(flag) + 1] = "0"
        output = io.StringIO()
        self.assertEqual(
            main(
                arguments,
                io.StringIO(json.dumps({"type": "turn.failed"})),
                output,
            ),
            0,
        )
        result = json.loads(output.getvalue())
        self.assertEqual(result["status"], "failed")
        self.assertIsNone(result["usage"])

    def test_reports_domain_errors_without_raw_input(self) -> None:
        error = io.StringIO()
        self.assertEqual(main(["route"], io.StringIO(""), io.StringIO(), error), 2)
        self.assertIn("at least one changed path", error.getvalue())

    def test_uses_process_streams_when_not_injected(self) -> None:
        process_input = io.StringIO("README.md\n")
        process_output = io.StringIO()
        process_error = io.StringIO()
        with (
            mock.patch.object(sys, "stdin", process_input),
            mock.patch.object(sys, "stdout", process_output),
            mock.patch.object(sys, "stderr", process_error),
        ):
            self.assertEqual(main(["route"]), 0)
        self.assertEqual(process_error.getvalue(), "")
        self.assertEqual(json.loads(process_output.getvalue())["reviewers"], ["general"])

    def test_integer_argument_boundaries(self) -> None:
        self.assertEqual(_nonnegative("0"), 0)
        self.assertEqual(_positive("1"), 1)
        with self.assertRaises(argparse.ArgumentTypeError):
            _nonnegative("-1")
        with self.assertRaises(argparse.ArgumentTypeError):
            _positive("0")


if __name__ == "__main__":
    unittest.main()
