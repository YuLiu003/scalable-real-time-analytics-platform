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
  ./internal/alpaca ./internal/archivemetrics ./internal/benchmark ./internal/event ./internal/scale ./internal/synthetic

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
"${REPO_ROOT}/scripts/ci/test-capacity-observability.sh"
"${PYTHON_BIN}" "${REPO_ROOT}/scripts/ci/validate-public-fixtures.py"
scale_config_log="${COVERAGE_DIR}/scale-config-validation.log"
scale_config_artifact_dir="${COVERAGE_DIR}/invalid-scale-config-artifacts"
mkdir -p "${scale_config_artifact_dir}"
printf 'stale evidence\n' >"${scale_config_artifact_dir}/report.json"
if SCALE_EVENT_COUNT=1200 SCALE_ARCHIVER_DELAY_MS=30 \
  SCALE_ARTIFACT_DIR="${scale_config_artifact_dir}" \
  "${REPO_ROOT}/platform/local/scripts/verify-scale-lab.sh" \
  >"${scale_config_log}" 2>&1; then
  printf 'ERROR: scale verification accepted a backlog shorter than the HPA observation window.\n' >&2
  exit 1
fi
grep -Fq 'must retain at least 35 seconds of injected work per partition' \
  "${scale_config_log}"
if [[ -e "${scale_config_artifact_dir}/report.json" ]]; then
  printf 'ERROR: invalid scale configuration left stale evidence available.\n' >&2
  exit 1
fi
capacity_config_log="${COVERAGE_DIR}/capacity-config-validation.log"
capacity_config_artifact_dir="$(mktemp -d "${COVERAGE_DIR}/invalid-capacity-config-artifacts.XXXXXX")"
capacity_path_log="${COVERAGE_DIR}/capacity-path-validation.log"
printf 'protected evidence\n' >"${COVERAGE_DIR}/summary.json"
if CAPACITY_SUITE_ID=.. \
  CAPACITY_ALLOCATED_CPUS=4 CAPACITY_ALLOCATED_MEMORY_GIB=8 CAPACITY_ALLOCATED_DISK_GIB=30 \
  CAPACITY_ARTIFACT_DIR="${capacity_config_artifact_dir}" \
  "${REPO_ROOT}/platform/local/scripts/verify-capacity-benchmark.sh" \
  >"${capacity_path_log}" 2>&1; then
  printf 'ERROR: capacity benchmark accepted an unsafe suite ID.\n' >&2
  exit 1
fi
grep -Fq 'CAPACITY_SUITE_ID' "${capacity_path_log}"
grep -Fqx 'protected evidence' "${COVERAGE_DIR}/summary.json"
if CAPACITY_SUITE_ID=badcfg CAPACITY_EVENT_COUNTS=1 \
  CAPACITY_ALLOCATED_CPUS=4 CAPACITY_ALLOCATED_MEMORY_GIB=8 CAPACITY_ALLOCATED_DISK_GIB=30 \
  CAPACITY_ARTIFACT_DIR="${capacity_config_artifact_dir}" \
  "${REPO_ROOT}/platform/local/scripts/verify-capacity-benchmark.sh" \
  >"${capacity_config_log}" 2>&1; then
  printf 'ERROR: capacity benchmark accepted an event count below its contract.\n' >&2
  exit 1
fi
grep -Fq 'event counts must be comma-separated integers' "${capacity_config_log}"
if [[ -e "${capacity_config_artifact_dir}/badcfg" ]]; then
  printf 'ERROR: invalid capacity configuration reserved an artifact suite.\n' >&2
  exit 1
fi
capacity_existing_log="${COVERAGE_DIR}/capacity-existing-suite.log"
mkdir "${capacity_config_artifact_dir}/preserve"
printf 'stale evidence\n' >"${capacity_config_artifact_dir}/preserve/summary.json"
if CAPACITY_SUITE_ID=preserve CAPACITY_EVENT_COUNTS=600 \
  CAPACITY_ALLOCATED_CPUS=4 CAPACITY_ALLOCATED_MEMORY_GIB=8 CAPACITY_ALLOCATED_DISK_GIB=30 \
  CAPACITY_ARTIFACT_DIR="${capacity_config_artifact_dir}" \
  "${REPO_ROOT}/platform/local/scripts/verify-capacity-benchmark.sh" \
  >"${capacity_existing_log}" 2>&1; then
  printf 'ERROR: capacity benchmark reused an existing suite.\n' >&2
  exit 1
fi
grep -Fq 'already exists' "${capacity_existing_log}"
grep -Fqx 'stale evidence' "${capacity_config_artifact_dir}/preserve/summary.json"
"${PYTHON_BIN}" -m json.tool "${REPO_ROOT}/contracts/fixtures/demo-fund-portfolio.v2.json" >/dev/null
if ! grep -Fqx 'cpu_rate_window_seconds=25' \
  "${REPO_ROOT}/platform/local/scripts/verify-capacity-benchmark.sh"; then
  printf 'ERROR: capacity CPU window must retain the reviewed scrape-jitter margin.\n' >&2
  exit 1
fi
if ! grep -Fqx $'\tCAPACITY_EVENT_COUNTS=10000 CAPACITY_REPETITIONS=1 CAPACITY_TARGET_RATE=250 ./scripts/verify-capacity-benchmark.sh' \
  "${REPO_ROOT}/platform/local/Makefile"; then
  printf 'ERROR: capacity smoke must retain a controlled rate long enough for its CPU window.\n' >&2
  exit 1
fi
if ! grep -Fqx 'resource_ingestion_timeout_seconds=30' \
  "${REPO_ROOT}/platform/local/scripts/verify-capacity-benchmark.sh"; then
  printf 'ERROR: capacity resource queries must retain the bounded ingestion retry.\n' >&2
  exit 1
fi
if ! grep -Fqx '  wait_for_resource_ranges "${raw_dir}" "${start_seconds}" "${end_seconds}"' \
  "${REPO_ROOT}/platform/local/scripts/verify-capacity-benchmark.sh"; then
  printf 'ERROR: capacity reporting must wait on the original resource-query boundaries.\n' >&2
  exit 1
fi
kubectl kustomize "${REPO_ROOT}/platform/gitops/apps/private/market-feed" >/dev/null

printf 'Portfolio feature quality gates passed with 100%% measured application coverage.\n'
