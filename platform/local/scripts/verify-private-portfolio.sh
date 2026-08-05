#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

context="${KUBERNETES_CONTEXT}"
namespace="${APPLICATION_NAMESPACE}"
test_token="test-only-private-portfolio-token-0000000000000000"
phase_timeout_seconds="${PRIVATE_PHASE_TIMEOUT_SECONDS:-300}"
analytics_job=private-portfolio-analytics-acceptance
port_forward_pid=""
port_forward_log=""
resources_created=false

cleanup() {
  status=$?
  trap - EXIT
  cleanup_failed=false
  if [[ -n "${port_forward_pid}" ]]; then
    kill "${port_forward_pid}" >/dev/null 2>&1 || true
    wait "${port_forward_pid}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${port_forward_log}" ]]; then
    rm -f "${port_forward_log}"
  fi
  if [[ "${resources_created}" == "true" ]]; then
    if ! kubectl --context "${context}" delete \
      -k "${REPO_ROOT}/platform/gitops/apps/private/portfolio-analytics" \
      --ignore-not-found --wait=true --timeout="${phase_timeout_seconds}s" >/dev/null 2>&1; then
      cleanup_failed=true
    fi
    if ! kubectl --context "${context}" --namespace "${namespace}" \
      delete job/private-portfolio-acceptance-producer job/"${analytics_job}" \
      --ignore-not-found --wait=true --timeout="${phase_timeout_seconds}s" >/dev/null 2>&1; then
      cleanup_failed=true
    fi
    if ! kubectl --context "${context}" --namespace "${namespace}" \
      delete secret/private-portfolio-holdings secret/private-portfolio-access \
      --ignore-not-found --wait=true --timeout="${phase_timeout_seconds}s" >/dev/null 2>&1; then
      cleanup_failed=true
    fi
    for resource in "${private_resources[@]}"; do
      if kubectl --context "${context}" --namespace "${namespace}" \
        get "${resource}" >/dev/null 2>&1; then
        cleanup_failed=true
      fi
    done
  fi
  if [[ "${cleanup_failed}" == "true" ]]; then
    printf 'ERROR: fictional private acceptance resources were not completely removed.\n' >&2
    if (( status == 0 )); then
      status=1
    fi
  fi
  exit "${status}"
}
trap cleanup EXIT

if [[ ! "${phase_timeout_seconds}" =~ ^[1-9][0-9]*$ ]]; then
  printf 'ERROR: PRIVATE_PHASE_TIMEOUT_SECONDS must be a positive integer.\n' >&2
  exit 2
fi

private_resources=(
  secret/private-portfolio-holdings
  secret/private-portfolio-access
  service/private-portfolio-api
  deployment/private-portfolio-api
  cronjob/private-portfolio-analytics
  serviceaccount/private-portfolio-analytics
  serviceaccount/private-portfolio-api
  job/private-portfolio-acceptance-producer
  job/private-portfolio-analytics-acceptance
)
for resource in "${private_resources[@]}"; do
  if kubectl --context "${context}" --namespace "${namespace}" \
    get "${resource}" >/dev/null 2>&1; then
    printf 'ERROR: refusing to overwrite an existing private portfolio runtime; use a disposable cluster.\n' >&2
    exit 1
  fi
done
resources_created=true

kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/scale-event-archiver --timeout=5m

kubectl --context "${context}" --namespace "${namespace}" \
  create secret generic private-portfolio-holdings \
  --from-file="holdings.json=${REPO_ROOT}/contracts/fixtures/demo-private-portfolio.v2.json" \
  --dry-run=client --output=yaml \
  | kubectl --context "${context}" apply -f - >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  create secret generic private-portfolio-access \
  --from-literal="token=${test_token}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${context}" apply -f - >/dev/null

kubectl --context "${context}" --namespace "${namespace}" \
  delete job/private-portfolio-acceptance-producer job/"${analytics_job}" \
  --ignore-not-found --wait=true >/dev/null
kubectl --context "${context}" apply \
  -k "${REPO_ROOT}/platform/gitops/apps/private/portfolio-analytics-acceptance" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/private-portfolio-acceptance-producer --for=condition=Complete --timeout="${phase_timeout_seconds}s"

lag_snapshot() {
  kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
    /topic-inspector \
    --topic market.prices.scale \
    --group scale-event-archiver-v1 \
    --lag \
    --expected-partitions 3 \
    --output json
}

deadline=$((SECONDS + phase_timeout_seconds))
lag_drained=false
while (( SECONDS < deadline )); do
  if lag_json="$(lag_snapshot 2>/dev/null)" && \
    [[ "$(python3 -c 'import json,sys; print(json.load(sys.stdin)["total_lag"])' <<<"${lag_json}")" == "0" ]]; then
    lag_drained=true
    break
  fi
  sleep 2
