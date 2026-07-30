#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  rollout status statefulset/jenkins --timeout=10m

executors="$(kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins -- \
  sh -c "grep -o '<numExecutors>[0-9]*</numExecutors>' /var/jenkins_home/config.xml")"
if [[ "${executors}" != "<numExecutors>0</numExecutors>" ]]; then
  printf 'ERROR: Jenkins controller executors are not disabled.\n' >&2
  exit 1
fi

job_config="$(kubectl --context "${JENKINS_CONTEXT}" \
  --namespace "${JENKINS_NAMESPACE}" exec statefulset/jenkins -- \
  cat /var/jenkins_home/jobs/investment-platform-presubmit/config.xml)"
for expected_setting in \
  '<shallow>true</shallow>' \
  '<noTags>true</noTags>' \
  '<honorRefspec>true</honorRefspec>' \
  '<depth>1</depth>' \
  '<name>EXPECTED_COMMIT</name>' \
  '<name>SOURCE_BRANCH</name>'; do
  if [[ "${job_config}" != *"${expected_setting}"* ]]; then
    printf 'ERROR: Jenkins job is missing SCM setting %s.\n' \
      "${expected_setting}" >&2
    exit 1
  fi
done

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get serviceaccount jenkins-agent \
  --output=jsonpath='{.automountServiceAccountToken}' | grep -Fxq false

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get networkpolicy jenkins-jenkins-controller >/dev/null
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get networkpolicy jenkins-jenkins-agent >/dev/null

printf 'Jenkins controller, isolation, RBAC, persistence, and network policy checks passed.\n'
