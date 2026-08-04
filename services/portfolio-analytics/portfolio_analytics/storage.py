from __future__ import annotations

import hashlib
import os
import re
from dataclasses import dataclass

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError

from .model import BronzeObject

DATE_PARTITION = re.compile(r"^date=[0-9]{4}-[0-9]{2}-[0-9]{2}/$")


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
        if not settings.region or not settings.bucket:
            raise ValueError("AWS region and S3 bucket are required")
        if bool(settings.access_key) != bool(settings.secret_key):
            raise ValueError("S3 access key and secret key must be configured together")
        if settings.endpoint and not settings.access_key:
            raise ValueError("a custom S3 endpoint requires static access credentials")
        return settings


class ObjectStore:
    def __init__(self, settings: S3Settings):
        self.bucket = settings.bucket
        options: dict[str, object] = {"region_name": settings.region}
        if settings.endpoint:
            options.update(
                endpoint_url=settings.endpoint,
                aws_access_key_id=settings.access_key,
                aws_secret_access_key=settings.secret_key,
                config=Config(s3={"addressing_style": "path"}),
            )
        self.client = boto3.client("s3", **options)

    def list_objects(self, prefix: str, required_key_segment: str = "") -> list[BronzeObject]:
        objects: list[BronzeObject] = []
        paginator = self.client.get_paginator("list_objects_v2")
        for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix):
            for item in page.get("Contents", []):
                key = item["Key"]
                if required_key_segment and required_key_segment not in key:
                    continue
                body = self.client.get_object(Bucket=self.bucket, Key=key)["Body"].read()
                objects.append(BronzeObject(key, body))
        return sorted(objects, key=lambda item: item.key)

    def list_latest_source_objects(
        self,
        prefix: str,
        required_key_segment: str,
        maximum_date_partitions: int,
    ) -> list[BronzeObject]:
        if maximum_date_partitions < 1:
            raise ValueError("maximum date partitions must be positive")
        date_prefixes: set[str] = set()
        paginator = self.client.get_paginator("list_objects_v2")
        for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix, Delimiter="/"):
            for item in page.get("CommonPrefixes", []):
                candidate = item.get("Prefix")
                if (
                    isinstance(candidate, str)
                    and candidate.startswith(prefix)
                    and DATE_PARTITION.fullmatch(candidate[len(prefix) :]) is not None
                ):
                    date_prefixes.add(candidate)

        objects: list[BronzeObject] = []
        matched_partitions = 0
        for date_prefix in sorted(date_prefixes, reverse=True):
            partition_objects = self.list_objects(date_prefix, required_key_segment)
            if not partition_objects:
                continue
            objects.extend(partition_objects)
            matched_partitions += 1
            if matched_partitions == maximum_date_partitions:
                break
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
