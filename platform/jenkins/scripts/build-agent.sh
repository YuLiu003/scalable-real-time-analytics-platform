#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${jenkins_dir}/../.." && pwd)"
build_context="$(mktemp -d "${TMPDIR:-/tmp}/jenkins-agent-context.XXXXXX")"

cleanup() {
  rm -rf "${build_context}"
}
trap cleanup EXIT

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

mkdir -p \
  "${build_context}/services/portfolio-analytics" \
  "${build_context}/tools/codex-plugins/cloud-platform-engineering/mcp"
cp \
  "${repo_root}/services/portfolio-analytics/requirements.txt" \
  "${repo_root}/services/portfolio-analytics/requirements-dev.txt" \
  "${build_context}/services/portfolio-analytics/"
cp \
  "${repo_root}/tools/codex-plugins/cloud-platform-engineering/mcp/requirements-dev.txt" \
  "${build_context}/tools/codex-plugins/cloud-platform-engineering/mcp/"

architecture="$(docker info --format '{{.Architecture}}')"
case "${architecture}" in
  aarch64 | arm64)
    target_arch=arm64
    docker_cli_image="${DOCKER_CLI_ARM64_IMAGE}"
    go_sha256="${GO_LINUX_ARM64_SHA256}"
    kubectl_sha256="${KUBECTL_LINUX_ARM64_SHA256}"
    helm_sha256="${HELM_LINUX_ARM64_SHA256}"
    tofu_sha256="${OPENTOFU_LINUX_ARM64_SHA256}"
    ;;
  x86_64 | amd64)
    target_arch=amd64
    docker_cli_image="${DOCKER_CLI_AMD64_IMAGE}"
    go_sha256="${GO_LINUX_AMD64_SHA256}"
    kubectl_sha256="${KUBECTL_LINUX_AMD64_SHA256}"
    helm_sha256="${HELM_LINUX_AMD64_SHA256}"
    tofu_sha256="${OPENTOFU_LINUX_AMD64_SHA256}"
    ;;
  *)
    printf 'ERROR: unsupported Docker architecture %s.\n' "${architecture}" >&2
    exit 1
    ;;
esac

build_args=(
  --file "${jenkins_dir}/agent/Dockerfile"
  --tag "${JENKINS_AGENT_IMAGE}"
  --build-arg "TARGETARCH=${target_arch}"
  --build-arg "DOCKER_CLI_IMAGE=${docker_cli_image}"
  --build-arg "PYTHON_IMAGE=${PYTHON_IMAGE}"
  --build-arg "GO_VERSION=${GO_VERSION}"
  --build-arg "GO_SHA256=${go_sha256}"
  --build-arg "KIND_VERSION=${KIND_VERSION}"
  --build-arg "KUBERNETES_VERSION=${KUBERNETES_VERSION}"
  --build-arg "KUBECTL_SHA256=${kubectl_sha256}"
  --build-arg "HELM_VERSION=${HELM_VERSION}"
  --build-arg "HELM_SHA256=${helm_sha256}"
  --build-arg "OPENTOFU_VERSION=${OPENTOFU_VERSION}"
  --build-arg "OPENTOFU_SHA256=${tofu_sha256}"
  --build-arg "ACTIONLINT_VERSION=${ACTIONLINT_VERSION}"
  "${build_context}"
)
for attempt in 1 2 3; do
  if docker build "${build_args[@]}"; then
    exit 0
  fi
  if (( attempt == 3 )); then
    printf 'ERROR: Jenkins agent image build failed after three attempts.\n' >&2
    exit 1
  fi
  printf 'WARN: Jenkins agent image build attempt %d failed; retrying cached layers.\n' \
    "${attempt}" >&2
  sleep "$((attempt * 5))"
done
