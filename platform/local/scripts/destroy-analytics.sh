#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

confirmation_name=portfolio-analytics
if [[ "${CONFIRM_DESTROY_ANALYTICS:-}" != "${confirmation_name}" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'ERROR: set CONFIRM_DESTROY_ANALYTICS=%s for non-interactive deletion.\n' "${confirmation_name}" >&2
    exit 1
  fi
  printf 'Type %s to delete derived silver/gold objects and Slice 3 workloads: ' "${confirmation_name}"
  read -r confirmation
  if [[ "${confirmation}" != "${confirmation_name}" ]]; then
    printf 'Deletion cancelled.\n'
    exit 0
  fi
fi

if kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  get deployment portfolio-api >/dev/null 2>&1; then
  kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
    exec deployment/portfolio-api -- /derived-reset
fi
kubectl --context "${KUBERNETES_CONTEXT}" delete \
  -k "${REPO_ROOT}/platform/gitops/apps/local/portfolio-analytics" \
  --ignore-not-found --wait=true
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  delete configmap demo-portfolio-holdings --ignore-not-found

printf 'Deleted Slice 3 workloads and derived analytics; bronze inputs remain intact.\n'
