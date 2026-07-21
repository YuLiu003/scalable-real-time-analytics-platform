#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

"${SCRIPT_DIR}/verify-data-path.sh"

context="${KUBERNETES_CONTEXT}"
namespace="${APPLICATION_NAMESPACE}"
service_proxy="/api/v1/namespaces/${namespace}/services/http:portfolio-api:8080/proxy"

kubectl --context "${context}" --namespace "${namespace}" get configmap demo-portfolio-holdings >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/portfolio-analytics-baseline --for=condition=Complete --timeout=120s

job_is_suspended() {
  [[ "$(kubectl --context "${context}" --namespace "${namespace}" get job "$1" --output=jsonpath='{.spec.suspend}')" == "true" ]]
}

# A verifier resumed after derived reset must replay before result-dependent
# readiness can recover. Every other state should begin with a Ready API.
if job_is_suspended portfolio-analytics-replay-reset || ! job_is_suspended portfolio-analytics-replay; then
  kubectl --context "${context}" --namespace "${namespace}" \
    rollout status deployment/portfolio-api --timeout=120s
fi

inspect_result() {
  kubectl --context "${context}" --namespace "${namespace}" exec deployment/portfolio-api -- \
    /portfolio-inspector \
    --portfolio "${PORTFOLIO_ID}" \
    --expected-total 5341.67000000 \
    --expected-positions 4 \
    --expected-silver-objects 1 \
    --expected-gold-objects 2 \
    "$@"
}

baseline_log="$(kubectl --context "${context}" --namespace "${namespace}" logs job/portfolio-analytics-baseline)"
baseline_sha="${baseline_log##*\"result_sha256\":\"}"
baseline_sha="${baseline_sha%%\"*}"
if [[ ! "${baseline_sha}" =~ ^[0-9a-f]{64}$ ]]; then
  printf 'ERROR: could not extract the baseline result SHA-256 from the completed Job.\n' >&2
  exit 1
fi

verify_user_visible_result() {
  local api_result dashboard
  api_result="$(kubectl --context "${context}" get --raw "${service_proxy}/api/v1/portfolios/${PORTFOLIO_ID}/allocation")"
  if [[ "${api_result}" != *'"total_market_value":"5341.67000000"'* ]] || [[ "${api_result}" != *'"positions":['* ]]; then
    printf 'ERROR: user-visible allocation API did not return the expected result.\n' >&2
    exit 1
  fi
  dashboard="$(kubectl --context "${context}" get --raw "${service_proxy}/")"
  if [[ "${dashboard}" != *'<title>Demo Portfolio Allocation</title>'* ]]; then
    printf 'ERROR: dashboard HTML was not served.\n' >&2
    exit 1
  fi
}

result_available=false
if baseline_inspection="$(inspect_result --expected-sha256 "${baseline_sha}" 2>/dev/null)"; then
  result_available=true
  printf '%s\n' "${baseline_inspection}"
  verify_user_visible_result
fi

if job_is_suspended portfolio-analytics-query; then
  if [[ "${result_available}" != true ]]; then
    printf 'ERROR: the Parquet query proof cannot start without the baseline result.\n' >&2
    exit 1
  fi
  printf 'Running the live DuckDB query over silver and gold Parquet...\n'
  kubectl --context "${context}" --namespace "${namespace}" patch \
    job portfolio-analytics-query --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
fi
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/portfolio-analytics-query --for=condition=Complete --timeout=120s
kubectl --context "${context}" --namespace "${namespace}" logs job/portfolio-analytics-query \
  | grep -F 'DuckDB Parquet query passed'

if job_is_suspended portfolio-analytics-replay-reset; then
  if [[ "${result_available}" != true ]]; then
    printf 'ERROR: derived reset cannot start because the baseline result is already unavailable.\n' >&2
    exit 1
  fi
  printf 'Deleting only derived products to test bronze replay...\n'
  kubectl --context "${context}" --namespace "${namespace}" patch \
    job portfolio-analytics-replay-reset --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
fi
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/portfolio-analytics-replay-reset --for=condition=Complete --timeout=120s
kubectl --context "${context}" --namespace "${namespace}" logs job/portfolio-analytics-replay-reset \
  | grep -F 'derived analytics reset'

if job_is_suspended portfolio-analytics-replay; then
  if inspect_result >/dev/null 2>&1; then
    printf 'ERROR: portfolio result remained available after derived reset.\n' >&2
    exit 1
  fi
  api_pod="$(kubectl --context "${context}" --namespace "${namespace}" get pod \
    -l app.kubernetes.io/name=portfolio-api --output=jsonpath='{.items[0].metadata.name}')"
  if [[ "$(kubectl --context "${context}" get --raw "/api/v1/namespaces/${namespace}/pods/${api_pod}:8080/proxy/healthz")" != "ok" ]]; then
    printf 'ERROR: portfolio API process was not live while its result was unavailable.\n' >&2
    exit 1
  fi
  if kubectl --context "${context}" get --raw "/api/v1/namespaces/${namespace}/pods/${api_pod}:8080/proxy/readyz" >/dev/null 2>&1; then
    printf 'ERROR: portfolio API remained ready without its result object.\n' >&2
    exit 1
  fi

  printf 'Rebuilding from bronze without invoking a producer or Kafka...\n'
  kubectl --context "${context}" --namespace "${namespace}" patch \
    job portfolio-analytics-replay --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
fi

kubectl --context "${context}" --namespace "${namespace}" \
  wait job/portfolio-analytics-replay --for=condition=Complete --timeout=120s
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/portfolio-api --timeout=120s >/dev/null
final_inspection="$(inspect_result --expected-sha256 "${baseline_sha}")"
printf '%s\n' "${final_inspection}"
verify_user_visible_result
printf 'Replay reproduced canonical result SHA-256 %s.\n' "${baseline_sha}"

printf '\nSlice 3 analytics and replay verification passed.\n'
kubectl --context "${context}" --namespace "${namespace}" get deployment,service,job \
  -l app.kubernetes.io/part-of=investment-analytics-platform
