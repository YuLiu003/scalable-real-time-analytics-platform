#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"
MARKET_DIR="${REPO_ROOT}/services/market-pipeline"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/capacity-observability-lib.sh"

context="${KUBERNETES_CONTEXT}"
namespace="${APPLICATION_NAMESPACE}"
suite_id="${CAPACITY_SUITE_ID:-c$(date -u +%j%H%M%S)}"
event_counts="${CAPACITY_EVENT_COUNTS:-10000,50000,100000}"
repetitions="${CAPACITY_REPETITIONS:-5}"
target_rate="${CAPACITY_TARGET_RATE:-0}"
phase_timeout_seconds="${CAPACITY_PHASE_TIMEOUT_SECONDS:-1800}"
inspection_timeout_seconds="${CAPACITY_INSPECTION_TIMEOUT_SECONDS:-300}"
sample_interval_seconds="${CAPACITY_SAMPLE_INTERVAL_SECONDS:-2}"
cpu_rate_window_seconds=25
resource_ingestion_timeout_seconds=30
prometheus_query_recovery_timeout_seconds=30
prometheus_request_timeout_cap_seconds=10
artifact_root="${CAPACITY_ARTIFACT_DIR:-${REPO_ROOT}/artifacts/kafka-capacity}"
reporter=""
go_cache="${CAPACITY_GOCACHE:-${TMPDIR:-/tmp}/market-capacity-gocache}"
active_job=""
runtime_changed=0
allocated_cpus="${CAPACITY_ALLOCATED_CPUS:-${EPHEMERAL_COLIMA_CPUS:-}}"
allocated_memory_gib="${CAPACITY_ALLOCATED_MEMORY_GIB:-${EPHEMERAL_COLIMA_MEMORY_GIB:-}}"
allocated_disk_gib="${CAPACITY_ALLOCATED_DISK_GIB:-${EPHEMERAL_COLIMA_DISK_GIB:-}}"

if [[ ! "${suite_id}" =~ ^[a-z0-9][a-z0-9-]{0,9}$ ]]; then
  printf 'ERROR: CAPACITY_SUITE_ID must contain 1-10 lowercase letters, digits, or hyphens.\n' >&2
  exit 2
fi
suite_dir="${artifact_root}/${suite_id}"
runs_dir="${suite_dir}/runs"
plan_path="${suite_dir}/plan.json"
run_matrix_path="${suite_dir}/runs.tsv"
environment_path="${suite_dir}/environment.json"
summary_path="${suite_dir}/summary.json"
failure_path="${suite_dir}/failure-state.txt"
stability_baseline_path="${suite_dir}/pod-stability-baseline.txt"

mkdir -p "${artifact_root}"
if [[ -e "${suite_dir}" ]]; then
  printf 'ERROR: benchmark suite %s already exists; choose a new CAPACITY_SUITE_ID.\n' "${suite_id}" >&2
  exit 2
fi

if [[ ! "${phase_timeout_seconds}" =~ ^[0-9]+$ ]] || (( phase_timeout_seconds < 120 || phase_timeout_seconds > 7200 )); then
  printf 'ERROR: CAPACITY_PHASE_TIMEOUT_SECONDS must be an integer between 120 and 7200.\n' >&2
  exit 2
fi
if [[ ! "${inspection_timeout_seconds}" =~ ^[0-9]+$ ]] || (( inspection_timeout_seconds < 30 || inspection_timeout_seconds > 1800 )); then
  printf 'ERROR: CAPACITY_INSPECTION_TIMEOUT_SECONDS must be an integer between 30 and 1800.\n' >&2
  exit 2
fi
if [[ ! "${sample_interval_seconds}" =~ ^[0-9]+$ ]] || (( sample_interval_seconds < 1 || sample_interval_seconds > 10 )); then
  printf 'ERROR: CAPACITY_SAMPLE_INTERVAL_SECONDS must be an integer between 1 and 10.\n' >&2
  exit 2
