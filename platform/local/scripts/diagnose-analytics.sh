#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
output_dir="${TMPDIR:-/tmp}/portfolio-analytics-diagnostics-${timestamp}"
mkdir -p "${output_dir}"

capture() {
  local output_file="$1"
  shift
  "$@" >"${output_dir}/${output_file}" 2>&1 || true
}

capture resources.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  get deployment,service,job,pod -l app.kubernetes.io/part-of=investment-analytics-platform -o wide
capture events.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  get events --sort-by=.lastTimestamp
capture baseline-logs.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  logs job/portfolio-analytics-baseline
capture replay-logs.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  logs job/portfolio-analytics-replay
capture query-logs.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  logs job/portfolio-analytics-query
capture api-logs.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  logs deployment/portfolio-api
capture result-metadata.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  exec deployment/portfolio-api -- /portfolio-inspector --portfolio "${PORTFOLIO_ID}"

printf 'Analytics diagnostics written to %s\n' "${output_dir}"
printf 'Secret objects, Secret values, bronze payloads, and portfolio result contents were intentionally excluded.\n'
