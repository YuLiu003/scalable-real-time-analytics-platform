#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
local_dir="${repo_root}/platform/local"

# shellcheck disable=SC1091
source "${local_dir}/versions.lock"

for command_name in tofu kubectl kind helm docker make; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: PS2 requires %s on the isolated build agent.\n' "${command_name}" >&2
    exit 1
  fi
done

"${repo_root}/scripts/ci/validate-aws-platform.sh"

kubectl kustomize "${repo_root}/platform/gitops/clusters/local" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/platform/local/market-data-services" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/local/market-pipeline" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/local/portfolio-analytics" >/dev/null

if [[ "$(uname -s)" == "Darwin" && "${CI:-false}" != "true" ]]; then
  make -C "${local_dir}" e2e-ephemeral
  exit
fi

if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  printf 'ERROR: refusing to reuse or delete existing cluster %s.\n' "${CLUSTER_NAME}" >&2
  exit 1
fi

cleanup() {
  status=$?
  trap - EXIT
  if (( status != 0 )) && kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
    make -C "${local_dir}" diagnose || true
    make -C "${local_dir}" diagnose-data-path || true
    make -C "${local_dir}" diagnose-analytics || true
  fi
  if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
    kind delete cluster --name "${CLUSTER_NAME}" || status=1
  fi
  exit "${status}"
}
trap cleanup EXIT

make -C "${local_dir}" bootstrap
make -C "${local_dir}" bootstrap-data-path
make -C "${local_dir}" bootstrap-analytics

printf 'PS2 integration, Kubernetes, and infrastructure checks passed.\n'
