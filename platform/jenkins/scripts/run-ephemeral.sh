#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"
mode="${1:-verify}"
profile="${JENKINS_COLIMA_PROFILE:-investment-platform-jenkins-ephemeral}"
runtime_config_dir=""
runtime_config_created=0
profile_reserved=0
colima_started=0
profile_lock_acquired=0
profile_lock_dir=""
retained_lock_acquired=0
fences_releasable=0
storage_monitor_pid=""
ui_session_pid=""
metrics_valid=true

if (( $# > 1 )) || [[ "${mode}" != "verify" && "${mode}" != "ui" ]]; then
  printf 'Usage: %s [verify|ui]\n' "$0" >&2
  exit 2
fi
if [[ ! "${profile}" =~ ^investment-platform-jenkins-[a-z0-9][a-z0-9-]*$ ]]; then
  printf 'ERROR: invalid Jenkins Colima profile name.\n' >&2
  exit 2
fi
for command_name in colima curl docker git kind kubectl helm make openssl python3; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: required command %s was not found.\n' "${command_name}" >&2
    exit 1
  fi
done

remove_runtime_config() {
  if (( runtime_config_created == 0 )); then
    return
  fi
  if rm -rf -- "${runtime_config_dir}"; then
    runtime_config_created=0
  else
    printf 'ERROR: failed to remove protected Jenkins runtime configuration.\n' >&2
    return 1
  fi
}

cleanup() {
  status="${1:-$?}"
  cleanup_status=0
  profile_delete_allowed=1
  trap - EXIT HUP INT TERM
  if [[ -n "${storage_monitor_pid}" ]]; then
    kill "${storage_monitor_pid}" >/dev/null 2>&1 || true
    wait "${storage_monitor_pid}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${ui_session_pid}" ]]; then
    kill "${ui_session_pid}" >/dev/null 2>&1 || true
    wait "${ui_session_pid}" >/dev/null 2>&1 || true
  fi
  if (( colima_started != 0 )); then
    if ! JENKINS_KEEP_RETAINED_LOCK=true "${script_dir}/destroy.sh"; then
      cleanup_status=1
      profile_delete_allowed=0
      fences_releasable=0
      printf 'ERROR: Jenkins cluster cleanup failed; retaining the Colima VM and ownership fences.\n' >&2
    fi
  fi
  if (( profile_reserved != 0 && profile_delete_allowed != 0 )); then
    if colima delete "${profile}" --force --data; then
      fences_releasable=1
    else
      cleanup_status=1
      fences_releasable=0
      printf 'ERROR: Colima deletion failed; retaining Jenkins runtime ownership fences.\n' >&2
    fi
  fi
  if (( retained_lock_acquired != 0 )); then
    if (( fences_releasable != 0 )); then
      if [[ "${mode}" == "ui" ]] && ! remove_runtime_config; then
        cleanup_status=1
        fences_releasable=0
      elif "${script_dir}/retained-state.sh" release; then
        if (( profile_lock_acquired != 0 )); then
          rmdir "${profile_lock_dir}" || cleanup_status=1
        fi
      else
        cleanup_status=1
      fi
    else
      printf 'WARN: runtime absence is unproven; retained-state and profile locks remain.\n' >&2
    fi
  elif (( profile_lock_acquired != 0 )); then
    rmdir "${profile_lock_dir}" || cleanup_status=1
  fi
  if [[ "${mode}" == "verify" ]]; then
    remove_runtime_config || cleanup_status=1
  elif (( runtime_config_created != 0 )); then
    printf 'WARN: protected UI runtime configuration remains with the ownership fences.\n' >&2
  fi
  if (( status == 0 && cleanup_status != 0 )); then
    status=1
  fi
  exit "${status}"
}
trap 'cleanup $?' EXIT
trap 'cleanup 129' HUP
trap 'cleanup 130' INT
trap 'cleanup 143' TERM

if command -v caffeinate >/dev/null 2>&1; then
  printf 'Preventing macOS idle sleep during disposable Jenkins %s.\n' "${mode}"
  caffeinate -dimsu -w "$$" &
fi

