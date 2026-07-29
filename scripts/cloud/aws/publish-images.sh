#!/usr/bin/env bash
set -Eeuo pipefail

: "${AWS_REGION:?AWS_REGION is required}"
: "${ECR_REGISTRY:?ECR_REGISTRY is required}"
: "${IMAGE_TAG:?IMAGE_TAG is required}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
image_prefix="${ECR_REGISTRY}/investment-analytics"

aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin "${ECR_REGISTRY}"

IMAGE_PREFIX="${image_prefix}" IMAGE_TAG="${IMAGE_TAG}" \
  "${repo_root}/scripts/ci/build-portfolio-images.sh"

for image in market-pipeline portfolio-analytics portfolio-api; do
  docker push "${image_prefix}/${image}:${IMAGE_TAG}"
done
