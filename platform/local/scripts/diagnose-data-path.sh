#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
output_dir="${TMPDIR:-/tmp}/market-data-path-diagnostics-${timestamp}"
mkdir -p "${output_dir}"

capture() {
  local output_file="$1"
  shift
  "$@" >"${output_dir}/${output_file}" 2>&1 || true
}

capture helm-status.txt helm status "${STRIMZI_RELEASE}" --kube-context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}"
capture data-resources.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" get kafka,kafkanodepool,kafkatopic,statefulset,pod,persistentvolumeclaim -o wide
capture app-resources.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" get kafkauser,deployment,job,pod -o wide
capture data-events.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" get events --sort-by=.lastTimestamp
capture app-events.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" get events --sort-by=.lastTimestamp
capture kafka-status.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" describe kafka "${KAFKA_CLUSTER_NAME}"
capture garage-status.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" exec garage-0 -- /garage status
capture archiver-logs.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" logs deployment/raw-event-archiver
capture archive-keys.txt kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" exec deployment/raw-event-archiver -- /archive-inspector --prefix bronze/

printf 'Data-path diagnostics written to %s\n' "${output_dir}"
printf 'Secret objects, Secret values, and archived object contents were intentionally excluded.\n'
