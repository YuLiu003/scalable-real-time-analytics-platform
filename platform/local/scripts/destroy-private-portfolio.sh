#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

confirmation_name=private-portfolio
if [[ "${CONFIRM_DESTROY_PRIVATE_PORTFOLIO:-}" != "${confirmation_name}" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'ERROR: set CONFIRM_DESTROY_PRIVATE_PORTFOLIO=%s for non-interactive deletion.\n' "${confirmation_name}" >&2
    exit 1
  fi
  printf 'Type %s to delete private workloads, runtime Secrets, and checkpoint state: ' "${confirmation_name}"
  read -r confirmation
  if [[ "${confirmation}" != "${confirmation_name}" ]]; then
    printf 'Deletion cancelled.\n'
    exit 0
  fi
fi

kubectl --context "${KUBERNETES_CONTEXT}" delete \
  -k "${REPO_ROOT}/platform/gitops/apps/private/portfolio-analytics" \
  --ignore-not-found --wait=true
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  delete job --selector app.kubernetes.io/name=private-portfolio-analytics \
  --ignore-not-found --wait=true
kubectl --context "${KUBERNETES_CONTEXT}" delete \
  -k "${REPO_ROOT}/platform/gitops/apps/private/market-feed" \
  --ignore-not-found --wait=true
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  delete secret alpaca-market-feed private-portfolio-holdings private-portfolio-access \
  --ignore-not-found --wait=true

printf 'Deleted private workloads, input Secrets, API token, and feed checkpoint.\n'

if [[ "${PURGE_ALL_LOCAL_MARKET_DATA:-false}" == "true" ]]; then
  if [[ "${CONFIRM_PURGE_ALL_LOCAL_MARKET_DATA:-}" != "all-local-market-data" ]]; then
    printf 'ERROR: set CONFIRM_PURGE_ALL_LOCAL_MARKET_DATA=all-local-market-data to delete shared Kafka and Garage data.\n' >&2
    exit 1
  fi
  CONFIRM_DESTROY_ANALYTICS=portfolio-analytics "${SCRIPT_DIR}/destroy-analytics.sh"
  CONFIRM_DESTROY_DATA_PATH=market-data-path "${SCRIPT_DIR}/destroy-data-path.sh"
  printf 'Deleted all local Kafka, Garage, bronze, and derived data, including private records.\n'
else
  printf 'Kafka and Garage use shared local storage. Market events and derived private quantities, allocations, benchmark, and portfolio valuations remain until the confirmed full-data purge.\n'
fi
