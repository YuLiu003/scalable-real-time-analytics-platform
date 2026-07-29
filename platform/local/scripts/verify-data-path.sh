#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

"${SCRIPT_DIR}/verify.sh"

namespace="${APPLICATION_NAMESPACE}"
context="${KUBERNETES_CONTEXT}"

printf 'Verifying Strimzi, Kafka, topics, Garage, and application identities...\n'
helm status "${STRIMZI_RELEASE}" --kube-context "${context}" --namespace "${DATA_NAMESPACE}" >/dev/null
kubectl --context "${context}" --namespace "${DATA_NAMESPACE}" \
  wait kafka/"${KAFKA_CLUSTER_NAME}" --for=condition=Ready --timeout=120s
kubectl --context "${context}" --namespace "${DATA_NAMESPACE}" \
  wait kafkatopic/market-prices kafkatopic/ingestion-quarantine --for=condition=Ready --timeout=120s
kubectl --context "${context}" --namespace "${DATA_NAMESPACE}" \
  rollout status statefulset/garage --timeout=120s
kubectl --context "${context}" --namespace "${namespace}" \
  wait kafkauser/synthetic-market-producer kafkauser/raw-event-archiver --for=condition=Ready --timeout=120s
kubectl --context "${context}" --namespace "${namespace}" \
  get configmap "${KAFKA_CLUSTER_NAME}-cluster-ca" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/raw-event-archiver --timeout=120s
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/synthetic-market-producer-baseline --for=condition=Complete --timeout=120s

baseline_acknowledgements="$(kubectl --context "${context}" --namespace "${namespace}" \
  logs job/synthetic-market-producer-baseline | grep -c 'broker acknowledged event')"
if [[ "${baseline_acknowledgements}" != "4" ]]; then
  printf 'ERROR: baseline producer has %s broker acknowledgements; expected 4.\n' "${baseline_acknowledgements}" >&2
  exit 1
fi

job_is_suspended() {
  [[ "$(kubectl --context "${context}" --namespace "${namespace}" get job "$1" --output=jsonpath='{.spec.suspend}')" == "true" ]]
}

wait_for_log() {
  local pod_name="$1"
  local pattern="$2"
  local attempts=45
  while (( attempts > 0 )); do
    if kubectl --context "${context}" --namespace "${namespace}" logs "${pod_name}" 2>/dev/null | grep -Fq "${pattern}"; then
      return 0
    fi
    attempts=$((attempts - 1))
    sleep 2
  done
  printf 'ERROR: pod %s did not log expected marker %s.\n' "${pod_name}" "${pattern}" >&2
  return 1
}

restore_archiver() {
  kubectl --context "${context}" --namespace "${namespace}" \
    set env deployment/raw-event-archiver ARCHIVER_POST_WRITE_DELAY- >/dev/null 2>&1 || true
}

if job_is_suspended synthetic-market-producer-ack-unknown; then
  printf 'Testing producer crash after broker acknowledgement...\n'
  kubectl --context "${context}" --namespace "${namespace}" patch \
    job synthetic-market-producer-ack-unknown --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    wait job/synthetic-market-producer-ack-unknown --for=condition=Failed --timeout=120s
  if ! kubectl --context "${context}" --namespace "${namespace}" \
    logs job/synthetic-market-producer-ack-unknown | grep -Fq 'broker acknowledged event'; then
    printf 'ERROR: injected producer failure occurred before broker acknowledgement.\n' >&2
    exit 1
  fi

  kubectl --context "${context}" --namespace "${namespace}" patch \
    job synthetic-market-producer-ack-retry --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    wait job/synthetic-market-producer-ack-retry --for=condition=Complete --timeout=120s
fi

if job_is_suspended synthetic-market-producer-consumer-crash; then
  printf 'Testing consumer crash after S3 write and before offset marking...\n'
  trap restore_archiver EXIT
  kubectl --context "${context}" --namespace "${namespace}" \
    set env deployment/raw-event-archiver ARCHIVER_POST_WRITE_DELAY=30s >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    rollout status deployment/raw-event-archiver --timeout=120s >/dev/null
  crash_pod="$(kubectl --context "${context}" --namespace "${namespace}" get pod \
    -l app.kubernetes.io/name=raw-event-archiver --output=jsonpath='{.items[0].metadata.name}')"

  kubectl --context "${context}" --namespace "${namespace}" patch \
    job synthetic-market-producer-consumer-crash --type=merge --patch '{"spec":{"suspend":false}}' >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    wait job/synthetic-market-producer-consumer-crash --for=condition=Complete --timeout=120s
  wait_for_log "${crash_pod}" \
    '"msg":"post-write failure window open","event_id":"synthetic:price:sp500:consumer-crash-001"'
  kubectl --context "${context}" --namespace "${namespace}" delete pod "${crash_pod}" --wait=false >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    rollout status deployment/raw-event-archiver --timeout=120s >/dev/null
  recovered_pod="$(kubectl --context "${context}" --namespace "${namespace}" get pod \
    -l app.kubernetes.io/name=raw-event-archiver --output=jsonpath='{.items[0].metadata.name}')"
  wait_for_log "${recovered_pod}" \
    '"msg":"archive effect durable","event_id":"synthetic:price:sp500:consumer-crash-001","result":"duplicate"'
  printf 'Observed consumer redelivery as an idempotent duplicate after the injected crash.\n'
  restore_archiver
  trap - EXIT
  kubectl --context "${context}" --namespace "${namespace}" \
    rollout status deployment/raw-event-archiver --timeout=120s >/dev/null
fi

kubectl --context "${context}" --namespace "${namespace}" \
  wait job/synthetic-market-producer-ack-unknown --for=condition=Failed --timeout=30s
kubectl --context "${context}" --namespace "${namespace}" \
  wait job/synthetic-market-producer-ack-retry job/synthetic-market-producer-consumer-crash \
  --for=condition=Complete --timeout=30s

printf 'Verifying broker records, quarantine outcome, and idempotent archive objects...\n'
kubectl --context "${context}" --namespace "${namespace}" exec deployment/raw-event-archiver -- \
  /topic-inspector --topic market.prices --expected 7 --timeout 20s
kubectl --context "${context}" --namespace "${namespace}" exec deployment/raw-event-archiver -- \
  /topic-inspector --topic ingestion.quarantine --expected 1 --timeout 20s
kubectl --context "${context}" --namespace "${namespace}" exec deployment/raw-event-archiver -- \
  /archive-inspector --prefix bronze/ --expected 4

printf '\nSlice 2 producer-to-storage verification passed.\n'
kubectl --context "${context}" --namespace "${DATA_NAMESPACE}" get kafka,kafkanodepool,kafkatopic
kubectl --context "${context}" --namespace "${namespace}" get kafkauser,deployment,job