fi
for allocation in allocated_cpus allocated_memory_gib allocated_disk_gib; do
  if [[ ! "${!allocation}" =~ ^[1-9][0-9]*$ ]]; then
    printf 'ERROR: %s must be a positive integer.\n' "${allocation}" >&2
    exit 2
  fi
done

reporter="$(mktemp "${TMPDIR:-/tmp}/capacity-report.XXXXXX")"
planning_dir="$(mktemp -d "${TMPDIR:-/tmp}/capacity-plan.XXXXXX")"
staged_plan_path="${planning_dir}/plan.json"
staged_run_matrix_path="${planning_dir}/runs.tsv"
mkdir -p "${go_cache}"
if ! (
  cd "${MARKET_DIR}"
  GOWORK=off GOCACHE="${go_cache}" go build -o "${reporter}" ./cmd/capacity-report
) || ! "${reporter}" plan \
  --suite-id "${suite_id}" \
  --event-counts "${event_counts}" \
  --repetitions "${repetitions}" \
  --target-rate "${target_rate}" \
  --output "${staged_plan_path}" \
  --runs-output "${staged_run_matrix_path}"; then
  rm -f "${reporter}" "${staged_plan_path}" "${staged_run_matrix_path}"
  rmdir "${planning_dir}" 2>/dev/null || true
  exit 2
fi
if ! mkdir "${suite_dir}"; then
  printf 'ERROR: benchmark suite %s was reserved concurrently; choose a new CAPACITY_SUITE_ID.\n' "${suite_id}" >&2
  rm -f "${reporter}" "${staged_plan_path}" "${staged_run_matrix_path}"
  rmdir "${planning_dir}" 2>/dev/null || true
  exit 2
fi
mv "${staged_plan_path}" "${plan_path}"
mv "${staged_run_matrix_path}" "${run_matrix_path}"
rmdir "${planning_dir}"
mkdir -p "${runs_dir}"

restore_runtime() {
  local status=0
  if [[ -n "${active_job}" ]]; then
    kubectl --context "${context}" --namespace "${namespace}" \
      delete job "${active_job}" --ignore-not-found --wait=false >/dev/null 2>&1 || status=1
  fi
  if (( runtime_changed != 0 )); then
    kubectl --context "${context}" --namespace "${namespace}" \
      set env deployment/scale-event-archiver ARCHIVER_POST_WRITE_DELAY- >/dev/null 2>&1 || status=1
    kubectl --context "${context}" --namespace "${namespace}" \
      patch scaledobject/scale-event-archiver --type=merge \
      --patch '{"spec":{"minReplicaCount":1,"maxReplicaCount":3}}' >/dev/null 2>&1 || status=1
    kubectl --context "${context}" --namespace "${namespace}" \
      set env cronjob/scale-load-producer \
      LOAD_RUN_ID=local-scale LOAD_PHASE=original LOAD_EVENT_COUNT=1200 \
      LOAD_INSTRUMENTS=LOAD-A,LOAD-B,LOAD-C,LOAD-D,LOAD-E,LOAD-F,LOAD-G,LOAD-H,LOAD-I,LOAD-J,LOAD-K,LOAD-L \
      LOAD_BASE_TIME=2026-07-30T00:00:00Z LOAD_TARGET_RATE=0 >/dev/null 2>&1 || status=1
  fi
  return "${status}"
}

