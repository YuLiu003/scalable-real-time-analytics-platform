#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

if ! command -v kind >/dev/null 2>&1; then
  printf 'ERROR: kind is required to inspect or destroy cluster %s.\n' "${CLUSTER_NAME}" >&2
  exit 1
fi

if ! kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  printf 'Cluster %s does not exist; nothing to destroy.\n' "${CLUSTER_NAME}"
  exit 0
fi

if [[ "${CONFIRM_DESTROY:-}" != "${CLUSTER_NAME}" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'ERROR: set CONFIRM_DESTROY=%s for non-interactive deletion.\n' "${CLUSTER_NAME}" >&2
    exit 1
  fi

  printf 'Type %s to delete the local cluster and all of its data: ' "${CLUSTER_NAME}"
  read -r confirmation
  if [[ "${confirmation}" != "${CLUSTER_NAME}" ]]; then
    printf 'Deletion cancelled.\n'
    exit 0
  fi
fi

kind delete cluster --name "${CLUSTER_NAME}"
printf 'Deleted local cluster %s.\n' "${CLUSTER_NAME}"
