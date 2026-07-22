#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

profile="${COLIMA_PROFILE:-investment-platform}"
docker_config=""

if [[ ! "${profile}" =~ ^investment-platform(-[a-z0-9][a-z0-9-]*)?$ ]]; then
  printf 'ERROR: refusing to delete non-project Colima profile %s.\n' "${profile}" >&2
  exit 2
fi
if ! command -v colima >/dev/null 2>&1; then
  printf 'ERROR: colima is required to reclaim the dedicated runtime.\n' >&2
  exit 1
fi

profile_exists() {
  colima list 2>/dev/null | awk 'NR > 1 { print $1 }' | grep -Fxq "${profile}"
}

if ! profile_exists; then
  printf 'Colima profile %s does not exist; nothing to reclaim.\n' "${profile}"
  exit 0
fi

if [[ "${CONFIRM_RUNTIME_CLEANUP:-}" != "${profile}" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'ERROR: set CONFIRM_RUNTIME_CLEANUP=%s for non-interactive deletion.\n' "${profile}" >&2
    exit 1
  fi
  printf 'Type %s to delete its kind cluster, images, volumes, and Colima VM disk: ' "${profile}"
  read -r confirmation
  if [[ "${confirmation}" != "${profile}" ]]; then
    printf 'Runtime cleanup cancelled.\n'
    exit 0
  fi
fi

socket_path="${HOME}/.colima/${profile}/docker.sock"
if command -v docker >/dev/null 2>&1 && command -v kind >/dev/null 2>&1; then
  docker_config="$(mktemp -d "${TMPDIR:-/tmp}/investment-platform-docker-config.XXXXXX")"
  trap 'rm -rf "${docker_config}"' EXIT
  export DOCKER_HOST="unix://${socket_path}"
  export DOCKER_CONFIG="${docker_config}"

  if docker info >/dev/null 2>&1 && kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
    kind delete cluster --name "${CLUSTER_NAME}"
  fi
fi

colima delete "${profile}" --force --data
printf 'Deleted dedicated Colima profile %s and its container-runtime data.\n' "${profile}"
