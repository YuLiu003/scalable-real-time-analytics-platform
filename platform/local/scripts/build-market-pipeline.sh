#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"
SERVICE_DIR="${REPO_ROOT}/services/market-pipeline"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

docker_arch="$(docker info --format '{{.Architecture}}')"
case "${docker_arch}" in
  aarch64 | arm64)
    go_arch=arm64
    ;;
  x86_64 | amd64)
    go_arch=amd64
    ;;
  *)
    printf 'ERROR: unsupported Docker architecture %s.\n' "${docker_arch}" >&2
    exit 1
    ;;
esac

mkdir -p "${SERVICE_DIR}/.build" "${TMPDIR:-/tmp}/market-pipeline-gocache"

printf 'Testing market pipeline with %s...\n' "$(cd "${SERVICE_DIR}" && GOWORK=off go env GOVERSION)"
(
  cd "${SERVICE_DIR}"
  GOWORK=off GOCACHE="${TMPDIR:-/tmp}/market-pipeline-gocache" go test ./...

  for command in synthetic-producer raw-event-archiver archive-inspector topic-inspector; do
    printf 'Building linux/%s %s...\n' "${go_arch}" "${command}"
    CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" GOWORK=off \
      GOCACHE="${TMPDIR:-/tmp}/market-pipeline-gocache" \
      go build -trimpath -ldflags='-s -w' -o ".build/${command}" "./cmd/${command}"
  done
)

docker build --tag "${MARKET_PIPELINE_IMAGE}" "${SERVICE_DIR}"
kind load docker-image --name "${CLUSTER_NAME}" "${MARKET_PIPELINE_IMAGE}"
printf 'Loaded %s into kind cluster %s.\n' "${MARKET_PIPELINE_IMAGE}" "${CLUSTER_NAME}"
