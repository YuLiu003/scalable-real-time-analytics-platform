#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

kind delete cluster --name "${JENKINS_CLUSTER_NAME}"