capture_failure() {
  {
    printf 'suite_id=%s\n' "${suite_id}"
    printf 'event_counts=%s\n' "${event_counts}"
    printf 'repetitions=%s\n' "${repetitions}"
    printf 'target_rate=%s\n' "${target_rate}"
    printf 'active_job=%s\n' "${active_job:-none}"
    kubectl --context "${context}" --request-timeout=10s --namespace "${namespace}" \
      get deployment/scale-event-archiver horizontalpodautoscaler/keda-hpa-scale-event-archiver \
      scaledobject/scale-event-archiver 2>/dev/null || printf 'scale controllers unavailable\n'
    kubectl --context "${context}" --request-timeout=10s --namespace "${namespace}" \
      get pods --selector app.kubernetes.io/name=scale-event-archiver 2>/dev/null || \
      printf 'scale pods unavailable\n'
    kubectl --context "${context}" --request-timeout=10s --namespace "${namespace}" \
      get jobs --selector app.kubernetes.io/name=scale-load-producer 2>/dev/null || \
      printf 'scale jobs unavailable\n'
    kubectl --context "${context}" --request-timeout=10s --namespace "${OBSERVABILITY_NAMESPACE}" \
      get statefulset/prometheus-monitoring-kube-prometheus-prometheus \
      --output=custom-columns='NAME:.metadata.name,READY:.status.readyReplicas,CURRENT:.status.currentReplicas,UPDATED:.status.updatedReplicas' \
      2>/dev/null || printf 'Prometheus StatefulSet unavailable\n'
    kubectl --context "${context}" --request-timeout=10s --namespace "${OBSERVABILITY_NAMESPACE}" \
      get endpoints/monitoring-kube-prometheus-prometheus \
      --output=jsonpath='ready={range .subsets[*].addresses[*]}x{end} not_ready={range .subsets[*].notReadyAddresses[*]}x{end}{"\n"}' \
      2>/dev/null || printf 'Prometheus endpoints unavailable\n'
    kubectl --context "${context}" --request-timeout=10s get pods --all-namespaces \
      --output=custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,PHASE:.status.phase' \
      2>/dev/null || printf 'cluster pods unavailable\n'
  } >"${failure_path}"
}

cleanup() {
  local original_status=$?
  local cleanup_status=0
  trap - EXIT HUP INT TERM
  if (( original_status != 0 )); then
    capture_failure || true
  fi
  restore_runtime || cleanup_status=1
  rm -f "${reporter}"
  if (( original_status == 0 && cleanup_status != 0 )); then
    original_status=${cleanup_status}
  fi
  exit "${original_status}"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

now_milliseconds() {
  python3 -c 'import time; print(time.time_ns() // 1_000_000)'
}

prometheus_query_to_file() {
  local query="$1"
  local required_label="$2"
  local output_file="$3"
  capacity_prometheus_query_to_file \
    "${context}" "${OBSERVABILITY_NAMESPACE}" "${query}" "${required_label}" "${output_file}" \
    "${prometheus_query_recovery_timeout_seconds}" "${sample_interval_seconds}" \
    "${prometheus_request_timeout_cap_seconds}"
}

prometheus_range_to_file() {
  local query="$1"
  local start_seconds="$2"
  local end_seconds="$3"
  local output_file="$4"
  local request_timeout_seconds="$5"
  local parameters
  parameters="$(python3 -c 'import sys, urllib.parse; print(urllib.parse.urlencode({"query": sys.argv[1], "start": sys.argv[2], "end": sys.argv[3], "step": "5"}))' \
    "${query}" "${start_seconds}" "${end_seconds}")"
  kubectl --context "${context}" --request-timeout="${request_timeout_seconds}s" get --raw \
    "/api/v1/namespaces/${OBSERVABILITY_NAMESPACE}/services/http:monitoring-kube-prometheus-prometheus:9090/proxy/api/v1/query_range?${parameters}" \
    >"${output_file}"
}

job_complete() {
  [[ "$(kubectl --context "${context}" --namespace "${namespace}" \
    get job "$1" --output=jsonpath='{.status.conditions[?(@.type=="Complete")].status}')" == "True" ]]
}

job_failed() {
  [[ "$(kubectl --context "${context}" --namespace "${namespace}" \
    get job "$1" --output=jsonpath='{.status.conditions[?(@.type=="Failed")].status}')" == "True" ]]
}

lag_snapshot() {
  kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
    /topic-inspector \
    --topic market.prices.scale \
    --group scale-event-archiver-v1 \
    --lag \
    --expected-partitions 3 \
    --output json
}

topic_offsets_to_file() {
  local output_file="$1"
  kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
    /topic-inspector --topic market.prices.scale --offsets --output json >"${output_file}"
}

total_lag() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)["total_lag"])'
}