unset DOCKER_CONTEXT DOCKER_HOST
if [[ "${mode}" == "verify" ]]; then
  runtime_config_dir="$(mktemp -d "${TMPDIR:-/tmp}/jenkins-runtime-config.XXXXXX")"
  runtime_config_created=1
  export DOCKER_CONFIG="${runtime_config_dir}/docker"
  export KUBECONFIG="${runtime_config_dir}/kubeconfig"
  mkdir -p "${DOCKER_CONFIG}"
fi

profile_lock_dir="${HOME}/.colima/.investment-platform-locks/${profile}"
mkdir -p "$(dirname "${profile_lock_dir}")"
if ! mkdir "${profile_lock_dir}" 2>/dev/null; then
  printf 'ERROR: Jenkins Colima profile is already owned by another run.\n' >&2
  exit 1
fi
profile_lock_acquired=1
retained_state_dir="$("${script_dir}/retained-state.sh" path)"
export JENKINS_RETAINED_STATE_DIR="${retained_state_dir}"
JENKINS_RETAINED_LOCK_TOKEN="$("${script_dir}/retained-state.sh" acquire)"
export JENKINS_RETAINED_LOCK_TOKEN
retained_lock_acquired=1
export JENKINS_RETAINED_LOCK_HELD=true
fences_releasable=1
if [[ "${mode}" == "ui" ]]; then
  runtime_config_dir="${retained_state_dir}/.ui-runtime"
  if [[ -e "${runtime_config_dir}" || -L "${runtime_config_dir}" ]]; then
    printf 'ERROR: protected Jenkins UI runtime configuration already exists.\n' >&2
    exit 1
  fi
  umask 077
  mkdir "${runtime_config_dir}"
  chmod 0700 "${runtime_config_dir}"
  runtime_config_created=1
  export DOCKER_CONFIG="${runtime_config_dir}/docker"
  export KUBECONFIG="${runtime_config_dir}/kubeconfig"
  mkdir "${DOCKER_CONFIG}"
  chmod 0700 "${DOCKER_CONFIG}"
fi
if ! profiles="$(colima list 2>/dev/null)"; then
  printf 'ERROR: cannot determine whether the Jenkins Colima profile exists.\n' >&2
  exit 1
fi
if printf '%s\n' "${profiles}" | awk 'NR > 1 { print $1 }' | grep -Fxq "${profile}"; then
  printf 'ERROR: refusing to reuse or delete existing profile %s.\n' "${profile}" >&2
  exit 1
fi

retained_state_dir="$("${script_dir}/retained-state.sh" prepare)"

profile_reserved=1
fences_releasable=0
colima start "${profile}" \
  --activate=false \
  --runtime docker \
  --cpus "${JENKINS_COLIMA_CPUS:-8}" \
  --memory "${JENKINS_COLIMA_MEMORY_GIB:-16}" \
  --disk "${JENKINS_COLIMA_DISK_GIB:-50}" \
  --mount "${retained_state_dir}:/var/local/investment-platform/jenkins-retained:w"
colima_started=1

export DOCKER_HOST="unix://${HOME}/.colima/${profile}/docker.sock"

make -C "${jenkins_dir}" bootstrap

if [[ "${mode}" == "ui" ]]; then
  env -u JENKINS_RETAINED_LOCK_TOKEN -u JENKINS_RETAINED_LOCK_HELD \
    "${script_dir}/ui-session.sh" &
  ui_session_pid=$!
  if wait "${ui_session_pid}"; then
    ui_session_pid=""
  else
    ui_status=$?
    ui_session_pid=""
    exit "${ui_status}"
  fi
  "${script_dir}/retained-state.sh" check
  printf 'Disposable Jenkins UI session ended; deleting %s.\n' "${profile}"
  exit 0
fi

