#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

wait_for_pods_deleted() {
  local selector="$1"
  local timeout="$2"
  local pods
  if ! pods="$(kubectl --context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" get pods \
    --selector "${selector}" --output=name)"; then
    printf 'ERROR: cannot list Jenkins pods during quiesce.\n' >&2
    return 1
  fi
  if [[ -n "${pods}" ]]; then
    kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
      wait pod --selector "${selector}" --for=delete --timeout="${timeout}"
  fi
}

scale_statefulset_if_present() {
  local name="$1"
  local resource
  if ! resource="$(kubectl --context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" get statefulset "${name}" \
    --ignore-not-found --output=name)"; then
    printf 'ERROR: cannot look up Jenkins statefulset %s during quiesce.\n' \
      "${name}" >&2
    return 1
  fi
  if [[ -n "${resource}" ]]; then
    kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
      scale "statefulset/${name}" --replicas=0 --timeout=2m
  fi
}

if ! namespace="$(kubectl --context "${JENKINS_CONTEXT}" get namespace \
  "${JENKINS_NAMESPACE}" --ignore-not-found --output=name)"; then
  printf 'ERROR: cannot determine whether the Jenkins namespace exists.\n' >&2
  exit 1
fi
if [[ -z "${namespace}" ]]; then
  exit 0
fi

status=0
scale_statefulset_if_present jenkins || status=1
wait_for_pods_deleted \
  'app.kubernetes.io/component=jenkins-controller,app.kubernetes.io/instance=jenkins' \
  2m || status=1

if ! agent_pods="$(kubectl --context "${JENKINS_CONTEXT}" \
  --namespace "${JENKINS_NAMESPACE}" get pods \
  --selector 'jenkins/jenkins-jenkins-agent=true' --output=name)"; then
  printf 'ERROR: cannot list Jenkins agent pods during quiesce.\n' >&2
  status=1
elif [[ -n "${agent_pods}" ]]; then
  kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
    delete pods --selector 'jenkins/jenkins-jenkins-agent=true' \
    --grace-period=30 --timeout=2m --wait=true || status=1
fi

scale_statefulset_if_present jenkins-artifacts || status=1
wait_for_pods_deleted 'app.kubernetes.io/name=jenkins-artifact-garage' 2m || status=1

if (( status != 0 )); then
  printf 'ERROR: Jenkins retained workloads did not quiesce cleanly.\n' >&2
fi
exit "${status}"