require_no_unfinished_producers() {
  local states name complete failed unfinished=""
  states="$(kubectl --context "${context}" --namespace "${namespace}" get jobs \
    --selector app.kubernetes.io/name=scale-load-producer \
    --output=jsonpath='{range .items[*]}{.metadata.name}{"|"}{.status.conditions[?(@.type=="Complete")].status}{"|"}{.status.conditions[?(@.type=="Failed")].status}{"\n"}{end}')"
  while IFS='|' read -r name complete failed; do
    if [[ -n "${name}" && "${complete}" != "True" && "${failed}" != "True" ]]; then
      unfinished="${unfinished}${unfinished:+, }${name}"
    fi
  done <<<"${states}"
  if [[ -n "${unfinished}" ]]; then
    printf 'ERROR: unfinished scale producer jobs prevent an isolated benchmark: %s.\n' "${unfinished}" >&2
    return 1
  fi
}

create_load_job() {
  local job_name="$1"
  local count="$2"
  local rate="$3"
  kubectl --context "${context}" --namespace "${namespace}" \
    set env cronjob/scale-load-producer \
    "LOAD_RUN_ID=${job_name}" LOAD_PHASE=original "LOAD_EVENT_COUNT=${count}" \
    LOAD_INSTRUMENTS=LOAD-A,LOAD-B,LOAD-C,LOAD-D,LOAD-E,LOAD-F,LOAD-G,LOAD-H,LOAD-I,LOAD-J,LOAD-K,LOAD-L \
    LOAD_BASE_TIME=2026-07-30T00:00:00Z "LOAD_TARGET_RATE=${rate}" >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    create job --from=cronjob/scale-load-producer "${job_name}" >/dev/null
  active_job="${job_name}"
}

wait_for_fixed_baseline() {
  local deadline available lag_json lag
  deadline=$((SECONDS + phase_timeout_seconds))
  while (( SECONDS < deadline )); do
    available="$(kubectl --context "${context}" --namespace "${namespace}" \
      get deployment/scale-event-archiver --output=jsonpath='{.status.availableReplicas}')"
    if lag_json="$(lag_snapshot 2>/dev/null)"; then
      lag="$(total_lag <<<"${lag_json}")"
      if [[ "${available:-0}" == "3" ]] && (( lag == 0 )); then
        return 0
      fi
    fi
    sleep 2
  done
  printf 'ERROR: fixed three-consumer benchmark baseline did not become ready with zero lag.\n' >&2
  return 1
}

wait_for_warmup() {
  local job_name="$1"
  local deadline lag_json lag
  deadline=$((SECONDS + phase_timeout_seconds))
  while (( SECONDS < deadline )); do
    if job_failed "${job_name}"; then
      kubectl --context "${context}" --namespace "${namespace}" logs "job/${job_name}" >&2 || true
      return 1
    fi
    if job_complete "${job_name}" && lag_json="$(lag_snapshot 2>/dev/null)"; then
      lag="$(total_lag <<<"${lag_json}")"
      if (( lag == 0 )); then
        return 0
      fi
    fi
    sleep 2
  done
  return 1
}

