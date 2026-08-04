#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

feed_environment="${PRIVATE_FEED_ENV_FILE:-}"
holdings_file="${PRIVATE_HOLDINGS_FILE:-}"
access_token_file="${PRIVATE_ACCESS_TOKEN_FILE:-}"
python_bin="${PYTHON_BIN:-python3}"
context="${KUBERNETES_CONTEXT}"
namespace="${APPLICATION_NAMESPACE}"
phase_timeout_seconds="${PRIVATE_PHASE_TIMEOUT_SECONDS:-600}"

for variable_name in PRIVATE_FEED_ENV_FILE PRIVATE_HOLDINGS_FILE PRIVATE_ACCESS_TOKEN_FILE; do
  if [[ -z "${!variable_name:-}" ]]; then
    printf 'ERROR: %s must name an absolute mode-0600 file outside the repository.\n' "${variable_name}" >&2
    exit 2
  fi
done
if [[ ! "${phase_timeout_seconds}" =~ ^[1-9][0-9]*$ ]]; then
  printf 'ERROR: PRIVATE_PHASE_TIMEOUT_SECONDS must be a positive integer.\n' >&2
  exit 2
fi

PYTHONPATH="${REPO_ROOT}/services/portfolio-analytics" "${python_bin}" \
  -m portfolio_analytics.private_inputs \
  --feed-environment "${feed_environment}" \
  --holdings "${holdings_file}" \
  --access-token "${access_token_file}" \
  --repository-root "${REPO_ROOT}"

"${SCRIPT_DIR}/bootstrap.sh"

if kubectl --context "${context}" --namespace "${DATA_NAMESPACE}" \
  get kafka "${KAFKA_CLUSTER_NAME}" >/dev/null 2>&1 && \
  kubectl --context "${context}" --namespace "${namespace}" \
    get deployment raw-event-archiver >/dev/null 2>&1; then
  printf 'Reusing the existing Kafka and archive data path without replaying exact-count fixtures.\n'
  kubectl --context "${context}" --namespace "${DATA_NAMESPACE}" \
    wait kafka/"${KAFKA_CLUSTER_NAME}" --for=condition=Ready --timeout=5m
  kubectl --context "${context}" --namespace "${namespace}" \
    rollout status deployment/raw-event-archiver --timeout=5m
else
  "${SCRIPT_DIR}/bootstrap-data-path.sh"
fi

"${SCRIPT_DIR}/build-market-pipeline.sh"
"${SCRIPT_DIR}/build-analytics.sh"

if kubectl --context "${context}" --namespace "${namespace}" \
  get cronjob/private-portfolio-analytics >/dev/null 2>&1; then
  printf 'Suspending periodic private analytics before rotating runtime inputs.\n'
  kubectl --context "${context}" --namespace "${namespace}" \
    patch cronjob/private-portfolio-analytics --type=merge \
    --patch '{"spec":{"suspend":true}}' >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    delete job --selector app.kubernetes.io/name=private-portfolio-analytics \
    --ignore-not-found --wait=true --timeout="${phase_timeout_seconds}s" >/dev/null
fi

kubectl --context "${context}" --namespace "${namespace}" \
  create secret generic alpaca-market-feed \
  --from-env-file="${feed_environment}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${context}" apply -f - >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  create secret generic private-portfolio-holdings \
  --from-file="holdings.json=${holdings_file}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${context}" apply -f - >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  create secret generic private-portfolio-access \
  --from-file="token=${access_token_file}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${context}" apply -f - >/dev/null

printf 'Starting the credential-backed market adapter. Private values remain in runtime Secrets.\n'
kubectl --context "${context}" apply \
  -k "${REPO_ROOT}/platform/gitops/apps/private/market-feed" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  wait kafkauser/alpaca-market-producer --for=condition=Ready --timeout=5m
kubectl --context "${context}" --namespace "${namespace}" \
  rollout restart deployment/alpaca-market-feed >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/alpaca-market-feed --timeout="${phase_timeout_seconds}s"

lag_snapshot() {
  kubectl --context "${context}" --namespace "${namespace}" exec deployment/raw-event-archiver -- \
    /topic-inspector \
    --topic market.prices \
    --group raw-event-archiver-v1 \
    --lag \
    --expected-partitions 3 \
    --output json
}

printf 'Waiting for the acknowledged backfill to reach immutable archive storage.\n'
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
  printf 'ERROR: private market archive lag did not drain before the timeout.\n' >&2
  exit 1
fi

kubectl --context "${context}" apply \
  -k "${REPO_ROOT}/platform/gitops/apps/private/portfolio-analytics" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  rollout restart deployment/private-portfolio-api >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  delete job --selector app.kubernetes.io/name=private-portfolio-analytics \
  --ignore-not-found --wait=true --timeout="${phase_timeout_seconds}s" >/dev/null

bootstrap_job="private-portfolio-bootstrap-$(date -u +%Y%m%dt%H%M%S)"
kubectl --context "${context}" --namespace "${namespace}" \
  create job --from=cronjob/private-portfolio-analytics "${bootstrap_job}" >/dev/null
if ! kubectl --context "${context}" --namespace "${namespace}" \
  wait job/"${bootstrap_job}" --for=condition=Complete --timeout="${phase_timeout_seconds}s"; then
  printf 'ERROR: private analytics did not publish a complete result; logs remain privacy-redacted.\n' >&2
  exit 1
fi
kubectl --context "${context}" --namespace "${namespace}" \
  patch cronjob/private-portfolio-analytics --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/private-portfolio-api --timeout=5m

config_proxy="/api/v1/namespaces/${namespace}/services/http:private-portfolio-api:8080/proxy/api/v1/config"
config="$(kubectl --context "${context}" get --raw "${config_proxy}")"
if [[ "${config}" != *'"data_mode":"private"'* ]] || \
  [[ "${config}" != *'"access_token_required":true'* ]]; then
  printf 'ERROR: private API did not expose the expected protected runtime configuration.\n' >&2
  exit 1
fi

printf '\nPrivate portfolio is ready. Open it through a loopback-only port forward:\n'
printf '  make -C platform/local private-portfolio-dashboard\n'
printf 'Paste the value from PRIVATE_ACCESS_TOKEN_FILE into the dashboard when prompted.\n'
