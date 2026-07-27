#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

"${script_dir}/preflight.sh"

if kind get clusters | grep -Fxq "${JENKINS_CLUSTER_NAME}"; then
  printf 'ERROR: refusing to reuse existing cluster %s.\n' "${JENKINS_CLUSTER_NAME}" >&2
  exit 1
fi

"${script_dir}/build-agent.sh"

kind create cluster \
  --name "${JENKINS_CLUSTER_NAME}" \
  --config "${jenkins_dir}/kind/cluster.yaml" \
  --wait 180s
kind load docker-image "${JENKINS_AGENT_IMAGE}" --name "${JENKINS_CLUSTER_NAME}"

kubectl --context "${JENKINS_CONTEXT}" create namespace "${JENKINS_NAMESPACE}"
kubectl --context "${JENKINS_CONTEXT}" label namespace "${JENKINS_NAMESPACE}" \
  pod-security.kubernetes.io/enforce=privileged \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted

admin_password="$(openssl rand -hex 24)"
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  create secret generic jenkins-admin \
  --from-literal=jenkins-admin-user=admin \
  --from-literal="jenkins-admin-password=${admin_password}"

helm repo add jenkins https://charts.jenkins.io --force-update
helm repo update jenkins
helm upgrade --install "${JENKINS_RELEASE}" jenkins/jenkins \
  --version "${JENKINS_CHART_VERSION}" \
  --kube-context "${JENKINS_CONTEXT}" \
  --namespace "${JENKINS_NAMESPACE}" \
  --values "${jenkins_dir}/helm/values.yaml" \
  --wait \
  --timeout 15m

"${script_dir}/verify.sh"