done
if [[ "${lag_drained}" != "true" ]]; then
  printf 'ERROR: private acceptance archive lag did not drain.\n' >&2
  exit 1
fi

kubectl --context "${context}" --namespace "${namespace}" \
  create job --from=cronjob/private-portfolio-analytics "${analytics_job}" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/"${analytics_job}" --for=condition=Complete --timeout="${phase_timeout_seconds}s"

analytics_log="$(kubectl --context "${context}" --namespace "${namespace}" logs job/"${analytics_job}")"
if [[ "${analytics_log}" != *'"event":"portfolio analytics build complete"'* ]] || \
  [[ "${analytics_log}" != *'"input_objects":3'* ]]; then
  printf 'ERROR: private analytics did not emit its aggregate completion record.\n' >&2
  exit 1
fi
if [[ "${analytics_log}" == *'DEMO-LIVE-'* ]] || \
  [[ "${analytics_log}" == *'total_market_value'* ]] || \
  [[ "${analytics_log}" == *'input_set_sha256'* ]] || \
  [[ "${analytics_log}" == *'result_sha256'* ]]; then
  printf 'ERROR: private analytics log exposed a disallowed portfolio detail.\n' >&2
  exit 1
fi

kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/private-portfolio-api --timeout="${phase_timeout_seconds}s"
service_proxy="/api/v1/namespaces/${namespace}/services/http:private-portfolio-api:8080/proxy"
config="$(kubectl --context "${context}" get --raw "${service_proxy}/api/v1/config")"
if [[ "${config}" != *'"portfolio_id":"private"'* ]] || \
  [[ "${config}" != *'"data_mode":"private"'* ]] || \
  [[ "${config}" != *'"access_token_required":true'* ]]; then
  printf 'ERROR: private API configuration is not protected.\n' >&2
  exit 1
fi
if kubectl --context "${context}" get --raw "${service_proxy}/api/v1/portfolios/private/allocation" \
  >/dev/null 2>&1; then
  printf 'ERROR: private allocation endpoint accepted an unauthenticated request.\n' >&2
  exit 1
fi

dashboard="$(kubectl --context "${context}" get --raw "${service_proxy}/")"
if [[ "${dashboard}" != *'id="access-token-form"'* ]] || [[ "${dashboard}" == *'/portfolios/demo/'* ]]; then
  printf 'ERROR: private-capable dashboard contract is missing.\n' >&2
  exit 1
fi

port_forward_log="$(mktemp "${TMPDIR:-/tmp}/private-portfolio-port-forward.XXXXXX")"
kubectl --context "${context}" --namespace "${namespace}" \
  port-forward service/private-portfolio-api 18081:8080 >"${port_forward_log}" 2>&1 &
port_forward_pid=$!
deadline=$((SECONDS + 30))
while (( SECONDS < deadline )); do
  if grep -Fq 'Forwarding from' "${port_forward_log}"; then
    break
  fi
  if ! kill -0 "${port_forward_pid}" 2>/dev/null; then
    printf 'ERROR: private API port-forward exited before verification.\n' >&2
    exit 1
  fi
  sleep 1
done
if ! grep -Fq 'Forwarding from' "${port_forward_log}"; then
  printf 'ERROR: private API port-forward did not become ready.\n' >&2
  exit 1
fi

PRIVATE_TEST_TOKEN="${test_token}" python3 - <<'PY'
import json
import os
import urllib.error
import urllib.request

url = "http://127.0.0.1:18081/api/v1/portfolios/private/allocation"
try:
    urllib.request.urlopen(
        urllib.request.Request(url, headers={"Authorization": "Bearer wrong"}),
        timeout=5,
    )
except urllib.error.HTTPError as error:
    if error.code != 401:
        raise
else:
    raise SystemExit("wrong private token was accepted")

request = urllib.request.Request(
    url,
    headers={"Authorization": f"Bearer {os.environ['PRIVATE_TEST_TOKEN']}"},
)
with urllib.request.urlopen(request, timeout=5) as response:
    if response.headers.get("Cache-Control") != "no-store":
        raise SystemExit("private result response is cacheable")
    result = json.load(response)
if (
    result["portfolio_id"] != "private"
    or result["input_object_count"] != 3
    or result["total_market_value"] != "500.00000000"
):
    raise SystemExit("private result identity or value mismatch")
if {item["asset_type"] for item in result["positions"]} != {"stock", "etf"}:
    raise SystemExit("private position semantics mismatch")
if result["benchmark"]["asset_type"] != "etf":
    raise SystemExit("private benchmark semantics mismatch")
PY

printf 'Private portfolio acceptance passed with protected API access and privacy-safe logs.\n'