expected_commit="${JENKINS_EXPECTED_COMMIT:-$(git -C "${jenkins_dir}/../.." rev-parse HEAD)}"
if [[ ! "${expected_commit}" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'ERROR: expected commit must be a 40-character lowercase SHA.\n' >&2
  exit 2
fi
export JENKINS_EXPECTED_COMMIT="${expected_commit}"

metrics_enabled="${JENKINS_STORAGE_METRICS_ENABLED:-true}"
if [[ "${metrics_enabled}" != "true" && "${metrics_enabled}" != "false" ]]; then
  printf 'ERROR: JENKINS_STORAGE_METRICS_ENABLED must be true or false.\n' >&2
  exit 2
fi
if [[ "${metrics_enabled}" == "true" ]]; then
  colima_profile_dir="${HOME}/.colima/${profile}"
  baseline_file="${runtime_config_dir}/storage-baseline.json"
  peak_file="${runtime_config_dir}/storage-peak.json"
  build_number_file="${runtime_config_dir}/build-number"
  python3 "${script_dir}/storage-metrics.py" sample \
    --retained-state "${retained_state_dir}" \
    --colima-profile "${colima_profile_dir}" >"${baseline_file}"
  cp "${baseline_file}" "${peak_file}"
  python3 "${script_dir}/storage-metrics.py" monitor \
    --retained-state "${retained_state_dir}" \
    --colima-profile "${colima_profile_dir}" \
    --peak-state "${peak_file}" \
    --interval-seconds "${JENKINS_STORAGE_SAMPLE_SECONDS:-5}" &
  storage_monitor_pid=$!
  sleep 0.1
  if ! kill -0 "${storage_monitor_pid}" >/dev/null 2>&1; then
    wait "${storage_monitor_pid}" >/dev/null 2>&1 || true
    storage_monitor_pid=""
    printf 'ERROR: Jenkins storage peak monitor failed during startup.\n' >&2
    exit 1
  fi
fi

trigger_status=0
if JENKINS_BUILD_NUMBER_FILE="${build_number_file:-}" \
  make -C "${jenkins_dir}" trigger; then
  build_result=SUCCESS
else
  trigger_status=$?
  build_result=UNKNOWN
fi

if [[ -n "${storage_monitor_pid}" ]]; then
  monitor_status=0
  if ! kill "${storage_monitor_pid}" >/dev/null 2>&1; then
    wait "${storage_monitor_pid}" >/dev/null 2>&1 || true
    metrics_valid=false
  else
    wait "${storage_monitor_pid}" >/dev/null 2>&1 || monitor_status=$?
    if (( monitor_status != 143 )); then
      metrics_valid=false
    fi
  fi
  storage_monitor_pid=""
  if [[ "${metrics_valid}" != "true" ]]; then
    printf 'ERROR: Jenkins storage peak monitor exited unexpectedly.\n' >&2
    trigger_status=1
  fi
fi

if [[ "${metrics_enabled}" == "true" && "${metrics_valid}" == "true" &&
  -s "${build_number_file}" ]]; then
  python3 "${script_dir}/storage-metrics.py" record-peak \
    --retained-state "${retained_state_dir}" \
    --colima-profile "${colima_profile_dir}" \
    --peak-state "${peak_file}"
  build_number="$(<"${build_number_file}")"
  controller_build_kib="$(kubectl --context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" exec statefulset/jenkins -- \
    du -sk "/var/jenkins_home/jobs/investment-platform-presubmit/builds/${build_number}" |
    awk '{print $1}')"
  console_bytes="$(kubectl --context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" exec statefulset/jenkins -- \
    sh -c 'wc -c <"$1"' sh \
    "/var/jenkins_home/jobs/investment-platform-presubmit/builds/${build_number}/log")"
  report_path="${retained_state_dir}/reports/${expected_commit}-build-${build_number}.json"
  if ! python3 "${script_dir}/storage-metrics.py" finalize \
    --baseline "${baseline_file}" \
    --peak-state "${peak_file}" \
    --report "${report_path}" \
    --commit "${expected_commit}" \
    --build-number "${build_number}" \
    --result "${build_result}" \
    --controller-build-bytes "$((controller_build_kib * 1024))" \
    --console-bytes "${console_bytes}"; then
    if (( trigger_status == 0 )); then
      trigger_status=1
    fi
  else
    printf 'Storage report: %s\n' "${report_path}"
  fi
elif [[ "${metrics_enabled}" == "true" && "${metrics_valid}" == "true" &&
  ${trigger_status} -eq 0 ]]; then
  printf 'ERROR: successful Jenkins run did not expose a numeric build for storage measurement.\n' >&2
  trigger_status=1
fi

if ! "${script_dir}/retained-state.sh" check; then
  trigger_status=1
fi
if (( trigger_status != 0 )); then
  exit "${trigger_status}"
fi

printf 'Disposable Jenkins production-like validation passed; deleting %s.\n' "${profile}"
