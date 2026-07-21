#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

"${SCRIPT_DIR}/preflight.sh"

if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  printf 'Cluster %s already exists; reusing it.\n' "${CLUSTER_NAME}"
else
  printf 'Creating kind cluster %s with %s...\n' "${CLUSTER_NAME}" "${KIND_NODE_IMAGE}"
  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${LOCAL_DIR}/kind/cluster.yaml" \
    --image "${KIND_NODE_IMAGE}" \
    --wait 180s
fi

kubectl config use-context "${KUBERNETES_CONTEXT}" >/dev/null

printf 'Applying namespace ownership and resource guardrails...\n'
kubectl --context "${KUBERNETES_CONTEXT}" apply \
  -k "${REPO_ROOT}/platform/gitops/clusters/local"

grafana_password="${GRAFANA_ADMIN_PASSWORD:-$(openssl rand -hex 16)}"
kubectl --context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" \
  create secret generic grafana-admin-credentials \
  --from-literal=admin-user=admin \
  --from-literal="admin-password=${grafana_password}" \
  --dry-run=client \
  --output=yaml | kubectl --context "${KUBERNETES_CONTEXT}" apply -f -

printf 'Installing kube-prometheus-stack %s...\n' "${KUBE_PROMETHEUS_STACK_VERSION}"
helm repo add prometheus-community \
  https://prometheus-community.github.io/helm-charts \
  --force-update
helm repo update prometheus-community
helm upgrade --install "${MONITORING_RELEASE}" \
  prometheus-community/kube-prometheus-stack \
  --version "${KUBE_PROMETHEUS_STACK_VERSION}" \
  --kube-context "${KUBERNETES_CONTEXT}" \
  --namespace "${OBSERVABILITY_NAMESPACE}" \
  --values "${LOCAL_DIR}/addons/monitoring/values.yaml" \
  --wait \
  --timeout 10m

"${SCRIPT_DIR}/verify.sh"

printf '\nLocal platform baseline is ready.\n'
printf 'Grafana user: admin\n'
if [[ -z "${GRAFANA_ADMIN_PASSWORD:-}" ]]; then
  printf 'Generated Grafana password: %s\n' "${grafana_password}"
else
  printf 'Grafana password came from GRAFANA_ADMIN_PASSWORD.\n'
fi
printf 'Run `make -C platform/local grafana` or `make -C platform/local prometheus` to access monitoring.\n'
