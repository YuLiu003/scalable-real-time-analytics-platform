#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ANALYTICS_DIR="${REPO_ROOT}/services/portfolio-analytics"
API_DIR="${REPO_ROOT}/services/portfolio-api"
MARKET_DIR="${REPO_ROOT}/services/market-pipeline"
PYTHON_BIN="${PYTHON_BIN:-python3}"
COVERAGE_DIR="${COVERAGE_DIR:-${TMPDIR:-/tmp}/portfolio-quality-coverage}"

mkdir -p "${COVERAGE_DIR}" "${TMPDIR:-/tmp}/portfolio-api-quality-gocache" "${TMPDIR:-/tmp}/market-quality-gocache"

if ! "${PYTHON_BIN}" -c 'import boto3, coverage, duckdb' >/dev/null 2>&1; then
  printf 'ERROR: Python quality dependencies are unavailable. Run `%s -m pip install -r %s/requirements-dev.txt`.\n' \
    "${PYTHON_BIN}" "${ANALYTICS_DIR}" >&2
  exit 1
fi

printf 'Running Python analytics tests with a 100%% statement and branch gate...\n'
(
  cd "${ANALYTICS_DIR}"
  export COVERAGE_FILE="${COVERAGE_DIR}/portfolio-analytics.coverage"
  "${PYTHON_BIN}" -m coverage erase
  "${PYTHON_BIN}" -m coverage run -m unittest discover --start-directory tests --verbose
  "${PYTHON_BIN}" -m coverage report
  "${PYTHON_BIN}" -m coverage xml -o "${COVERAGE_DIR}/portfolio-analytics.xml"
)

assert_go_coverage() {
  local service_dir="$1"
  local cache_dir="$2"
  local profile="$3"
  shift 3

  (
    cd "${service_dir}"
    GOWORK=off GOCACHE="${cache_dir}" go test -race -count=1 \
      -covermode=atomic -coverprofile="${profile}" "$@"
  )
  local total
  total="$(cd "${service_dir}" && GOWORK=off GOCACHE="${cache_dir}" go tool cover -func="${profile}" | awk '/^total:/ {print $3}')"
  printf 'Measured Go coverage for %s: %s\n' "${service_dir#"${REPO_ROOT}/"}" "${total}"
  if [[ "${total}" != "100.0%" ]]; then
    printf 'ERROR: Go coverage for %s is %s; required 100.0%%.\n' "${service_dir}" "${total:-unknown}" >&2
    exit 1
  fi
}

printf 'Running portfolio API domain tests with the race detector and a 100%% statement gate...\n'
assert_go_coverage \
  "${API_DIR}" \
  "${TMPDIR:-/tmp}/portfolio-api-quality-gocache" \
  "${COVERAGE_DIR}/portfolio-api.out" \
  ./internal/...

printf 'Running market contract/fixture tests with the race detector and a 100%% statement gate...\n'
assert_go_coverage \
  "${MARKET_DIR}" \
  "${TMPDIR:-/tmp}/market-quality-gocache" \
  "${COVERAGE_DIR}/market-domain.out" \
  ./internal/event ./internal/synthetic

printf 'Building and race-testing every Go package in the feature services...\n'
(
  cd "${API_DIR}"
  GOWORK=off GOCACHE="${TMPDIR:-/tmp}/portfolio-api-quality-gocache" go test -race -count=1 ./...
  GOWORK=off GOCACHE="${TMPDIR:-/tmp}/portfolio-api-quality-gocache" go vet ./...
)
(
  cd "${MARKET_DIR}"
  GOWORK=off GOCACHE="${TMPDIR:-/tmp}/market-quality-gocache" go test -race -count=1 ./...
  GOWORK=off GOCACHE="${TMPDIR:-/tmp}/market-quality-gocache" go vet ./...
)

printf 'Checking shell and JSON source contracts...\n'
while IFS= read -r script; do
  bash -n "${REPO_ROOT}/${script}"
done < <(cd "${REPO_ROOT}" && rg --files platform/local/scripts scripts/ci scripts/cloud -g '*.sh' | sort)
"${REPO_ROOT}/scripts/ci/test-local-runtime-cleanup.sh"
"${PYTHON_BIN}" -m json.tool "${REPO_ROOT}/contracts/fixtures/demo-fund-portfolio.v2.json" >/dev/null

printf 'Portfolio feature quality gates passed with 100%% measured application coverage.\n'
