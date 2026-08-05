#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  rollout status statefulset/jenkins --timeout=10m
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  rollout status statefulset/jenkins-artifacts --timeout=5m

for claim in jenkins-retained-home jenkins-artifact-garage; do
  phase="$(kubectl --context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" get pvc "${claim}" \
    --output=jsonpath='{.status.phase}')"
  if [[ "${phase}" != "Bound" ]]; then
    printf 'ERROR: retained claim %s is not bound.\n' "${claim}" >&2
    exit 1
  fi
done

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
  '<name>SOURCE_BRANCH</name>' \
  '<name>TRUSTED_PIPELINE_BRANCH</name>' \
  '<daysToKeep>3</daysToKeep>' \
  '<numToKeep>20</numToKeep>'; do
  if [[ "${job_config}" != *"${expected_setting}"* ]]; then
    printf 'ERROR: Jenkins job is missing SCM setting %s.\n' \
      "${expected_setting}" >&2
    exit 1
  fi
done
if [[ "${job_config}" != *'${TRUSTED_PIPELINE_BRANCH}'* ]] ||
  [[ "${job_config}" == *'refs/heads/${SOURCE_BRANCH}:refs/remotes/origin/${SOURCE_BRANCH}'* ]]; then
  printf 'ERROR: Jenkins is not loading its Pipeline from the trusted branch boundary.\n' >&2
  exit 1
fi

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins -- \
  test -f /var/jenkins_home/plugins/artifact-manager-s3.jpi
artifact_config='/var/jenkins_home/io.jenkins.plugins.artifact_manager_jclouds.s3.S3BlobStoreConfig.xml'
for expected_setting in \
  '<container>jenkins-artifacts</container>' \
  '<prefix>jenkins/</prefix>' \
  '<usePathStyleUrl>true</usePathStyleUrl>' \
  '<useHttp>true</useHttp>' \
  '<disableSessionToken>true</disableSessionToken>' \
  '<customEndpoint>127.0.0.1:13900</customEndpoint>' \
  '<customSigningRegion>us-east-1</customSigningRegion>'; do
  if ! kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
    exec statefulset/jenkins -- grep -Fq "${expected_setting}" "${artifact_config}"; then
    printf 'ERROR: Jenkins artifact manager is missing required Garage configuration.\n' >&2
    exit 1
  fi
done
if ! kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins -- \
  grep -Fq '<scope>SYSTEM</scope>' /var/jenkins_home/credentials.xml; then
  printf 'ERROR: Jenkins artifact credentials are visible outside system scope.\n' >&2
  exit 1
fi

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get serviceaccount jenkins-agent \
  --output=jsonpath='{.automountServiceAccountToken}' | grep -Fxq false

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get networkpolicy jenkins-jenkins-controller >/dev/null
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get networkpolicy jenkins-jenkins-agent >/dev/null

printf 'Jenkins controller, isolated Garage artifacts, RBAC, persistence, and network policy checks passed.\n'
