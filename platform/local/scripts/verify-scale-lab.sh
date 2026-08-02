#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

context="${KUBERNETES_CONTEXT}"
namespace="${APPLICATION_NAMESPACE}"
event_count="${SCALE_EVENT_COUNT:-1200}"
archiver_delay_ms="${SCALE_ARCHIVER_DELAY_MS:-100}"
phase_timeout_seconds="${SCALE_PHASE_TIMEOUT_SECONDS:-240}"
run_id="${SCALE_RUN_ID:-scale-$(date -u +%Y%m%dt%H%M%S)}"
artifact_dir="${SCALE_ARTIFACT_DIR:-${REPO_ROOT}/artifacts/kafka-scale}"
report_path="${artifact_dir}/report.json"
samples_path="${artifact_dir}/scale-samples.jsonl"
failure_path="${artifact_dir}/failure-state.txt"
recovery_path="${artifact_dir}/consumer-group-recovery.json"
recovery_candidate_path="${artifact_dir}/consumer-group-recovery.candidate.json"
original_job="scale-original-${run_id}"
replay_job="scale-replay-${run_id}"

clear_evidence() {
  mkdir -p "${artifact_dir}"
  rm -f \
    "${report_path}" \
    "${samples_path}" \
    "${failure_path}" \
    "${recovery_path}" \
    "${recovery_candidate_path}" \
    "${artifact_dir}/ordering-original.json" \
    "${artifact_dir}/ordering-replay.json" \
    "${artifact_dir}/outcomes-before-replay.json" \
    "${artifact_dir}/outcomes-after-replay.json" \
    "${artifact_dir}/producer-original.jsonl" \
    "${artifact_dir}/producer-replay.jsonl" \
    "${artifact_dir}/prometheus-p95.json" \
    "${artifact_dir}/replay-outcomes.json"
}

clear_evidence

if [[ ! "${event_count}" =~ ^[0-9]+$ ]] || (( event_count < 600 || event_count > 100000 )); then
  printf 'ERROR: SCALE_EVENT_COUNT must be an integer between 600 and 100000.\n' >&2
  exit 2
fi
if [[ ! "${archiver_delay_ms}" =~ ^[0-9]+$ ]] || (( archiver_delay_ms < 1 || archiver_delay_ms > 100 )); then
  printf 'ERROR: SCALE_ARCHIVER_DELAY_MS must be an integer between 1 and 100.\n' >&2
  exit 2
fi
minimum_partition_backlog_ms=$(((event_count / 12) * 4 * archiver_delay_ms))
if (( minimum_partition_backlog_ms < 35000 )); then
  printf 'ERROR: SCALE_EVENT_COUNT and SCALE_ARCHIVER_DELAY_MS must retain at least 35 seconds of injected work per partition so HPA can observe lag.\n' >&2
  exit 2
fi
if [[ ! "${phase_timeout_seconds}" =~ ^[0-9]+$ ]] || (( phase_timeout_seconds < 120 || phase_timeout_seconds > 3600 )); then
  printf 'ERROR: SCALE_PHASE_TIMEOUT_SECONDS must be an integer between 120 and 3600.\n' >&2
  exit 2
fi
if [[ ! "${run_id}" =~ ^[a-z0-9][a-z0-9-]{0,25}$ ]]; then
  printf 'ERROR: SCALE_RUN_ID must be 1-26 lowercase letters, digits, or hyphens.\n' >&2
  exit 2
fi

: >"${samples_path}"

restore_runtime() {
  kubectl --context "${context}" --namespace "${namespace}" \
    set env deployment/scale-event-archiver ARCHIVER_POST_WRITE_DELAY- >/dev/null 2>&1 || true
  kubectl --context "${context}" --namespace "${namespace}" \
    patch scaledobject/scale-event-archiver --type=merge \
    --patch '{"spec":{"minReplicaCount":1,"maxReplicaCount":3}}' >/dev/null 2>&1 || true
  kubectl --context "${context}" --namespace "${namespace}" \
    set env cronjob/scale-load-producer \
    LOAD_RUN_ID=local-scale LOAD_PHASE=original LOAD_EVENT_COUNT=1200 \
    LOAD_INSTRUMENTS=LOAD-A,LOAD-B,LOAD-C,LOAD-D,LOAD-E,LOAD-F,LOAD-G,LOAD-H,LOAD-I,LOAD-J,LOAD-K,LOAD-L \
    LOAD_BASE_TIME=2026-07-30T00:00:00Z >/dev/null 2>&1 || true
}

