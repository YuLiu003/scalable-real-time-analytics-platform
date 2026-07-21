#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

"${SCRIPT_DIR}/preflight.sh"

if ! kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  printf 'ERROR: cluster %s does not exist. Run `make -C platform/local bootstrap`.\n' "${CLUSTER_NAME}" >&2
  exit 1
fi

printf 'Verifying node readiness...\n'
kubectl --context "${KUBERNETES_CONTEXT}" wait \
  --for=condition=Ready node --all --timeout=180s

printf 'Verifying namespace boundaries...\n'
for namespace in platform-observability analytics-data analytics-apps; do
  kubectl --context "${KUBERNETES_CONTEXT}" get namespace "${namespace}" >/dev/null
done
kubectl --context "${KUBERNETES_CONTEXT}" get namespaces \
  platform-observability analytics-data analytics-apps --show-labels

printf 'Verifying monitoring release...\n'
helm status "${MONITORING_RELEASE}" \
  --kube-context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" >/dev/null

kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" \
  wait pod \
  --selector app.kubernetes.io/instance="${MONITORING_RELEASE}" \
  --for=condition=Ready \
  --timeout=300s

not_ready="$(kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" \
  get pods --no-headers | awk '$3 != "Running" && $3 != "Completed" {print}')"
if [[ -n "${not_ready}" ]]; then
  printf 'ERROR: monitoring contains pods that are not ready:\n%s\n' "${not_ready}" >&2
  exit 1
fi

printf 'Verifying monitoring APIs and services...\n'
kubectl --context "${KUBERNETES_CONTEXT}" get customresourcedefinition prometheuses.monitoring.coreos.com >/dev/null
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" \
  get service monitoring-grafana monitoring-kube-prometheus-prometheus >/dev/null

printf '\nLocal platform verification passed.\n'
kubectl --context "${KUBERNETES_CONTEXT}" get nodes -o wide
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" get pods
