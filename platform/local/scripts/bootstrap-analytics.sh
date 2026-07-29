#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

"${SCRIPT_DIR}/verify-data-path.sh"
"${SCRIPT_DIR}/build-analytics.sh"

printf 'Publishing the versioned demo holdings fixture to the application namespace...\n'
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  create configmap demo-portfolio-holdings \
  --from-file="holdings.json=${REPO_ROOT}/contracts/fixtures/demo-fund-portfolio.v2.json" \
  --dry-run=client --output=yaml \
  | kubectl --context "${KUBERNETES_CONTEXT}" apply -f -

printf 'Applying DuckDB analytics Jobs and the portfolio API...\n'
kubectl --context "${KUBERNETES_CONTEXT}" apply \
  -k "${REPO_ROOT}/platform/gitops/apps/local/portfolio-analytics"
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  wait job/portfolio-analytics-baseline --for=condition=Complete --timeout=5m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  rollout status deployment/portfolio-api --timeout=5m

"${SCRIPT_DIR}/verify-analytics.sh"
