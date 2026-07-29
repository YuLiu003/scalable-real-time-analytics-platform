#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"
API_DIR="${REPO_ROOT}/services/portfolio-api"
ANALYTICS_DIR="${REPO_ROOT}/services/portfolio-analytics"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

if ! grep -Fqx "duckdb==${DUCKDB_VERSION}" "${ANALYTICS_DIR}/requirements.txt"; then
  printf 'ERROR: requirements.txt does not match DUCKDB_VERSION=%s.\n' "${DUCKDB_VERSION}" >&2
  exit 1
fi

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

mkdir -p "${API_DIR}/.build" "${TMPDIR:-/tmp}/portfolio-api-gocache"
(
  cd "${API_DIR}"
  printf 'Testing portfolio API with %s...\n' "$(GOWORK=off go env GOVERSION)"
  GOWORK=off GOCACHE="${TMPDIR:-/tmp}/portfolio-api-gocache" go test ./...
  for command in portfolio-api portfolio-inspector derived-reset; do
    printf 'Building linux/%s %s...\n' "${go_arch}" "${command}"
    CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" GOWORK=off \
      GOCACHE="${TMPDIR:-/tmp}/portfolio-api-gocache" \
      go build -trimpath -ldflags='-s -w' -o ".build/${command}" "./cmd/${command}"
  done
)

docker build --tag "${PORTFOLIO_API_IMAGE}" "${API_DIR}"
docker build \
  --build-arg "PYTHON_IMAGE=${PYTHON_IMAGE}" \
  --tag "${PORTFOLIO_ANALYTICS_IMAGE}" \
  "${ANALYTICS_DIR}"
docker run --rm --entrypoint python "${PORTFOLIO_ANALYTICS_IMAGE}" \
  -m unittest discover --start-directory tests --verbose

kind load docker-image --name "${CLUSTER_NAME}" "${PORTFOLIO_API_IMAGE}" "${PORTFOLIO_ANALYTICS_IMAGE}"
printf 'Loaded Slice 3 images into kind cluster %s.\n' "${CLUSTER_NAME}"
