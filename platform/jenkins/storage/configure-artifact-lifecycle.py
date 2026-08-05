#!/usr/bin/env python3
"""Apply and verify the bounded Garage lifecycle used by Jenkins artifacts."""

from __future__ import annotations

import os
from collections.abc import Mapping

import boto3
from botocore.config import Config


RULE_ID = "expire-jenkins-artifacts-after-3-days"


def settings(environ: Mapping[str, str]) -> tuple[str, str, str, str, int]:
    names = (
        "AWS_ACCESS_KEY_ID",
        "AWS_SECRET_ACCESS_KEY",
        "S3_ENDPOINT",
        "S3_BUCKET",
        "S3_RETENTION_DAYS",
    )
    values = tuple(environ.get(name, "").strip() for name in names)
    if any(not value for value in values):
        raise ValueError("artifact lifecycle configuration is incomplete")
    endpoint, bucket = values[2], values[3]
    if not endpoint.startswith("http://") or bucket != "jenkins-artifacts":
        raise ValueError(
            "artifact lifecycle target is outside the local Jenkins boundary"
        )
    try:
        days = int(values[4])
    except ValueError as exc:
        raise ValueError("artifact retention must be an integer") from exc
    if days != 3:
        raise ValueError("artifact retention must be exactly three days")
    return values[0], values[1], endpoint, bucket, days


def lifecycle(days: int) -> dict[str, list[dict[str, object]]]:
    return {
        "Rules": [
            {
                "ID": RULE_ID,
                "Status": "Enabled",
                "Filter": {"Prefix": ""},
                "Expiration": {"Days": days},
            }
        ]
    }


def apply(environ: Mapping[str, str], client_factory=boto3.client) -> None:
    access_key, secret_key, endpoint, bucket, days = settings(environ)
    client = client_factory(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        region_name="garage",
        config=Config(
            signature_version="s3v4",
            s3={"addressing_style": "path"},
            retries={"max_attempts": 3, "mode": "standard"},
        ),
    )
    expected = lifecycle(days)
    client.put_bucket_lifecycle_configuration(
        Bucket=bucket,
        LifecycleConfiguration=expected,
    )
    actual = client.get_bucket_lifecycle_configuration(Bucket=bucket)
    if actual.get("Rules") != expected["Rules"]:
        raise RuntimeError("Garage did not retain the required lifecycle rule")


def main() -> None:
    apply(os.environ)
    print("Jenkins artifact lifecycle is configured for three days.")


if __name__ == "__main__":  # pragma: no cover
    main()
