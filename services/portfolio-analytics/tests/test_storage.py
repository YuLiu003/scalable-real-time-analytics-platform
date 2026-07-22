from __future__ import annotations

import hashlib
import io
import unittest
from unittest.mock import Mock, patch

from botocore.exceptions import ClientError

from portfolio_analytics.storage import ObjectStore, S3Settings


def client_error(code: str) -> ClientError:
    return ClientError({"Error": {"Code": code, "Message": code}}, "HeadObject")


class StorageTests(unittest.TestCase):
    def test_settings_support_local_and_workload_identity_environments(self) -> None:
        with patch.dict("os.environ", {}, clear=True), self.assertRaisesRegex(ValueError, "required"):
            S3Settings.from_environment()
        values = {
            "S3_ENDPOINT": "http://garage",
            "AWS_REGION": "garage",
            "S3_BUCKET": "analytics",
            "AWS_ACCESS_KEY_ID": "access",
            "AWS_SECRET_ACCESS_KEY": "secret",
        }
        with patch.dict("os.environ", values, clear=True):
            settings = S3Settings.from_environment()
        self.assertEqual(settings.bucket, "analytics")

        cloud_values = {"AWS_REGION": "us-west-2", "S3_BUCKET": "analytics"}
        with patch.dict("os.environ", cloud_values, clear=True):
            settings = S3Settings.from_environment()
        self.assertEqual(settings.endpoint, "")

        for invalid in (
            {**cloud_values, "AWS_ACCESS_KEY_ID": "partial"},
            {**cloud_values, "S3_ENDPOINT": "http://garage"},
        ):
            with self.subTest(invalid=invalid), patch.dict("os.environ", invalid, clear=True), self.assertRaises(
                ValueError
            ):
                S3Settings.from_environment()

    @patch("portfolio_analytics.storage.boto3.client")
    def test_constructor_configures_path_style_s3(self, client: Mock) -> None:
        settings = S3Settings("http://garage", "garage", "analytics", "access", "secret")
        store = ObjectStore(settings)
        self.assertEqual(store.bucket, "analytics")
        _, kwargs = client.call_args
        self.assertEqual(kwargs["endpoint_url"], "http://garage")
        self.assertEqual(kwargs["config"].s3["addressing_style"], "path")

        ObjectStore(S3Settings("", "us-west-2", "analytics", "", ""))
        _, kwargs = client.call_args
        self.assertEqual(kwargs, {"region_name": "us-west-2"})

    def test_list_and_get_objects(self) -> None:
        body = Mock()
        body.read.return_value = b"payload"
        client = Mock()
        client.get_paginator.return_value.paginate.return_value = [
            {"Contents": [{"Key": "z.json"}, {"Key": "a.json"}]},
            {},
        ]
        client.get_object.return_value = {"Body": body}
        store = object.__new__(ObjectStore)
        store.bucket = "analytics"
        store.client = client
        objects = store.list_objects("bronze/")
        self.assertEqual([item.key for item in objects], ["a.json", "z.json"])
        self.assertEqual(store.get("latest.json"), b"payload")

    def test_put_immutable_creates_duplicate_and_rejects_collisions(self) -> None:
        data = b"parquet"
        digest = hashlib.sha256(data).hexdigest()
        store = object.__new__(ObjectStore)
        store.bucket = "analytics"

        created = Mock()
        created.head_object.side_effect = client_error("404")
        store.client = created
        self.assertEqual(store.put_immutable("gold/run.parquet", data, "application/parquet"), "created")
        created.put_object.assert_called_once()

        duplicate = Mock()
        duplicate.head_object.return_value = {"Metadata": {"sha256": digest.upper()}}
        store.client = duplicate
        self.assertEqual(store.put_immutable("gold/run.parquet", data, "application/parquet"), "duplicate")
        duplicate.put_object.assert_not_called()

        collision = Mock()
        collision.head_object.return_value = {"Metadata": {"sha256": "different"}}
        store.client = collision
        with self.assertRaisesRegex(ValueError, "collision"):
            store.put_immutable("gold/run.parquet", data, "application/parquet")

        unexpected = Mock()
        unexpected.head_object.side_effect = client_error("AccessDenied")
        store.client = unexpected
        with self.assertRaises(ClientError):
            store.put_immutable("gold/run.parquet", data, "application/parquet")

    def test_put_latest_writes_replaceable_json_with_digest(self) -> None:
        client = Mock()
        store = object.__new__(ObjectStore)
        store.bucket = "analytics"
        store.client = client
        store.put_latest("gold/latest.json", b"{}\n")
        _, kwargs = client.put_object.call_args
        self.assertEqual(kwargs["ContentType"], "application/json")
        self.assertEqual(kwargs["Metadata"]["sha256"], hashlib.sha256(b"{}\n").hexdigest())


if __name__ == "__main__":
    unittest.main()
