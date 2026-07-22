#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
API_DIR="${REPO_ROOT}/services/portfolio-api"
MARKET_DIR="${REPO_ROOT}/services/market-pipeline"
ANALYTICS_DIR="${REPO_ROOT}/services/portfolio-analytics"
IMAGE_PREFIX="${IMAGE_PREFIX:-local}"
IMAGE_TAG="${IMAGE_TAG:-quality}"
GO_ARCH="${GO_ARCH:-amd64}"

# shellcheck disable=SC1091
source "${REPO_ROOT}/platform/local/versions.lock"

mkdir -p "${API_DIR}/.build" "${MARKET_DIR}/.build" \
  "${TMPDIR:-/tmp}/portfolio-api-image-gocache" "${TMPDIR:-/tmp}/market-image-gocache"

for command in portfolio-api portfolio-inspector derived-reset; do
  (
    cd "${API_DIR}"
    CGO_ENABLED=0 GOOS=linux GOARCH="${GO_ARCH}" GOWORK=off \
      GOCACHE="${TMPDIR:-/tmp}/portfolio-api-image-gocache" \
      go build -trimpath -ldflags='-s -w' -o ".build/${command}" "./cmd/${command}"
  )
done

for command in synthetic-producer raw-event-archiver archive-inspector topic-inspector; do
  (
    cd "${MARKET_DIR}"
    CGO_ENABLED=0 GOOS=linux GOARCH="${GO_ARCH}" GOWORK=off \
      GOCACHE="${TMPDIR:-/tmp}/market-image-gocache" \
      go build -trimpath -ldflags='-s -w' -o ".build/${command}" "./cmd/${command}"
  )
done

docker build --tag "${IMAGE_PREFIX}/portfolio-api:${IMAGE_TAG}" "${API_DIR}"
docker build \
  --build-arg "PYTHON_IMAGE=${PYTHON_IMAGE}" \
  --tag "${IMAGE_PREFIX}/portfolio-analytics:${IMAGE_TAG}" \
  "${ANALYTICS_DIR}"
docker build --tag "${IMAGE_PREFIX}/market-pipeline:${IMAGE_TAG}" "${MARKET_DIR}"

printf 'Built %s/{portfolio-api,portfolio-analytics,market-pipeline}:%s\n' "${IMAGE_PREFIX}" "${IMAGE_TAG}"
