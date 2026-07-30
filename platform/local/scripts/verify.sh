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

assert_pod_security_enforcement() {
  local namespace="$1"
  local expected="$2"
  local actual

  actual="$(kubectl --context "${KUBERNETES_CONTEXT}" get namespace "${namespace}" \
    --output=jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}')"
  if [[ "${actual}" != "${expected}" ]]; then
    printf 'ERROR: namespace %s enforces Pod Security %s; expected %s.\n' \
      "${namespace}" "${actual:-unset}" "${expected}" >&2
    exit 1
  fi
}

assert_pod_security_enforcement platform-observability privileged
assert_pod_security_enforcement analytics-data baseline
assert_pod_security_enforcement analytics-apps restricted

kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace analytics-apps get limitrange default-container-resources >/dev/null
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace analytics-data get limitrange default-container-resources >/dev/null
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace platform-observability get limitrange default-container-resources >/dev/null
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace analytics-apps get resourcequota local-application-budget >/dev/null
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace analytics-data get resourcequota local-data-budget >/dev/null

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

wait_for_monitoring_api() {
  local name="$1"
  local path="$2"
  local expected="$3"
  local response

  for _ in {1..60}; do
    if response="$(kubectl --context "${KUBERNETES_CONTEXT}" get --raw "${path}" 2>/dev/null)" &&
      grep -Eq "${expected}" <<<"${response}"; then
      return
    fi
    sleep 2
  done
  printf 'ERROR: %s did not become ready within 120 seconds.\n' "${name}" >&2
  return 1
}

wait_for_monitoring_api "Grafana health endpoint" \
  "/api/v1/namespaces/${OBSERVABILITY_NAMESPACE}/services/http:monitoring-grafana:80/proxy/api/health" \
  '"database"[[:space:]]*:[[:space:]]*"ok"'
wait_for_monitoring_api "Prometheus readiness endpoint" \
  "/api/v1/namespaces/${OBSERVABILITY_NAMESPACE}/services/http:monitoring-kube-prometheus-prometheus:9090/proxy/-/ready" \
  "Prometheus Server is Ready\\."

printf '\nLocal platform verification passed.\n'
kubectl --context "${KUBERNETES_CONTEXT}" get nodes -o wide
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" get pods