capture_failure() {
  {
    printf 'run_id=%s\n' "${run_id}"
    printf 'event_count=%s\n' "${event_count}"
    printf 'archiver_delay_ms=%s\n' "${archiver_delay_ms}"
    printf 'phase_timeout_seconds=%s\n' "${phase_timeout_seconds}"
    kubectl --context "${context}" --namespace "${namespace}" \
      get deployment/scale-event-archiver \
      horizontalpodautoscaler/keda-hpa-scale-event-archiver \
      scaledobject/scale-event-archiver \
      --output=wide 2>&1 || true
    kubectl --context "${context}" --namespace "${namespace}" \
      get pod --selector app.kubernetes.io/name=scale-event-archiver \
      --output=wide 2>&1 || true
    kubectl --context "${context}" --namespace "${namespace}" \
      get job "${original_job}" "${replay_job}" --ignore-not-found \
      --output=wide 2>&1 || true
  } >"${failure_path}"
}

cleanup() {
  local status=$?
  trap - EXIT
  if (( status != 0 )); then
    capture_failure || true
  fi
  restore_runtime
  exit "${status}"
}
trap cleanup EXIT

quantity_to_integer() {
  local quantity="${1:-}"
  if [[ "${quantity}" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "${quantity}"
  elif [[ "${quantity}" =~ ^[0-9]+m$ ]]; then
    local milli="${quantity%m}"
    printf '%d\n' "$(((milli + 999) / 1000))"
  else
    return 1
  fi
}

hpa_state() {
  kubectl --context "${context}" --namespace "${namespace}" \
    get horizontalpodautoscaler/keda-hpa-scale-event-archiver \
    --output=jsonpath='{.status.currentReplicas} {.status.desiredReplicas} {.status.currentMetrics[0].external.current.averageValue}{.status.currentMetrics[0].external.current.value}'
}

wait_for_idle_baseline() {
  local deadline current_replicas desired_replicas raw_lag lag lag_valid specified_replicas available_replicas
  deadline=$((SECONDS + phase_timeout_seconds))
  current_replicas=0
  desired_replicas=0
  lag=""
  lag_valid=false
  specified_replicas=0
  available_replicas=0
  while (( SECONDS < deadline )); do
    read -r current_replicas desired_replicas raw_lag <<<"$(hpa_state)"
    current_replicas="${current_replicas:-0}"
    desired_replicas="${desired_replicas:-0}"
    if lag="$(quantity_to_integer "${raw_lag:-}")"; then
      lag_valid=true
    else
      lag=""
      lag_valid=false
    fi
    specified_replicas="$(kubectl --context "${context}" --namespace "${namespace}" \
      get deployment/scale-event-archiver --output=jsonpath='{.spec.replicas}')"
    available_replicas="$(kubectl --context "${context}" --namespace "${namespace}" \
      get deployment/scale-event-archiver --output=jsonpath='{.status.availableReplicas}')"
    if [[ "${lag_valid}" == "true" && "${current_replicas}" == "1" && "${desired_replicas}" == "1" && \
      "${specified_replicas}" == "1" && "${available_replicas:-0}" == "1" ]] && \
      (( lag == 0 )); then
      return 0
    fi
    sleep 3
  done
  printf 'ERROR: scale workload did not reach an idle one-replica baseline: current=%s desired=%s specified=%s available=%s lag=%s lag_metric_available=%s.\n' \
    "${current_replicas}" "${desired_replicas}" "${specified_replicas}" \
    "${available_replicas:-0}" "${lag:-unavailable}" "${lag_valid}" >&2
  return 1
}

job_complete() {
  [[ "$(kubectl --context "${context}" --namespace "${namespace}" \
    get job "$1" --output=jsonpath='{.status.conditions[?(@.type=="Complete")].status}')" == "True" ]]
}

job_failed() {
  [[ "$(kubectl --context "${context}" --namespace "${namespace}" \
    get job "$1" --output=jsonpath='{.status.conditions[?(@.type=="Failed")].status}')" == "True" ]]
}

require_no_unfinished_scale_jobs() {
  local job_states job_name complete failed unfinished_jobs
  if ! job_states="$(kubectl --context "${context}" --namespace "${namespace}" \
    get job \
    --output=jsonpath='{range .items[*]}{.metadata.name}{"|"}{.status.conditions[?(@.type=="Complete")].status}{"|"}{.status.conditions[?(@.type=="Failed")].status}{"\n"}{end}')"; then
    printf 'ERROR: unable to inspect existing scale producer jobs.\n' >&2
    return 1
  fi

  unfinished_jobs=""
  while IFS='|' read -r job_name complete failed; do
    case "${job_name}" in
      scale-original-* | scale-replay-*)
        if [[ "${complete}" != "True" && "${failed}" != "True" ]]; then
          unfinished_jobs="${unfinished_jobs}${unfinished_jobs:+, }${job_name}"
        fi
        ;;
    esac
  done <<<"${job_states}"

  if [[ -n "${unfinished_jobs}" ]]; then
    printf 'ERROR: unfinished prior scale producer job(s) prevent an isolated baseline: %s.\n' \
      "${unfinished_jobs}" >&2
    return 1
  fi
}

create_load_job() {
  local phase="$1"
  local job_name="$2"
  kubectl --context "${context}" --namespace "${namespace}" \
    set env cronjob/scale-load-producer \
    "LOAD_RUN_ID=${run_id}" "LOAD_PHASE=${phase}" "LOAD_EVENT_COUNT=${event_count}" \
    LOAD_INSTRUMENTS=LOAD-A,LOAD-B,LOAD-C,LOAD-D,LOAD-E,LOAD-F,LOAD-G,LOAD-H,LOAD-I,LOAD-J,LOAD-K,LOAD-L \
    LOAD_BASE_TIME=2026-07-30T00:00:00Z >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    delete job "${job_name}" --ignore-not-found --wait=true >/dev/null
  kubectl --context "${context}" --namespace "${namespace}" \
    create job --from=cronjob/scale-load-producer "${job_name}" >/dev/null
}

find_replacement_pod() {
  local previous_pods="$1"
  local candidate
  while IFS= read -r candidate; do
    if [[ -n "${candidate}" ]] &&
      ! grep -Fxq "${candidate}" <<<"${previous_pods}" &&
      [[ -z "$(kubectl --context "${context}" --namespace "${namespace}" \
        get pod "${candidate}" --output=jsonpath='{.metadata.deletionTimestamp}')" ]] &&
      [[ "$(kubectl --context "${context}" --namespace "${namespace}" \
        get pod "${candidate}" --output=jsonpath='{.status.conditions[?(@.type=="Ready")].status}')" == "True" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done < <(kubectl --context "${context}" --namespace "${namespace}" get pod \
    --selector app.kubernetes.io/name=scale-event-archiver \
    --output=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
  return 1
}

pin_scale_consumers() {
  local deadline hpa_min_replicas
  kubectl --context "${context}" --namespace "${namespace}" \
    patch scaledobject/scale-event-archiver --type=merge \
    --patch '{"spec":{"minReplicaCount":3,"maxReplicaCount":3}}' >/dev/null
  deadline=$((SECONDS + 30))
  hpa_min_replicas=""
  while (( SECONDS < deadline )); do
    hpa_min_replicas="$(kubectl --context "${context}" --namespace "${namespace}" \
      get horizontalpodautoscaler/keda-hpa-scale-event-archiver \
      --output=jsonpath='{.spec.minReplicas}')"
    if [[ "${hpa_min_replicas}" == "3" ]]; then
      return 0
    fi
    sleep 1
  done
  printf 'ERROR: KEDA did not pin the scale consumer minimum at three replicas.\n' >&2
  return 1
}

prometheus_query_to_file() {
  local query="$1"
  local output_file="$2"
  local encoded
  encoded="$(python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "${query}")"
  kubectl --context "${context}" get --raw \
    "/api/v1/namespaces/${OBSERVABILITY_NAMESPACE}/services/http:monitoring-kube-prometheus-prometheus:9090/proxy/api/v1/query?query=${encoded}" \
    >"${output_file}"
}

validate_replay_outcomes() {
  python3 - "$1" "$2" "${event_count}" "$3" <<'PY'
import json
import sys
from pathlib import Path


def values(path: str) -> dict[str, int]:
    document = json.loads(Path(path).read_text(encoding="utf-8"))
    if document.get("status") != "success":
        raise SystemExit("Prometheus counter query failed")
    return {
        item["metric"]["outcome"]: int(float(item["value"][1]))
        for item in document.get("data", {}).get("result", [])
    }


before = values(sys.argv[1])
after = values(sys.argv[2])
expected = int(sys.argv[3])
outcomes = ("created", "duplicate", "quarantined", "error")
if any(outcome not in before or outcome not in after for outcome in outcomes):
    raise SystemExit("Prometheus did not return every scale archiver outcome")
deltas = {outcome: after[outcome] - before[outcome] for outcome in outcomes}
if deltas != {"created": 0, "duplicate": expected, "quarantined": 0, "error": 0}:
    raise SystemExit(f"replay outcome deltas are not exact: {deltas}")
Path(sys.argv[4]).write_text(
    json.dumps(deltas, indent=2, sort_keys=True) + "\n",
    encoding="utf-8",
)
PY
}

printf 'Verifying KEDA and the isolated scale workload...\n'
helm status "${KEDA_RELEASE}" --kube-context "${context}" --namespace "${KEDA_NAMESPACE}" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  wait scaledobject/scale-event-archiver --for=condition=Ready --timeout=120s
restore_runtime
require_no_unfinished_scale_jobs
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/scale-event-archiver --timeout=120s
wait_for_idle_baseline

kubectl --context "${context}" --namespace "${namespace}" \
  set env deployment/scale-event-archiver "ARCHIVER_POST_WRITE_DELAY=${archiver_delay_ms}ms" >/dev/null
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/scale-event-archiver --timeout=120s >/dev/null

create_load_job original "${original_job}"

max_lag=0
max_replicas=1
killed_pod=""
pods_before_kill=""
kill_epoch=0
recovery_seconds=0
lag=0
lag_valid=false
ready_replicas=0
current_replicas=0
desired_replicas=0
original_complete=false
consumers_pinned=false
pod_deleted=false
phase_start_seconds=${SECONDS}
deadline=$((SECONDS + phase_timeout_seconds))
while (( SECONDS < deadline )); do
  read -r current_replicas desired_replicas raw_lag <<<"$(hpa_state)"
  current_replicas="${current_replicas:-0}"
  desired_replicas="${desired_replicas:-0}"
  if lag="$(quantity_to_integer "${raw_lag:-}")"; then
    lag_valid=true
    lag_json="${lag}"
  else
    lag=""
    lag_valid=false
    lag_json=null
  fi
  ready_replicas="$(kubectl --context "${context}" --namespace "${namespace}" \
    get deployment/scale-event-archiver --output=jsonpath='{.status.availableReplicas}')"
  ready_replicas="${ready_replicas:-0}"

  if [[ "${lag_valid}" == "true" ]] && (( lag > max_lag )); then
    max_lag="${lag}"
  fi
  (( current_replicas > max_replicas )) && max_replicas="${current_replicas}"
  (( desired_replicas > max_replicas )) && max_replicas="${desired_replicas}"

  if job_failed "${original_job}"; then
    kubectl --context "${context}" --namespace "${namespace}" logs "job/${original_job}" >&2 || true
    printf 'ERROR: original scale producer failed.\n' >&2
    exit 1
  fi
  if job_complete "${original_job}"; then
    original_complete=true
  fi

  printf '{"elapsed_seconds":%d,"lag":%s,"lag_metric_available":%s,"current_replicas":%d,"desired_replicas":%d,"available_replicas":%d,"producer_complete":%s,"consumer_pinned":%s,"pod_deleted":%s,"recovery_seconds":%d}\n' \
    "$((SECONDS - phase_start_seconds))" "${lag_json}" "${lag_valid}" "${current_replicas}" "${desired_replicas}" "${ready_replicas}" \
    "${original_complete}" "${consumers_pinned}" "${pod_deleted}" \
    "${recovery_seconds}" >>"${samples_path}"

  if [[ -z "${killed_pod}" ]] && (( ready_replicas >= 3 )); then
    pin_scale_consumers
    consumers_pinned=true
    pods_before_kill="$(kubectl --context "${context}" --namespace "${namespace}" \
      get pod --selector app.kubernetes.io/name=scale-event-archiver \
      --output=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')"
    killed_pod="$(head -n 1 <<<"${pods_before_kill}")"
    kill_epoch="$(date +%s)"
    kubectl --context "${context}" --namespace "${namespace}" \
      delete pod "${killed_pod}" --wait=false >/dev/null
    pod_deleted=true
  elif [[ -n "${killed_pod}" && "${recovery_seconds}" == "0" ]] && (( $(date +%s) > kill_epoch + 1 && ready_replicas >= 3 )); then
    if ! kubectl --context "${context}" --namespace "${namespace}" get pod "${killed_pod}" >/dev/null 2>&1; then
      replacement_pod="$(find_replacement_pod "${pods_before_kill}" || true)"
      if [[ -n "${replacement_pod}" ]]; then
        replacement_ip="$(kubectl --context "${context}" --namespace "${namespace}" \
          get pod "${replacement_pod}" --output=jsonpath='{.status.podIP}')"
        if kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
          /topic-inspector \
          --topic market.prices.scale \
          --group scale-event-archiver-v1 \
          --require-client-host "${replacement_ip}" \
          --expected-members 3 \
          --expected-partitions 3 \
          --output json >"${recovery_candidate_path}" 2>/dev/null; then
          mv "${recovery_candidate_path}" "${recovery_path}"
          recovery_seconds="$(($(date +%s) - kill_epoch))"
        else
          rm -f "${recovery_candidate_path}"
        fi
      fi
    fi
  fi

  if [[ "${original_complete}" == "true" && -n "${killed_pod}" && "${lag_valid}" == "true" ]] && \
    (( recovery_seconds > 0 && lag == 0 )); then
    break
  fi
  sleep 2
done

if [[ "${original_complete}" != "true" || -z "${killed_pod}" || "${lag_valid}" != "true" ]] || \
  (( recovery_seconds == 0 || lag != 0 )); then
  printf 'ERROR: original scale phase timed out after %d seconds: complete=%s max_lag=%d max_replicas=%d pinned=%s pod_deleted=%s recovery_seconds=%d final_lag=%s lag_metric_available=%s.\n' \
    "${phase_timeout_seconds}" "${original_complete}" "${max_lag}" "${max_replicas}" \
    "${consumers_pinned}" "${pod_deleted}" \
    "${recovery_seconds}" "${lag:-unavailable}" "${lag_valid}" >&2
  exit 1
fi
if (( max_lag < 1 || max_replicas != 3 || recovery_seconds > 60 )); then
  printf 'ERROR: scale evidence is outside bounds: max_lag=%d max_replicas=%d recovery_seconds=%d.\n' \
    "${max_lag}" "${max_replicas}" "${recovery_seconds}" >&2
  exit 1
fi

kubectl --context "${context}" --namespace "${namespace}" logs "job/${original_job}" \
  >"${artifact_dir}/producer-original.jsonl"
kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
  /topic-inspector \
  --topic market.prices.scale \
  --run-id "${run_id}" \
  --phase original \
  --expected "${event_count}" \
  --output json \
  --timeout 60s >"${artifact_dir}/ordering-original.json"
kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
  /archive-inspector \
  --prefix "bronze/market.price.observed/v1/date=2026-07-30/source=scale.${run_id}/" \
  --expected "${event_count}"

printf 'Keeping three consumers pinned while measuring exact replay outcomes...\n'
kubectl --context "${context}" --namespace "${namespace}" \
  rollout status deployment/scale-event-archiver --timeout=120s >/dev/null
sleep 6
outcome_query='sum by (outcome) (market_archiver_events_total{scope="scale"})'
prometheus_query_to_file "${outcome_query}" "${artifact_dir}/outcomes-before-replay.json"

printf 'Replaying the same event values to verify idempotent archive effects...\n'
create_load_job replay "${replay_job}"
replay_complete=false
deadline=$((SECONDS + phase_timeout_seconds))
while (( SECONDS < deadline )); do
  if job_failed "${replay_job}"; then
    kubectl --context "${context}" --namespace "${namespace}" logs "job/${replay_job}" >&2 || true
    printf 'ERROR: replay scale producer failed.\n' >&2
    exit 1
  fi
  if job_complete "${replay_job}"; then
    replay_complete=true
    break
  fi
  sleep 2
done
if [[ "${replay_complete}" != "true" ]]; then
  printf 'ERROR: replay scale producer did not complete within %d seconds.\n' \
    "${phase_timeout_seconds}" >&2
  exit 1
fi

deadline=$((SECONDS + phase_timeout_seconds))
lag=""
lag_valid=false
while (( SECONDS < deadline )); do
  read -r _ _ raw_lag <<<"$(hpa_state)"
  if lag="$(quantity_to_integer "${raw_lag:-}")"; then
    lag_valid=true
  else
    lag=""
    lag_valid=false
  fi
  if [[ "${lag_valid}" == "true" ]] && (( lag == 0 )); then
    break
  fi
  sleep 2
done
if [[ "${lag_valid}" != "true" ]] || (( lag != 0 )); then
  printf 'ERROR: replay consumer lag did not drain within %d seconds: final_lag=%s lag_metric_available=%s.\n' \
    "${phase_timeout_seconds}" "${lag:-unavailable}" "${lag_valid}" >&2
  exit 1
fi

kubectl --context "${context}" --namespace "${namespace}" logs "job/${replay_job}" \
  >"${artifact_dir}/producer-replay.jsonl"
kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
  /topic-inspector \
  --topic market.prices.scale \
  --run-id "${run_id}" \
  --phase replay \
  --expected "${event_count}" \
  --output json \
  --timeout 60s >"${artifact_dir}/ordering-replay.json"
kubectl --context "${context}" --namespace "${namespace}" exec deployment/scale-event-archiver -- \
  /archive-inspector \
  --prefix "bronze/market.price.observed/v1/date=2026-07-30/source=scale.${run_id}/" \
  --expected "${event_count}"

original_digest="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["value_digest_sha256"])' \
  "${artifact_dir}/ordering-original.json")"
replay_digest="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["value_digest_sha256"])' \
  "${artifact_dir}/ordering-replay.json")"
if [[ "${original_digest}" != "${replay_digest}" ]]; then
  printf 'ERROR: replay values do not match the original phase digest.\n' >&2
  exit 1
fi

replay_outcomes_valid=0
for _ in {1..20}; do
  prometheus_query_to_file "${outcome_query}" "${artifact_dir}/outcomes-after-replay.json"
  if validate_replay_outcomes \
    "${artifact_dir}/outcomes-before-replay.json" \
    "${artifact_dir}/outcomes-after-replay.json" \
    "${artifact_dir}/replay-outcomes.json" 2>/dev/null; then
    replay_outcomes_valid=1
    break
  fi
  sleep 2
done
if (( replay_outcomes_valid == 0 )); then
  validate_replay_outcomes \
    "${artifact_dir}/outcomes-before-replay.json" \
    "${artifact_dir}/outcomes-after-replay.json" \
    "${artifact_dir}/replay-outcomes.json"
fi

prometheus_query='histogram_quantile(0.95, sum by (le) (market_archiver_durable_latency_seconds_bucket{scope="scale"}))'
prometheus_query_to_file "${prometheus_query}" "${artifact_dir}/prometheus-p95.json"

kubectl --context "${context}" --namespace "${namespace}" \
  patch scaledobject/scale-event-archiver --type=merge \
  --patch '{"spec":{"minReplicaCount":1,"maxReplicaCount":3}}' >/dev/null
wait_for_idle_baseline
settled_replicas=1

REPORT_PATH="${report_path}" \
EVENT_COUNT="${event_count}" \
ARCHIVER_DELAY_MS="${archiver_delay_ms}" \
PHASE_TIMEOUT_SECONDS="${phase_timeout_seconds}" \
MINIMUM_PARTITION_BACKLOG_MS="${minimum_partition_backlog_ms}" \
RUN_ID="${run_id}" \
MAX_LAG="${max_lag}" \
MAX_REPLICAS="${max_replicas}" \
RECOVERY_SECONDS="${recovery_seconds}" \
SETTLED_REPLICAS="${settled_replicas}" \
ARTIFACT_DIR="${artifact_dir}" \
python3 - <<'PY'
import json
import os
from pathlib import Path

artifact_dir = Path(os.environ["ARTIFACT_DIR"])


def load_summary(path: Path) -> dict:
    for line in path.read_text(encoding="utf-8").splitlines():
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if value.get("msg") == "load scenario completed":
            return {
                "messages": value["messages"],
                "duration_milliseconds": value["duration_milliseconds"],
                "throughput_events_per_second": value["throughput_events_per_second"],
                "ack_p95_milliseconds": value["ack_p95_milliseconds"],
            }
    raise SystemExit(f"missing producer summary in {path}")


prometheus = json.loads((artifact_dir / "prometheus-p95.json").read_text(encoding="utf-8"))
results = prometheus.get("data", {}).get("result", [])
if prometheus.get("status") != "success" or len(results) != 1:
    raise SystemExit("Prometheus did not return exactly one scale p95 series")

report = {
    "schema_version": 1,
    "evidence_scope": "local_kind_synthetic",
    "run_id": os.environ["RUN_ID"],
    "privacy": {
        "run_instruments": "fixed_fictional",
        "personal_watchlist_in_report": False,
        "instrument_metric_labels": False,
    },
    "config": {
        "events_per_phase": int(os.environ["EVENT_COUNT"]),
        "injected_archive_delay_milliseconds": int(os.environ["ARCHIVER_DELAY_MS"]),
        "minimum_injected_work_per_partition_milliseconds": int(
            os.environ["MINIMUM_PARTITION_BACKLOG_MS"]
        ),
        "phase_timeout_seconds": int(os.environ["PHASE_TIMEOUT_SECONDS"]),
        "topic": "market.prices.scale",
        "partitions": 3,
        "consumer_group": "scale-event-archiver-v1",
    },
    "measurements": {
        "original_producer": load_summary(artifact_dir / "producer-original.jsonl"),
        "replay_producer": load_summary(artifact_dir / "producer-replay.jsonl"),
        "archive_durable_p95_seconds": float(results[0]["value"][1]),
        "max_consumer_lag": int(os.environ["MAX_LAG"]),
        "max_consumer_replicas": int(os.environ["MAX_REPLICAS"]),
        "consumer_group_recovery_seconds": int(os.environ["RECOVERY_SECONDS"]),
        "settled_consumer_replicas": int(os.environ["SETTLED_REPLICAS"]),
    },
    "assertions": {
        "original_ordering": json.loads((artifact_dir / "ordering-original.json").read_text(encoding="utf-8")),
        "replay_ordering": json.loads((artifact_dir / "ordering-replay.json").read_text(encoding="utf-8")),
        "consumer_group_recovery": json.loads((artifact_dir / "consumer-group-recovery.json").read_text(encoding="utf-8")),
        "replay_outcomes": json.loads((artifact_dir / "replay-outcomes.json").read_text(encoding="utf-8")),
        "unique_archive_objects": int(os.environ["EVENT_COUNT"]),
        "replay_archive_effect_idempotent": True,
        "replay_value_digest_matches_original": True,
        "lag_drained": True,
        "replica_ceiling_matches_partitions": True,
    },
    "limitations": [
        "Synthetic local load is not production or AWS capacity evidence.",
        "The single-replica Kafka and object-store services are not availability evidence.",
        "Provider market-data limits are outside this scale run.",
    ],
}
Path(os.environ["REPORT_PATH"]).write_text(
    json.dumps(report, indent=2, sort_keys=True) + "\n",
    encoding="utf-8",
)
PY

restore_runtime
trap - EXIT

printf 'Kafka scale lab passed. Evidence: %s\n' "${report_path}"
