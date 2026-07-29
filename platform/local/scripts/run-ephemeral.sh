#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

profile="${EPHEMERAL_COLIMA_PROFILE:-investment-platform-ephemeral}"
cpus="${EPHEMERAL_COLIMA_CPUS:-4}"
memory_gib="${EPHEMERAL_COLIMA_MEMORY_GIB:-8}"
disk_gib="${EPHEMERAL_COLIMA_DISK_GIB:-30}"
profile_reserved=0
runtime_ready=0
docker_config=""

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

for command_name in colima docker kind kubectl helm make; do
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
  trap - EXIT

  if (( profile_reserved != 0 )); then
    if (( runtime_ready != 0 )) && docker info >/dev/null 2>&1; then
      kind delete cluster --name "${CLUSTER_NAME}" || cleanup_status=1
    fi
    if ! colima delete "${profile}" --force --data; then
      printf 'ERROR: failed to delete ephemeral Colima profile %s.\n' "${profile}" >&2
      cleanup_status=1
    fi
  fi

  if [[ -n "${docker_config}" ]]; then
    rm -rf "${docker_config}"
  fi
  if (( original_status == 0 && cleanup_status != 0 )); then
    original_status=${cleanup_status}
  fi
  exit "${original_status}"
}
trap cleanup EXIT

if profile_exists; then
  printf 'ERROR: profile %s already exists; refusing to delete a runtime this script did not create.\n' "${profile}" >&2
  printf 'Choose another EPHEMERAL_COLIMA_PROFILE or explicitly reclaim the existing dedicated profile.\n' >&2
  exit 1
fi

profile_reserved=1
docker_config="$(mktemp -d "${TMPDIR:-/tmp}/investment-platform-docker-config.XXXXXX")"
colima start "${profile}" \
  --activate=false \
  --runtime docker \
  --cpus "${cpus}" \
  --memory "${memory_gib}" \
  --disk "${disk_gib}"

export DOCKER_HOST="unix://${HOME}/.colima/${profile}/docker.sock"
export DOCKER_CONFIG="${docker_config}"
runtime_ready=1

run_complete_platform() {
  make -C "${LOCAL_DIR}" bootstrap || return
  make -C "${LOCAL_DIR}" bootstrap-data-path || return
  make -C "${LOCAL_DIR}" bootstrap-analytics || return
}

if ! run_complete_platform; then
  printf 'Ephemeral platform run failed; emitting diagnostics before cleanup.\n' >&2
  make -C "${LOCAL_DIR}" diagnose || true
  make -C "${LOCAL_DIR}" diagnose-data-path || true
  make -C "${LOCAL_DIR}" diagnose-analytics || true
  exit 1
fi

printf 'Ephemeral platform verification passed; deleting kind and Colima profile %s.\n' "${profile}"
