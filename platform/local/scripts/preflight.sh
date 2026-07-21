#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

failed=0

require_command() {
  local command_name="$1"
  local install_hint="$2"

  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: required command %s was not found. %s\n' "${command_name}" "${install_hint}" >&2
    failed=1
  fi
}

require_command docker "Install and start Docker Desktop or another Docker-compatible daemon."
require_command kind "Install kind ${KIND_VERSION}."
require_command kubectl "Install a kubectl client compatible with Kubernetes ${KUBERNETES_VERSION}."
require_command helm "Install Helm 3."
require_command openssl "Install OpenSSL for local credential generation."

if (( failed != 0 )); then
  exit 1
fi

installed_kind_version="$(kind version | awk '{print $2}')"
if [[ "${installed_kind_version}" != "${KIND_VERSION}" ]]; then
  printf 'ERROR: kind %s is installed; this baseline requires %s.\n' "${installed_kind_version}" "${KIND_VERSION}" >&2
  failed=1
fi

installed_helm_version="$(helm version --short 2>/dev/null | sed 's/+.*//')"
if [[ ! "${installed_helm_version}" =~ ^v3\. ]]; then
  printf 'ERROR: Helm 3 is required; found %s.\n' "${installed_helm_version:-unknown}" >&2
  failed=1
fi

if ! docker info >/dev/null 2>&1; then
  printf 'ERROR: Docker is installed but its daemon is not reachable.\n' >&2
  failed=1
fi

kubectl_client_version="$(kubectl version --client=true -o json 2>/dev/null | sed -n 's/.*"gitVersion": "\([^"]*\)".*/\1/p' | head -n 1)"
printf 'kind:       %s\n' "${installed_kind_version}"
printf 'Kubernetes: %s (%s)\n' "${KUBERNETES_VERSION}" "${KIND_NODE_IMAGE}"
printf 'kubectl:    %s\n' "${kubectl_client_version:-unknown}"
printf 'Helm:       %s\n' "${installed_helm_version}"
printf 'Monitoring: kube-prometheus-stack %s\n' "${KUBE_PROMETHEUS_STACK_VERSION}"

if (( failed != 0 )); then
  exit 1
fi

printf 'Preflight checks passed.\n'
