#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

if ! clusters="$(kind get clusters)"; then
  printf 'ERROR: cannot determine whether the Jenkins cluster exists.\n' >&2
  exit 1
fi

status=0
safe_to_release=0
if printf '%s\n' "${clusters}" | grep -Fxq "${JENKINS_CLUSTER_NAME}"; then
  if [[ -z "${JENKINS_RETAINED_LOCK_TOKEN:-}" ]]; then
    if ! JENKINS_RETAINED_LOCK_TOKEN="$(kubectl \
      --context "${JENKINS_CONTEXT}" \
      --namespace "${JENKINS_NAMESPACE}" \
      get secret jenkins-retained-lock-owner \
      -o 'go-template={{index .data "owner-token" | base64decode}}')"; then
      printf 'ERROR: cannot recover retained-state lock ownership from the Jenkins cluster.\n' >&2
      exit 1
    fi
    export JENKINS_RETAINED_LOCK_TOKEN
  fi
  "${script_dir}/retained-state.sh" verify-owner
  if "${script_dir}/quiesce.sh"; then
    if kind delete cluster --name "${JENKINS_CLUSTER_NAME}"; then
      safe_to_release=1
    else
      status=1
    fi
  else
    status=1
  fi
else
  "${script_dir}/retained-state.sh" verify-owner
  safe_to_release=1
fi

if (( safe_to_release != 0 )) &&
  [[ "${JENKINS_KEEP_RETAINED_LOCK:-false}" != "true" ]]; then
  "${script_dir}/retained-state.sh" release || status=1
fi
exit "${status}"
