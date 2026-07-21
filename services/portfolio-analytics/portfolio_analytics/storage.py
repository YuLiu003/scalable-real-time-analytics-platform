from __future__ import annotations

import hashlib
import os
from dataclasses import dataclass

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError

from .model import BronzeObject


@dataclass(frozen=True)
class S3Settings:
    endpoint: str
    region: str
    bucket: str
    access_key: str
    secret_key: str

    @classmethod
    def from_environment(cls) -> "S3Settings":
        settings = cls(
            endpoint=os.environ.get("S3_ENDPOINT", ""),
            region=os.environ.get("AWS_REGION", ""),
            bucket=os.environ.get("S3_BUCKET", ""),
            access_key=os.environ.get("AWS_ACCESS_KEY_ID", ""),
            secret_key=os.environ.get("AWS_SECRET_ACCESS_KEY", ""),
        )
        if not all(settings.__dict__.values()):
            raise ValueError("S3 endpoint, region, bucket, access key, and secret key are required")
        return settings


class ObjectStore:
    def __init__(self, settings: S3Settings):
        self.bucket = settings.bucket
        self.client = boto3.client(
            "s3",
            endpoint_url=settings.endpoint,
            region_name=settings.region,
            aws_access_key_id=settings.access_key,
            aws_secret_access_key=settings.secret_key,
            config=Config(s3={"addressing_style": "path"}),
        )

    def list_objects(self, prefix: str) -> list[BronzeObject]:
        objects: list[BronzeObject] = []
        paginator = self.client.get_paginator("list_objects_v2")
        for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix):
            for item in page.get("Contents", []):
                key = item["Key"]
                body = self.client.get_object(Bucket=self.bucket, Key=key)["Body"].read()
                objects.append(BronzeObject(key, body))
        return sorted(objects, key=lambda item: item.key)

    def get(self, key: str) -> bytes:
        return self.client.get_object(Bucket=self.bucket, Key=key)["Body"].read()

    def put_immutable(self, key: str, data: bytes, content_type: str) -> str:
        digest = hashlib.sha256(data).hexdigest()
        try:
            existing = self.client.head_object(Bucket=self.bucket, Key=key)
        except ClientError as error:
            code = error.response.get("Error", {}).get("Code")
            if code not in {"404", "NoSuchKey", "NotFound"}:
                raise
        else:
            if existing.get("Metadata", {}).get("sha256", "").lower() == digest:
                return "duplicate"
            raise ValueError(f"immutable analytical object collision: {key}")
        self.client.put_object(
            Bucket=self.bucket,
            Key=key,
            Body=data,
            ContentType=content_type,
            Metadata={"sha256": digest},
        )
        return "created"

    def put_latest(self, key: str, data: bytes) -> None:
        self.client.put_object(
            Bucket=self.bucket,
            Key=key,
            Body=data,
            ContentType="application/json",
            Metadata={"sha256": hashlib.sha256(data).hexdigest()},
        )