capture_resource_ranges() {
  local raw_dir="$1"
  local start_seconds="$2"
  local end_seconds="$3"
  local deadline="$4"
  local cpu_start_seconds component resource_namespace pod_pattern cpu_query memory_query memory_selector remaining
  cpu_start_seconds="$(python3 -c 'import sys; print(float(sys.argv[1]) + int(sys.argv[2]))' \
    "${start_seconds}" "${cpu_rate_window_seconds}")"
  while IFS='|' read -r component resource_namespace pod_pattern; do
    cpu_query="sum(rate(container_cpu_usage_seconds_total{namespace=\"${resource_namespace}\",pod=~\"${pod_pattern}\",container!=\"\",container!=\"POD\"}[${cpu_rate_window_seconds}s]))"
    memory_selector="container_memory_working_set_bytes{namespace=\"${resource_namespace}\",pod=~\"${pod_pattern}\",container!=\"\",container!=\"POD\"}"
    memory_query="sum(${memory_selector} and (timestamp(${memory_selector}) >= ${start_seconds}))"
    remaining=$((deadline - SECONDS))
    (( remaining > 0 )) || return 1
    prometheus_range_to_file "${cpu_query}" "${cpu_start_seconds}" "${end_seconds}" \
      "${raw_dir}/resource-${component}-cpu.json" "${remaining}" || return 1
    remaining=$((deadline - SECONDS))
    (( remaining > 0 )) || return 1
    prometheus_range_to_file "${memory_query}" "${start_seconds}" "${end_seconds}" \
      "${raw_dir}/resource-${component}-memory.json" "${remaining}" || return 1
  done <<EOF
archiver|${namespace}|scale-event-archiver-.*
kafka|${DATA_NAMESPACE}|market-kafka-.*
object_store|${DATA_NAMESPACE}|garage-.*
EOF
}

resource_ranges_available() {
  local raw_dir="$1"
  python3 - "${raw_dir}" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for component in ("archiver", "kafka", "object_store"):
    for resource in ("cpu", "memory"):
        try:
            response = json.loads(
                (root / f"resource-{component}-{resource}.json").read_text()
            )
            series = response["data"]["result"]
        except (KeyError, OSError, json.JSONDecodeError):
            raise SystemExit(1)
        if (
            response.get("status") != "success"
            or not isinstance(series, list)
            or not any(isinstance(item, dict) and item.get("values") for item in series)
        ):
            raise SystemExit(1)
PY
}

wait_for_resource_ranges() {
  local raw_dir="$1"
  local start_seconds="$2"
  local end_seconds="$3"
  local deadline=$((SECONDS + resource_ingestion_timeout_seconds))
  local remaining sleep_seconds
  if ! capacity_duration_supports_window \
    "${start_seconds}" "${end_seconds}" "${cpu_rate_window_seconds}"; then
    printf 'ERROR: durable trial duration is too short for an in-boundary %d-second CPU rate sample.\n' \
      "${cpu_rate_window_seconds}" >&2
    return 1
  fi
  while true; do
    if capture_resource_ranges "${raw_dir}" "${start_seconds}" "${end_seconds}" "${deadline}" && \
      resource_ranges_available "${raw_dir}"; then
      return 0
    fi
    if (( SECONDS >= deadline )); then
      printf 'ERROR: resource measurements were not ingested within %d seconds.\n' \
        "${resource_ingestion_timeout_seconds}" >&2
      return 1
    fi
    remaining=$((deadline - SECONDS))
    sleep_seconds="${sample_interval_seconds}"
    if (( sleep_seconds > remaining )); then
      sleep_seconds="${remaining}"
    fi
    sleep "${sleep_seconds}"
  done
}

