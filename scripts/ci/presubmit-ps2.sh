#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
local_dir="${repo_root}/platform/local"

# shellcheck disable=SC1091
source "${local_dir}/versions.lock"

for command_name in tofu kubectl kind helm docker go make python3; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: PS2 requires %s on the isolated build agent.\n' "${command_name}" >&2
    exit 1
  fi
done

"${repo_root}/scripts/ci/validate-aws-platform.sh"

kubectl kustomize "${repo_root}/platform/gitops/clusters/local" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/platform/local/market-data-services" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/local/market-pipeline" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/private/market-feed" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/local/portfolio-analytics" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/private/portfolio-analytics" >/dev/null
kubectl kustomize "${repo_root}/platform/gitops/apps/private/portfolio-analytics-acceptance" >/dev/null

if [[ "$(uname -s)" == "Darwin" && "${CI:-false}" != "true" ]]; then
  EPHEMERAL_CAPACITY_SMOKE=true make -C "${local_dir}" e2e-ephemeral
  exit
fi

export KIND_CLUSTER_CONFIG="${KIND_CLUSTER_CONFIG:-${local_dir}/kind/ci-cluster.yaml}"

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
make -C "${local_dir}" verify-scale-lab
docker_cpus="${CAPACITY_ALLOCATED_CPUS:-$(docker info --format '{{.NCPU}}')}"
if [[ -n "${CAPACITY_ALLOCATED_MEMORY_GIB:-}" ]]; then
  docker_memory_gib="${CAPACITY_ALLOCATED_MEMORY_GIB}"
else
  docker_memory_bytes="$(docker info --format '{{.MemTotal}}')"
  docker_memory_gib="$(python3 -c 'import sys; print(max(1, int(sys.argv[1]) // (1024 ** 3)))' "${docker_memory_bytes}")"
fi
if [[ -n "${CAPACITY_ALLOCATED_DISK_GIB:-}" ]]; then
  docker_disk_gib="${CAPACITY_ALLOCATED_DISK_GIB}"
else
  docker_root="$(docker info --format '{{.DockerRootDir}}')"
  docker_disk_gib="$(docker run --rm --network none --read-only --cap-drop=ALL \
    --security-opt no-new-privileges:true \
    --mount "type=bind,src=${docker_root},dst=/capacity-docker-root,readonly" \
    --entrypoint /usr/bin/df "${KIND_NODE_IMAGE}" -Pk /capacity-docker-root | \
    awk 'NR == 2 { gib = int($2 / 1048576); print gib > 0 ? gib : 1 }')"
fi
CAPACITY_ALLOCATED_CPUS="${docker_cpus}" \
CAPACITY_ALLOCATED_MEMORY_GIB="${docker_memory_gib}" \
CAPACITY_ALLOCATED_DISK_GIB="${docker_disk_gib}" \
  make -C "${local_dir}" verify-capacity-smoke
CONFIRM_DESTROY_ANALYTICS=portfolio-analytics make -C "${local_dir}" destroy-analytics
make -C "${local_dir}" bootstrap-analytics
make -C "${local_dir}" verify-private-portfolio

printf 'PS2 integration, Kubernetes, and infrastructure checks passed.\n'
