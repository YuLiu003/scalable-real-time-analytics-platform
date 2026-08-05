#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

state_dir="$("${script_dir}/retained-state.sh" path)"
runtime_dir="${state_dir}/.ui-runtime"
kubeconfig="${runtime_dir}/kubeconfig"
if [[ -L "${runtime_dir}" ]] || [[ ! -d "${runtime_dir}" ]] ||
  [[ -L "${kubeconfig}" ]] || [[ ! -f "${kubeconfig}" ]]; then
  printf 'ERROR: no active Jenkins UI session; run `make -C platform/jenkins ui`.\n' >&2
  exit 1
fi
if [[ ! -d "${state_dir}/.active-lock" ]] || [[ -L "${state_dir}/.active-lock" ]]; then
  printf 'ERROR: no active Jenkins UI session; run `make -C platform/jenkins ui`.\n' >&2
  exit 1
fi

if ! password="$(KUBECONFIG="${kubeconfig}" kubectl \
  --context "${JENKINS_CONTEXT}" \
  --namespace "${JENKINS_NAMESPACE}" \
  --request-timeout=5s \
  get secret jenkins-admin \
  --output='go-template={{index .data "jenkins-admin-password" | base64decode}}' \
  2>/dev/null)"; then
  printf 'ERROR: the Jenkins UI session is not reachable.\n' >&2
  exit 1
fi
if [[ ! "${password}" =~ ^[0-9a-f]{48}$ ]]; then
  printf 'ERROR: the live Jenkins password has an invalid format.\n' >&2
  exit 1
fi
printf '%s\n' "${password}"