run_capacity_trial() {
  local run_id="$1"
  local event_count="$2"
  local repetition="$3"
  local rate="$4"
  local resource_measurements_required="$5"
  local run_dir="${runs_dir}/${run_id}"
  local raw_dir="${run_dir}/raw"
  local started_ms completed_ms duration_ms start_seconds end_seconds deadline
  local lag_json lag available producer_complete=false archive_ready=false
  local start_offsets end_offsets
  mkdir -p "${raw_dir}"
  : >"${raw_dir}/samples.jsonl"

  if [[ "${resource_measurements_required}" != "true" && "${resource_measurements_required}" != "false" ]]; then
    printf 'ERROR: capacity plan has an invalid resource policy for %s.\n' "${run_id}" >&2
    return 1
  fi
  printf 'Running capacity trial %s: events=%s repetition=%s target_rate=%s resources_required=%s...\n' \
    "${run_id}" "${event_count}" "${repetition}" "${rate}" "${resource_measurements_required}"
  prometheus_query_to_file \
    'sum by (le) (market_archiver_durable_latency_seconds_bucket{scope="scale"})' \
    le \
    "${raw_dir}/latency-before.json"
  prometheus_query_to_file \
    'sum by (outcome) (market_archiver_events_total{scope="scale"})' \
    outcome \
    "${raw_dir}/outcomes-before.json"
  topic_offsets_to_file "${raw_dir}/topic-offsets-before.json"

  started_ms="$(now_milliseconds)"
  start_seconds="$(python3 -c 'import sys; print(int(sys.argv[1]) / 1000)' "${started_ms}")"
  create_load_job "${run_id}" "${event_count}" "${rate}"
  deadline=$((SECONDS + phase_timeout_seconds))
  while (( SECONDS < deadline )); do
    if job_failed "${run_id}"; then
      kubectl --context "${context}" --namespace "${namespace}" logs "job/${run_id}" >&2 || true
      printf 'ERROR: capacity producer %s failed.\n' "${run_id}" >&2
      return 1
    fi
    if job_complete "${run_id}"; then
      producer_complete=true
    fi
    if ! lag_json="$(lag_snapshot 2>/dev/null)"; then
      sleep "${sample_interval_seconds}"
      continue
    fi
    lag="$(total_lag <<<"${lag_json}")"
    available="$(kubectl --context "${context}" --namespace "${namespace}" \
      get deployment/scale-event-archiver --output=jsonpath='{.status.availableReplicas}')"
    printf '{"elapsed_milliseconds":%d,"lag":%s,"available_replicas":%d,"producer_complete":%s}\n' \
      "$(( $(now_milliseconds) - started_ms ))" "${lag_json}" "${available:-0}" "${producer_complete}" \
      >>"${raw_dir}/samples.jsonl"
    if [[ "${producer_complete}" == "true" ]] && (( lag == 0 )); then
      if kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
        /archive-inspector \
        --prefix "bronze/market.price.observed/v1/date=2026-07-30/source=scale.${run_id}/" \
        --expected "${event_count}" >"${raw_dir}/archive.json" 2>/dev/null; then
        archive_ready=true
        completed_ms="$(now_milliseconds)"
        break
      fi
    fi
    sleep "${sample_interval_seconds}"
  done
  if [[ "${archive_ready}" != "true" ]]; then
    printf 'ERROR: capacity trial %s did not reach durable completion within %d seconds.\n' \
      "${run_id}" "${phase_timeout_seconds}" >&2
    return 1
  fi
  duration_ms=$((completed_ms - started_ms))
  end_seconds="$(python3 -c 'import sys; print(int(sys.argv[1]) / 1000)' "${completed_ms}")"
  topic_offsets_to_file "${raw_dir}/topic-offsets-after.json"
  start_offsets="$(<"${raw_dir}/topic-offsets-before.json")"
  end_offsets="$(<"${raw_dir}/topic-offsets-after.json")"

  kubectl --context "${context}" --namespace "${namespace}" logs "job/${run_id}" \
    >"${raw_dir}/producer.jsonl"
  kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
    /topic-inspector \
    --topic market.prices.scale \
    --run-id "${run_id}" \
    --phase original \
    --expected "${event_count}" \
    --start-offsets "${start_offsets}" \
    --end-offsets "${end_offsets}" \
    --output json \
    --timeout "${inspection_timeout_seconds}s" >"${raw_dir}/ordering.json"

  sleep 6
  prometheus_query_to_file \
    'sum by (le) (market_archiver_durable_latency_seconds_bucket{scope="scale"})' \
    le \
    "${raw_dir}/latency-after.json"
  prometheus_query_to_file \
    'sum by (outcome) (market_archiver_events_total{scope="scale"})' \
    outcome \
    "${raw_dir}/outcomes-after.json"
  if [[ "${resource_measurements_required}" == "true" ]]; then
    wait_for_resource_ranges "${raw_dir}" "${start_seconds}" "${end_seconds}"
  fi

  "${reporter}" run \
    --plan "${plan_path}" \
    --run-id "${run_id}" \
    --raw-dir "${raw_dir}" \
    --duration-ms "${duration_ms}" \
    --partitions 3 \
    --workers 3 \
    --output "${run_dir}/report.json"

  kubectl --context "${context}" --namespace "${namespace}" \
    delete job "${run_id}" --wait=true >/dev/null
  active_job=""
  capacity_capture_pod_stability "${context}" "${raw_dir}/pod-stability-after.txt"
  capacity_assert_pod_stability "${stability_baseline_path}" "${raw_dir}/pod-stability-after.txt"
}

