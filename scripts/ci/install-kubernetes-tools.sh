#!/usr/bin/env bash
set -Eeuo pipefail

if (( $# != 1 )); then
  printf 'Usage: %s OUTPUT_DIRECTORY\n' "$0" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUTPUT_DIR="$1"

# shellcheck disable=SC1091
source "${REPO_ROOT}/platform/local/versions.lock"

if [[ "$(uname -s)" != "Linux" || "$(uname -m)" != "x86_64" ]]; then
  printf 'ERROR: this CI installer currently supports Linux amd64 only.\n' >&2
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"
temporary="$(mktemp -d)"
trap 'rm -rf "${temporary}"' EXIT

printf 'Installing kind %s from the signed Go module graph...\n' "${KIND_VERSION}"
GOBIN="${OUTPUT_DIR}" go install "sigs.k8s.io/kind@${KIND_VERSION}"

printf 'Installing kubectl %s with SHA-256 verification...\n' "${KUBERNETES_VERSION}"
curl --fail --silent --show-error --location \
  "https://dl.k8s.io/release/${KUBERNETES_VERSION}/bin/linux/amd64/kubectl" \
  --output "${temporary}/kubectl"
printf '%s  %s\n' "${KUBECTL_LINUX_AMD64_SHA256}" "${temporary}/kubectl" | sha256sum --check --status
install -m 0755 "${temporary}/kubectl" "${OUTPUT_DIR}/kubectl"

printf 'Installing Helm %s with SHA-256 verification...\n' "${HELM_VERSION}"
helm_archive="helm-${HELM_VERSION}-linux-amd64.tar.gz"
curl --fail --silent --show-error --location \
  "https://get.helm.sh/${helm_archive}" \
  --output "${temporary}/${helm_archive}"
printf '%s  %s\n' "${HELM_LINUX_AMD64_SHA256}" "${temporary}/${helm_archive}" | sha256sum --check --status
tar -xzf "${temporary}/${helm_archive}" -C "${temporary}"
install -m 0755 "${temporary}/linux-amd64/helm" "${OUTPUT_DIR}/helm"

"${OUTPUT_DIR}/kind" version
"${OUTPUT_DIR}/kubectl" version --client=true
"${OUTPUT_DIR}/helm" version --short
