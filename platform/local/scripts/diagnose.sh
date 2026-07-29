#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
output_dir="${TMPDIR:-/tmp}/investment-platform-diagnostics-${timestamp}"
mkdir -p "${output_dir}"

capture() {
  local output_file="$1"
  shift
  "$@" >"${output_dir}/${output_file}" 2>&1 || true
}

capture kind-version.txt kind version
capture kind-clusters.txt kind get clusters
capture kubectl-version.txt kubectl version --client=true
capture cluster-info.txt kubectl --context "${KUBERNETES_CONTEXT}" cluster-info
capture nodes.txt kubectl --context "${KUBERNETES_CONTEXT}" get nodes -o wide
capture namespaces.txt kubectl --context "${KUBERNETES_CONTEXT}" get namespaces --show-labels
capture workloads.txt kubectl --context "${KUBERNETES_CONTEXT}" get all --all-namespaces -o wide
capture persistent-volumes.txt kubectl --context "${KUBERNETES_CONTEXT}" get persistentvolumes,persistentvolumeclaims --all-namespaces
capture recent-events.txt kubectl --context "${KUBERNETES_CONTEXT}" get events --all-namespaces --sort-by=.lastTimestamp
capture helm-releases.txt helm list --all-namespaces --kube-context "${KUBERNETES_CONTEXT}"
capture monitoring-status.txt helm status "${MONITORING_RELEASE}" --namespace "${OBSERVABILITY_NAMESPACE}" --kube-context "${KUBERNETES_CONTEXT}"

if kind get clusters 2>/dev/null | grep -Fxq "${CLUSTER_NAME}"; then
  kind export logs "${output_dir}/kind" --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
fi

printf 'Diagnostics written to %s\n' "${output_dir}"
printf 'Secret objects and secret values were intentionally excluded.\n'
