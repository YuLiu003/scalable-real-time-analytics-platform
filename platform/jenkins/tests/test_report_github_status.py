#!/usr/bin/env python3

import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "report-github-status.py"
SPEC = importlib.util.spec_from_file_location("report_github_status", SCRIPT)
assert SPEC and SPEC.loader
reporter = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(reporter)


class GitHubStatusReporterTests(unittest.TestCase):
    def setUp(self) -> None:
        self.commit = "a" * 40
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.log = Path(self.temporary.name) / "jenkins.log"

    def write_evidence(self, prefix: str = "") -> None:
        self.log.write_text(
            prefix
            + f"Jenkins PS0, PS1, and PS2 pipeline passed for {self.commit}.\n"
            + "Disposable Jenkins production-like validation passed; deleting "
            + "investment-platform-jenkins-ephemeral.\n"
            + "Deleted `colima-investment-platform-jenkins-ephemeral`\n"
            + "msg=done\n",
            encoding="utf-8",
        )

    @mock.patch.object(reporter.subprocess, "run")
    def test_command_boundaries(self, run: mock.Mock) -> None:
        run.return_value = subprocess.CompletedProcess([], 0, "ok\n", "")
        self.assertEqual(reporter.command(("tool",)), "ok")
        run.return_value = subprocess.CompletedProcess([], 1, "", "bad\n")
        with self.assertRaisesRegex(reporter.ReportError, "tool: bad"):
            reporter.command(("tool",))
        run.return_value = subprocess.CompletedProcess([], 1, "", "")
        with self.assertRaisesRegex(reporter.ReportError, "command failed"):
            reporter.command(("tool",))
        run.return_value = subprocess.CompletedProcess([], 0, "not-json", "")
        with self.assertRaisesRegex(reporter.ReportError, "invalid JSON"):
            reporter.command_json(("tool",))
        run.side_effect = FileNotFoundError("missing")
        with self.assertRaisesRegex(reporter.ReportError, "tool: missing"):
            reporter.command(("tool",))

    def test_evidence_validation(self) -> None:
        self.write_evidence()
        digest = reporter.validate_evidence(self.log.resolve(), self.commit)
        self.assertEqual(len(digest), 64)

        relative = Path("jenkins.log")
        with self.assertRaisesRegex(reporter.ReportError, "absolute"):
            reporter.validate_evidence(relative, self.commit)

        self.write_evidence("ERROR: Jenkins failed\n")
        with self.assertRaisesRegex(reporter.ReportError, "failure before cleanup"):
            reporter.validate_evidence(self.log.resolve(), self.commit)

        self.log.write_text("incomplete", encoding="utf-8")
        with self.assertRaisesRegex(reporter.ReportError, "missing ordered marker"):
            reporter.validate_evidence(self.log.resolve(), self.commit)
        self.write_evidence()
        with mock.patch.object(Path, "read_text", side_effect=OSError("denied")):
            with self.assertRaisesRegex(reporter.ReportError, "cannot read"):
                reporter.validate_evidence(self.log.resolve(), self.commit)

    @mock.patch.object(reporter, "command_json")
    def test_pull_request_validation(self, command_json: mock.Mock) -> None:
        valid = {
            "state": "OPEN",
            "baseRefName": "main",
            "headRefOid": self.commit,
        }
        command_json.return_value = valid
        self.assertIs(reporter.validate_pull_request(29, self.commit), valid)

        command_json.return_value = []
        with self.assertRaisesRegex(reporter.ReportError, "pull-request object"):
            reporter.validate_pull_request(29, self.commit)
        command_json.return_value = {**valid, "state": "CLOSED"}
        with self.assertRaisesRegex(reporter.ReportError, "open pull request"):
            reporter.validate_pull_request(29, self.commit)
        command_json.return_value = {**valid, "headRefOid": "b" * 40}
        with self.assertRaisesRegex(reporter.ReportError, "does not match"):
            reporter.validate_pull_request(29, self.commit)

    @mock.patch.object(reporter, "command_json")
    @mock.patch.object(reporter, "command")
    def test_comment_create_and_update(
        self, command: mock.Mock, command_json: mock.Mock
    ) -> None:
        command.return_value = "owner"
        command_json.side_effect = [
            [],
            {"html_url": "https://example.test/new"},
            [
                {
                    "id": 7,
                    "body": reporter.MARKER,
                    "user": {"login": "owner"},
                }
            ],
            {"html_url": "https://example.test/existing"},
        ]
        self.assertEqual(
            reporter.publish_comment(29, "body"),
            "https://example.test/new",
        )
        self.assertEqual(
            reporter.publish_comment(29, "body"),
            "https://example.test/existing",
        )
        self.assertIn("PATCH", command_json.call_args_list[-1].args[0])

        command_json.side_effect = [object()]
        with self.assertRaisesRegex(reporter.ReportError, "comment list"):
            reporter.publish_comment(29, "body")
        command_json.side_effect = [[], {}]
        with self.assertRaisesRegex(reporter.ReportError, "comment URL"):
            reporter.publish_comment(29, "body")

    @mock.patch.object(reporter, "command")
    def test_publish_status(self, command: mock.Mock) -> None:
        reporter.publish_status(self.commit, "https://example.test/evidence")
        arguments = command.call_args.args[0]
        self.assertIn(f"context={reporter.CONTEXT}", arguments)
        self.assertIn("state=success", arguments)

    @mock.patch.object(reporter, "publish_status")
    @mock.patch.object(reporter, "publish_comment")
    @mock.patch.object(reporter, "validate_pull_request")
    @mock.patch.object(reporter, "command")
    def test_report(
        self,
        command: mock.Mock,
        validate_pull_request: mock.Mock,
        publish_comment: mock.Mock,
        publish_status: mock.Mock,
    ) -> None:
        self.write_evidence()
        command.return_value = self.commit
        publish_comment.return_value = "https://example.test/evidence"
        result = reporter.report(29, self.log.resolve())
        self.assertIn(self.commit, result)
        validate_pull_request.assert_called_once_with(29, self.commit)
        publish_status.assert_called_once_with(
            self.commit, "https://example.test/evidence"
        )

        command.return_value = "invalid"
        with self.assertRaisesRegex(reporter.ReportError, "full lowercase"):
            reporter.report(29, self.log.resolve())

    @mock.patch.object(reporter, "report")
    def test_main(self, report: mock.Mock) -> None:
        report.return_value = "published"
        with mock.patch("builtins.print") as output:
            self.assertEqual(
                reporter.main(("--pr", "29", "--log", str(self.log))),
                0,
            )
            output.assert_called_once_with("published")
        report.side_effect = reporter.ReportError("no")
        with mock.patch("builtins.print") as output:
            self.assertEqual(
                reporter.main(("--pr", "29", "--log", str(self.log))),
                1,
            )
            self.assertEqual(output.call_args.kwargs["file"], reporter.sys.stderr)

    def test_body(self) -> None:
        body = reporter.evidence_body(self.commit, "d" * 64, "now")
        self.assertIn(reporter.MARKER, body)
        self.assertIn(self.commit, body)
        self.assertIn("production installation", body)
        self.assertIn("bounded history and Garage artifacts", body)
        self.assertIn("not published or an off-host backup", body)
        self.assertNotIn("controller and console are intentionally deleted", body)


if __name__ == "__main__":
    unittest.main()
