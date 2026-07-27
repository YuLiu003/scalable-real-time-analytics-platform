#!/usr/bin/env python3

import io
import json
import os
import runpy
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import presubmit


COMPLETE_CHECKLIST = "\n".join(
    f"- [x] `{code}` reviewed" for code in presubmit.CHECKLIST_CODES
)


class PresubmitTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary_directory.name)
        self.original_root = presubmit.REPO_ROOT
        presubmit.REPO_ROOT = self.root

    def tearDown(self) -> None:
        presubmit.REPO_ROOT = self.original_root
        self.temporary_directory.cleanup()

    def write(self, relative: str, content: str | bytes) -> Path:
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        if isinstance(content, bytes):
            path.write_bytes(content)
        else:
            path.write_text(content, encoding="utf-8")
        return path

    def test_run_and_git_output(self) -> None:
        completed = subprocess.CompletedProcess([], 0, stdout="a\0b\0")
        with mock.patch.object(presubmit.subprocess, "run", return_value=completed) as process:
            self.assertEqual(presubmit.git_output(("status", "-z"), {"A": "B"}), ["a", "b"])
            presubmit.run(("true",), {"A": "B"})
        self.assertEqual(process.call_count, 2)
        self.assertEqual(process.call_args.kwargs["cwd"], self.root)

    def test_base_revision_and_changed_files(self) -> None:
        self.assertEqual(presubmit.base_revision({"PRESUBMIT_BASE_SHA": "abc"}), "abc")
        self.assertEqual(presubmit.base_revision({"PRESUBMIT_BASE_REF": "origin/main"}), "origin/main")
        self.assertEqual(presubmit.base_revision({"CHANGE_TARGET": "main"}), "origin/main")
        self.assertIsNone(presubmit.base_revision({}))

        with mock.patch.object(presubmit, "git_output", return_value=["b", "a", "a"]) as output:
            self.assertEqual(
                presubmit.changed_files({"PRESUBMIT_BASE_SHA": "abc"}),
                ["a", "b"],
            )
        output.assert_called_once_with(("diff", "--name-only", "-z", "abc...HEAD"), {"PRESUBMIT_BASE_SHA": "abc"})

        with mock.patch.object(
            presubmit,
            "git_output",
            side_effect=[["unstaged"], ["staged"], ["untracked", "staged"]],
        ):
            self.assertEqual(presubmit.changed_files({}), ["staged", "unstaged", "untracked"])

    def test_check_diff_uses_base_or_local_diffs(self) -> None:
        with mock.patch.object(presubmit, "run") as run:
            presubmit.check_diff({"PRESUBMIT_BASE_REF": "origin/main"})
        run.assert_called_once_with(("git", "diff", "--check", "origin/main...HEAD"), {"PRESUBMIT_BASE_REF": "origin/main"})

        with mock.patch.object(presubmit, "run") as run:
            presubmit.check_diff({})
        self.assertEqual(
            [call.args[0] for call in run.call_args_list],
            [("git", "diff", "--check"), ("git", "diff", "--cached", "--check")],
        )

    def test_workflow_pin_policy(self) -> None:
        self.write(
            ".github/workflows/good.yml",
            "steps:\n"
            "  - uses: ./local-action\n"
            "  - uses: owner/action@0123456789abcdef0123456789abcdef01234567 # v1\n"
            f"  - uses: docker://image@sha256:{'a' * 64}\n",
        )
        presubmit.check_workflow_pins([".github/workflows/good.yml", "missing.yml"])

        self.write(
            ".github/workflows/bad.yaml",
            "steps:\n"
            "  - uses: owner/action@v4\n"
            "  - uses: docker://image:latest\n"
            "  - uses: docker://image@sha256:abc\n"
            "  - uses: no-revision\n",
        )
        with self.assertRaisesRegex(presubmit.PresubmitError, "full commit SHA"):
            presubmit.check_workflow_pins([".github/workflows/bad.yaml"])

    def test_sensitive_path_policy(self) -> None:
        for name in (".env", "kubeconfig", "id_rsa", "id_ed25519", "tls.pem", "token.p12", "app.pfx", "app.key", "secrets.yaml"):
            with self.subTest(name=name):
                self.assertTrue(presubmit.sensitive_path(Path(name)))
        for name in ("config.env", "tls.example.pem", "secret.template.yaml", "fixture.key", "app.go"):
            with self.subTest(name=name):
                self.assertFalse(presubmit.sensitive_path(Path(name)))

    def test_file_safety_checks_names_content_size_and_binary(self) -> None:
        self.write("safe.txt", "ordinary")
        self.write("large.txt", b"x" * (presubmit.MAX_SCANNED_BYTES + 1))
        self.write("binary.dat", b"\xff")
        presubmit.check_file_safety(["safe.txt", "large.txt", "binary.dat", "deleted.txt"])

        self.write(".env", "SAFE=value")
        with self.assertRaisesRegex(presubmit.PresubmitError, "sensitive credential filename"):
            presubmit.check_file_safety([".env"])

        for index, marker in enumerate(
            (
                "-----BEGIN " + "PRIVATE KEY-----",
                "AKIA" + "1234567890123456",
                "ghp" + "_" + ("a" * 36),
            )
        ):
            path = f"credential-{index}.txt"
            self.write(path, marker)
            with self.subTest(marker=marker), self.assertRaisesRegex(
                presubmit.PresubmitError,
                "credential material",
            ):
                presubmit.check_file_safety([path])

    def test_syntax_checks_supported_files(self) -> None:
        self.write("valid.json", '{"ok": true}')
        self.write("valid.py", "value = 1\n")
        self.write("valid.sh", "#!/usr/bin/env bash\ntrue\n")
        self.write("ignored.txt", "text")
        with mock.patch.object(presubmit, "run") as run:
            presubmit.check_syntax(
                ["valid.json", "valid.py", "valid.sh", "ignored.txt", "deleted.py"],
                {},
            )
        run.assert_called_once_with(("bash", "-n", "valid.sh"), {})

        invalid_cases = (
            ("bad.json", "{", "bad.json"),
            ("bad.py", "if", "bad.py"),
        )
        for relative, content, expected in invalid_cases:
            self.write(relative, content)
            with self.subTest(relative=relative), self.assertRaisesRegex(
                presubmit.PresubmitError,
                expected,
            ):
                presubmit.check_syntax([relative], {})

        self.write("bad.sh", "true")
        with mock.patch.object(
            presubmit,
            "run",
            side_effect=subprocess.CalledProcessError(1, ["bash"]),
        ), self.assertRaisesRegex(presubmit.PresubmitError, "bad.sh"):
            presubmit.check_syntax(["bad.sh"], {})

    def test_pull_request_body_sources(self) -> None:
        self.assertEqual(
            presubmit.pull_request_body({"PRESUBMIT_PR_BODY": "body"}),
            "body",
        )
        self.assertIsNone(presubmit.pull_request_body({}))

        event_path = self.write("event.json", json.dumps({"pull_request": {"body": "event body"}}))
        self.assertEqual(
            presubmit.pull_request_body({"GITHUB_EVENT_PATH": str(event_path)}),
            "event body",
        )
        event_path.write_text(json.dumps({"pull_request": {"body": None}}), encoding="utf-8")
        self.assertEqual(presubmit.pull_request_body({"GITHUB_EVENT_PATH": str(event_path)}), "")
        event_path.write_text(json.dumps({"push": {}}), encoding="utf-8")
        self.assertIsNone(presubmit.pull_request_body({"GITHUB_EVENT_PATH": str(event_path)}))

    def test_jenkins_pipeline_policy(self) -> None:
        presubmit.check_jenkins_pipeline(["README.md"])
        self.write(
            "Jenkinsfile",
            "agent none\nlabel 'jenkins-verify'\nlabel 'jenkins-integration'\n"
            "stage('PS0') {}\nstage('PS1') {}\nstage('PS2') {}\n",
        )
        presubmit.check_jenkins_pipeline(["Jenkinsfile"])

        self.write("Jenkinsfile", "agent any\nstage('PS0') {}\nwithCredentials([]) {}\n")
        with self.assertRaisesRegex(presubmit.PresubmitError, "missing PS1"):
            presubmit.check_jenkins_pipeline(["Jenkinsfile"])

        self.write(
            "Jenkinsfile",
            "agent none\nlabel 'jenkins-verify'\nlabel 'jenkins-integration'\n"
            "stage('PS0') {}\nstage('PS1') {}\nstage('PS2') {}\npodTemplate([]) {}\n",
        )
        with self.assertRaisesRegex(presubmit.PresubmitError, "agent privilege"):
            presubmit.check_jenkins_pipeline(["Jenkinsfile"])

    def test_pr_checklist_policy(self) -> None:
        presubmit.check_pr_checklist({})
        presubmit.check_pr_checklist({"PRESUBMIT_PR_BODY": COMPLETE_CHECKLIST})
        with self.assertRaisesRegex(presubmit.PresubmitError, "SEC"):
            presubmit.check_pr_checklist({"PRESUBMIT_PR_BODY": "- [x] `SCOPE` reviewed"})
        with self.assertRaisesRegex(presubmit.PresubmitError, "no pull-request body"):
            presubmit.check_pr_checklist({"PRESUBMIT_REQUIRE_PR_CHECKLIST": "true"})

    def test_ps0_runs_every_check(self) -> None:
        self.write(".github/workflows/a.yml", "name: A\n")
        self.write(".github/workflows/z.yaml", "name: Z\n")
        with (
            mock.patch.object(presubmit, "changed_files", return_value=["a.py"]),
            mock.patch.object(presubmit, "check_diff") as diff,
            mock.patch.object(presubmit, "check_workflow_pins") as pins,
            mock.patch.object(presubmit, "check_file_safety") as safety,
            mock.patch.object(presubmit, "check_syntax") as syntax,
            mock.patch.object(presubmit, "check_jenkins_pipeline") as jenkins,
            mock.patch.object(presubmit, "check_pr_checklist") as checklist,
            mock.patch("sys.stdout", new_callable=io.StringIO) as output,
        ):
            presubmit.run_ps0({})
        diff.assert_called_once_with({})
        pins.assert_called_once_with(
            [".github/workflows/a.yml", ".github/workflows/z.yaml"],
        )
        safety.assert_called_once_with(["a.py"])
        syntax.assert_called_once_with(["a.py"], {})
        jenkins.assert_called_once_with(["a.py"])
        checklist.assert_called_once_with({})
        self.assertIn("1 changed file", output.getvalue())

    def test_run_stage_routes_all_stages(self) -> None:
        with mock.patch.object(presubmit, "run_ps0") as ps0, mock.patch(
            "sys.stdout",
            new_callable=io.StringIO,
        ):
            presubmit.run_stage("PS0", {})
        ps0.assert_called_once_with({})

        with mock.patch.object(presubmit, "run") as run, mock.patch(
            "sys.stdout",
            new_callable=io.StringIO,
        ):
            presubmit.run_stage("PS1", {"CI": "true"})
            presubmit.run_stage("PS2", {"CI": "true"})
        expected = [
            *presubmit.STAGE_COMMANDS["PS1"],
            *presubmit.STAGE_COMMANDS["PS2"],
        ]
        self.assertEqual([call.args[0] for call in run.call_args_list], expected)

    def test_argument_parsing_and_main(self) -> None:
        self.assertEqual(presubmit.parse_args(("PS0",)).stage, "PS0")
        with mock.patch("sys.stderr", new_callable=io.StringIO):
            with self.assertRaises(SystemExit):
                presubmit.parse_args(())
            with self.assertRaises(SystemExit):
                presubmit.parse_args(("invalid",))

        with mock.patch("sys.stdout", new_callable=io.StringIO) as output:
            self.assertEqual(presubmit.main(("--list",), {}), 0)
        self.assertEqual(output.getvalue().splitlines(), list(presubmit.STAGES))

        with mock.patch.object(presubmit, "run_stage") as run:
            self.assertEqual(presubmit.main(("all",), {"CI": "true"}), 0)
        self.assertEqual([call.args[0] for call in run.call_args_list], list(presubmit.STAGES))

        with mock.patch.object(
            presubmit,
            "run_stage",
            side_effect=presubmit.PresubmitError("blocked"),
        ), mock.patch("sys.stderr", new_callable=io.StringIO) as error:
            self.assertEqual(presubmit.main(("PS0",), {}), 1)
        self.assertIn("blocked", error.getvalue())

        with mock.patch.object(
            presubmit,
            "run_stage",
            side_effect=subprocess.CalledProcessError(1, ["false"]),
        ), mock.patch("sys.stderr", new_callable=io.StringIO):
            self.assertEqual(presubmit.main(("PS1",), {}), 1)

    def test_entrypoint_lists_stages(self) -> None:
        output = io.StringIO()
        with (
            mock.patch.object(sys, "argv", [presubmit.__file__, "--list"]),
            mock.patch.object(sys, "stdout", output),
        ):
            with self.assertRaises(SystemExit) as exit_context:
                runpy.run_path(presubmit.__file__, run_name="__main__")
        self.assertEqual(exit_context.exception.code, 0)
        self.assertEqual(output.getvalue().splitlines(), list(presubmit.STAGES))


if __name__ == "__main__":
    unittest.main()
