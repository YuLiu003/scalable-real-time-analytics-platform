#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

wait_for_selected_deletion() {
  local resource_type="$1"
  local selector="$2"
  local -a resources=()
  while IFS= read -r resource; do
    if [[ -n "${resource}" ]]; then
      resources+=("${resource}")
    fi
  done < <(kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
    get "${resource_type}" --selector "${selector}" --output=name)
  if (( ${#resources[@]} > 0 )); then
    kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
      wait --for=delete "${resources[@]}" --timeout=5m
  fi
}

confirmation_name=market-data-path
if [[ "${CONFIRM_DESTROY_DATA_PATH:-}" != "${confirmation_name}" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'ERROR: set CONFIRM_DESTROY_DATA_PATH=%s for non-interactive deletion.\n' "${confirmation_name}" >&2
    exit 1
  fi
  printf 'Type %s to delete Kafka, Garage, archived data, and all Slice 2 jobs: ' "${confirmation_name}"
  read -r confirmation
  if [[ "${confirmation}" != "${confirmation_name}" ]]; then
    printf 'Deletion cancelled.\n'
    exit 0
  fi
fi

kubectl --context "${KUBERNETES_CONTEXT}" delete \
  -k "${REPO_ROOT}/platform/gitops/apps/local/market-pipeline" \
  --ignore-not-found --wait=true
kubectl --context "${KUBERNETES_CONTEXT}" delete \
  -k "${REPO_ROOT}/platform/gitops/platform/local/market-data-services" \
  --ignore-not-found --wait=true
helm uninstall "${STRIMZI_RELEASE}" --kube-context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" --ignore-not-found
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
  delete secret garage-server-config market-archive-credentials --ignore-not-found
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  delete secret market-archive-credentials --ignore-not-found
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  delete configmap "${KAFKA_CLUSTER_NAME}-cluster-ca" --ignore-not-found
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
  delete persistentvolumeclaim data-garage-0 --ignore-not-found --wait=true

wait_for_selected_deletion pod "strimzi.io/cluster=${KAFKA_CLUSTER_NAME}"
wait_for_selected_deletion pod 'name=strimzi-cluster-operator'
wait_for_selected_deletion pod 'app.kubernetes.io/name=garage'
wait_for_selected_deletion persistentvolumeclaim "strimzi.io/cluster=${KAFKA_CLUSTER_NAME}"

printf 'Deleted the local Slice 2 market data path and its archived data.\n'
