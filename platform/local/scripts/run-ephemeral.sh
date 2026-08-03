#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

workflow="${EPHEMERAL_WORKFLOW:-complete}"
case "${workflow}" in
  complete)
    default_profile=investment-platform-ephemeral
    ;;
  capacity)
    default_profile=investment-platform-capacity-ephemeral
    ;;
  *)
    printf 'ERROR: EPHEMERAL_WORKFLOW must be complete or capacity.\n' >&2
    exit 2
    ;;
esac

profile="${EPHEMERAL_COLIMA_PROFILE:-${default_profile}}"
cpus="${EPHEMERAL_COLIMA_CPUS:-4}"
memory_gib="${EPHEMERAL_COLIMA_MEMORY_GIB:-8}"
disk_gib="${EPHEMERAL_COLIMA_DISK_GIB:-30}"
capacity_smoke="${EPHEMERAL_CAPACITY_SMOKE:-false}"
profile_reserved=0
runtime_ready=0
runtime_config_dir=""

if [[ ! "${profile}" =~ ^investment-platform-[a-z0-9][a-z0-9-]*$ ]]; then
  printf 'ERROR: EPHEMERAL_COLIMA_PROFILE must start with investment-platform- and contain lowercase letters, digits, or hyphens.\n' >&2
  exit 2
fi
for value_name in cpus memory_gib disk_gib; do
  value="${!value_name}"
  if [[ ! "${value}" =~ ^[1-9][0-9]*$ ]]; then
    printf 'ERROR: %s must be a positive integer.\n' "${value_name}" >&2
    exit 2
  fi
done
if [[ "${capacity_smoke}" != "true" && "${capacity_smoke}" != "false" ]]; then
  printf 'ERROR: EPHEMERAL_CAPACITY_SMOKE must be true or false.\n' >&2
  exit 2
fi

for command_name in colima docker kind kubectl helm go make openssl python3; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: required command %s was not found.\n' "${command_name}" >&2
    exit 1
  fi
done

profile_exists() {
  colima list 2>/dev/null | awk 'NR > 1 { print $1 }' | grep -Fxq "${profile}"
}

cleanup() {
  original_status=$?
  cleanup_status=0
  trap - EXIT HUP INT TERM

  if (( profile_reserved != 0 )); then
    if (( runtime_ready != 0 )) && docker info >/dev/null 2>&1; then
      kind delete cluster --name "${CLUSTER_NAME}" || cleanup_status=1
    fi
    if ! colima delete "${profile}" --force --data; then
      printf 'ERROR: failed to delete ephemeral Colima profile %s.\n' "${profile}" >&2
      cleanup_status=1
    fi
  fi

  if [[ -n "${runtime_config_dir}" ]]; then
    rm -rf "${runtime_config_dir}"
  fi
  if (( original_status == 0 && cleanup_status != 0 )); then
    original_status=${cleanup_status}
  fi
  exit "${original_status}"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if profile_exists; then
  printf 'ERROR: profile %s already exists; refusing to delete a runtime this script did not create.\n' "${profile}" >&2
  printf 'Choose another EPHEMERAL_COLIMA_PROFILE or explicitly reclaim the existing dedicated profile.\n' >&2
  exit 1
fi

runtime_config_dir="$(mktemp -d "${TMPDIR:-/tmp}/investment-platform-runtime-config.XXXXXX")"
export DOCKER_CONFIG="${runtime_config_dir}/docker"
export KUBECONFIG="${runtime_config_dir}/kubeconfig"
mkdir -p "${DOCKER_CONFIG}"
unset DOCKER_CONTEXT DOCKER_HOST

profile_reserved=1
colima start "${profile}" \
  --activate=false \
  --runtime docker \
  --cpus "${cpus}" \
  --memory "${memory_gib}" \
  --disk "${disk_gib}"

export DOCKER_HOST="unix://${HOME}/.colima/${profile}/docker.sock"
runtime_ready=1

run_complete_platform() {
  make -C "${LOCAL_DIR}" bootstrap || return
  make -C "${LOCAL_DIR}" bootstrap-data-path || return
  make -C "${LOCAL_DIR}" bootstrap-analytics || return
  make -C "${LOCAL_DIR}" verify-scale-lab || return
  if [[ "${capacity_smoke}" == "true" ]]; then
    CAPACITY_ALLOCATED_CPUS="${cpus}" \
    CAPACITY_ALLOCATED_MEMORY_GIB="${memory_gib}" \
    CAPACITY_ALLOCATED_DISK_GIB="${disk_gib}" \
      make -C "${LOCAL_DIR}" verify-capacity-smoke || return
  fi
  CONFIRM_DESTROY_ANALYTICS=portfolio-analytics \
    make -C "${LOCAL_DIR}" destroy-analytics || return
  make -C "${LOCAL_DIR}" bootstrap-analytics || return
}

run_capacity_platform() {
  make -C "${LOCAL_DIR}" bootstrap || return
  make -C "${LOCAL_DIR}" bootstrap-data-path || return
  make -C "${LOCAL_DIR}" verify-scale-lab || return
  CAPACITY_ALLOCATED_CPUS="${cpus}" \
  CAPACITY_ALLOCATED_MEMORY_GIB="${memory_gib}" \
  CAPACITY_ALLOCATED_DISK_GIB="${disk_gib}" \
    make -C "${LOCAL_DIR}" verify-capacity-benchmark || return
}

if [[ "${workflow}" == "capacity" ]]; then
  run_platform=run_capacity_platform
else
  run_platform=run_complete_platform
fi

if ! "${run_platform}"; then
  printf 'Ephemeral %s run failed; emitting diagnostics before cleanup.\n' "${workflow}" >&2
  make -C "${LOCAL_DIR}" diagnose || true
  make -C "${LOCAL_DIR}" diagnose-data-path || true
  if [[ "${workflow}" == "complete" ]]; then
    make -C "${LOCAL_DIR}" diagnose-analytics || true
  fi
  exit 1
fi

printf 'Ephemeral %s verification passed; deleting kind and Colima profile %s.\n' "${workflow}" "${profile}"
