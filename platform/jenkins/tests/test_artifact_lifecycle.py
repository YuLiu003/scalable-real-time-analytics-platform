#!/usr/bin/env python3

import importlib.util
import io
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch


MODULE_PATH = (
    Path(__file__).resolve().parents[1]
    / "storage"
    / "configure-artifact-lifecycle.py"
)
SPEC = importlib.util.spec_from_file_location("artifact_lifecycle", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class FakeClient:
    def __init__(self, rules=None) -> None:
        self.rules = rules
        self.put = None

    def put_bucket_lifecycle_configuration(self, **kwargs) -> None:
        self.put = kwargs
        self.rules = kwargs["LifecycleConfiguration"]["Rules"]

    def get_bucket_lifecycle_configuration(self, **kwargs):
        self.bucket = kwargs["Bucket"]
        return {"Rules": self.rules}


class ArtifactLifecycleTests(unittest.TestCase):
    def setUp(self) -> None:
        self.environ = {
            "AWS_ACCESS_KEY_ID": "access",
            "AWS_SECRET_ACCESS_KEY": "secret",
            "S3_ENDPOINT": "http://jenkins-artifacts:3900",
            "S3_BUCKET": "jenkins-artifacts",
            "S3_RETENTION_DAYS": "3",
        }

    def test_apply_writes_and_verifies_exact_rule(self) -> None:
        client = FakeClient()
        calls = []

        def factory(*args, **kwargs):
            calls.append((args, kwargs))
            return client

        MODULE.apply(self.environ, factory)
        self.assertEqual(calls[0][0], ("s3",))
        self.assertEqual(calls[0][1]["region_name"], "garage")
        self.assertEqual(client.bucket, "jenkins-artifacts")
        self.assertEqual(
            client.put["LifecycleConfiguration"],
            MODULE.lifecycle(3),
        )

    def test_settings_reject_incomplete_or_out_of_boundary_values(self) -> None:
        for update in (
            {"AWS_ACCESS_KEY_ID": ""},
            {"S3_ENDPOINT": "https://s3.example.com"},
            {"S3_BUCKET": "market-raw"},
            {"S3_RETENTION_DAYS": "thirty"},
            {"S3_RETENTION_DAYS": "30"},
        ):
            with self.subTest(update=update), self.assertRaises(ValueError):
                MODULE.settings({**self.environ, **update})

    def test_apply_rejects_a_mismatched_readback(self) -> None:
        client = FakeClient([])

        def factory(*_args, **_kwargs):
            client.put_bucket_lifecycle_configuration = lambda **_values: None
            return client

        with self.assertRaisesRegex(RuntimeError, "did not retain"):
            MODULE.apply(self.environ, factory)

    def test_main_reports_only_the_retention_outcome(self) -> None:
        output = io.StringIO()
        with patch.object(MODULE, "apply") as apply, patch.dict(
            MODULE.os.environ, self.environ, clear=True
        ), redirect_stdout(output):
            MODULE.main()
        apply.assert_called_once_with(MODULE.os.environ)
        self.assertEqual(
            output.getvalue(),
            "Jenkins artifact lifecycle is configured for three days.\n",
        )


if __name__ == "__main__":
    unittest.main()