printf 'Preparing fixed-worker Kafka capacity benchmark %s...\n' "${suite_id}"
if ! kubectl config get-contexts "${context}" >/dev/null 2>&1; then
  printf 'ERROR: Kubernetes context %s does not exist. Use `make -C platform/local e2e-capacity-ephemeral`.\n' "${context}" >&2
  exit 1
fi
helm status "${KEDA_RELEASE}" --kube-context "${context}" --namespace "${KEDA_NAMESPACE}" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  wait scaledobject/scale-event-archiver --for=condition=Ready --timeout=120s
require_no_unfinished_producers

runtime_changed=1
kubectl --context "${context}" --namespace "${namespace}" \
  set env deployment/scale-event-archiver ARCHIVER_POST_WRITE_DELAY- >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  patch scaledobject/scale-event-archiver --type=merge \
  --patch '{"spec":{"minReplicaCount":3,"maxReplicaCount":3}}' >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/scale-event-archiver --timeout=120s >/dev/null
wait_for_fixed_baseline

printf 'Warming the broker-to-archive path with 600 unmeasured fictional events...\n'
warmup_job="${suite_id}-warm"
create_load_job "${warmup_job}" 600 0
if ! wait_for_warmup "${warmup_job}"; then
  printf 'ERROR: capacity warm-up did not complete and drain.\n' >&2
  exit 1
fi
kubectl --context "${context}" --namespace "${namespace}" delete job "${warmup_job}" --wait=true >/dev/null
active_job=""
sleep 6
capacity_capture_pod_stability "${context}" "${stability_baseline_path}"

git_revision="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
tracked_tree_clean=true
if ! git -C "${REPO_ROOT}" diff --quiet || ! git -C "${REPO_ROOT}" diff --cached --quiet; then
  tracked_tree_clean=false
fi
architecture="$(uname -m)"
node_count="$(kubectl --context "${context}" get nodes --no-headers | wc -l | tr -d ' ')"
printf '{\n  "git_revision": "%s",\n  "tracked_tree_clean": %s,\n  "architecture": "%s",\n  "allocated_cpus": %s,\n  "allocated_memory_gib": %s,\n  "allocated_disk_gib": %s,\n  "kubernetes_nodes": %s,\n  "versions": {\n    "go": "%s",\n    "kubernetes": "%s",\n    "kind": "%s",\n    "kafka": "%s",\n    "strimzi": "%s",\n    "keda": "%s",\n    "garage": "%s"\n  }\n}\n' \
  "${git_revision}" "${tracked_tree_clean}" "${architecture}" \
  "${allocated_cpus}" "${allocated_memory_gib}" "${allocated_disk_gib}" "${node_count}" \
  "${GO_VERSION}" "${KUBERNETES_VERSION}" "${KIND_VERSION}" "${KAFKA_VERSION}" \
  "${STRIMZI_OPERATOR_VERSION}" "${KEDA_VERSION}" "${GARAGE_VERSION}" >"${environment_path}"

while IFS=$'\t' read -r run_id event_count repetition rate resource_measurements_required; do
  run_capacity_trial \
    "${run_id}" "${event_count}" "${repetition}" "${rate}" "${resource_measurements_required}"
done <"${run_matrix_path}"

"${reporter}" summary \
  --plan "${plan_path}" \
  --environment "${environment_path}" \
  --runs-dir "${runs_dir}" \
  --output "${summary_path}"

restore_runtime
runtime_changed=0
trap - EXIT HUP INT TERM
rm -f "${reporter}"
printf 'Kafka capacity benchmark passed. Summary: %s\n' "${summary_path}"
