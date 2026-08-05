#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

standalone_lock_acquired=0
cluster_create_attempted=0

cleanup_bootstrap_lock() {
  status="${1:-$?}"
  trap - EXIT HUP INT TERM
  if (( status != 0 && standalone_lock_acquired != 0 )); then
    safe_to_release=0
    if (( cluster_create_attempted == 0 )); then
      safe_to_release=1
    elif clusters="$(kind get clusters)"; then
      if printf '%s\n' "${clusters}" | grep -Fxq "${JENKINS_CLUSTER_NAME}"; then
        if "${script_dir}/quiesce.sh"; then
          if kind delete cluster --name "${JENKINS_CLUSTER_NAME}"; then
            safe_to_release=1
          else
            printf 'WARN: bootstrap failed and could not delete its Jenkins cluster; retaining ownership.\n' >&2
          fi
        else
          printf 'WARN: bootstrap failed and its Jenkins workloads did not quiesce; retaining ownership.\n' >&2
        fi
      else
        safe_to_release=1
      fi
    else
      printf 'WARN: bootstrap failed and cluster absence is unproven; retaining ownership.\n' >&2
    fi
    if (( safe_to_release != 0 )) &&
      ! "${script_dir}/retained-state.sh" release; then
      printf 'WARN: bootstrap failed and could not release its retained-state lock.\n' >&2
    fi
  fi
  exit "${status}"
}
trap 'cleanup_bootstrap_lock $?' EXIT
trap 'cleanup_bootstrap_lock 129' HUP
trap 'cleanup_bootstrap_lock 130' INT
trap 'cleanup_bootstrap_lock 143' TERM

"${script_dir}/preflight.sh"
if [[ "${JENKINS_RETAINED_LOCK_HELD:-false}" != "true" ]]; then
  retained_state_dir="$("${script_dir}/retained-state.sh" path)"
  export JENKINS_RETAINED_STATE_DIR="${retained_state_dir}"
  JENKINS_RETAINED_LOCK_TOKEN="$("${script_dir}/retained-state.sh" acquire)"
  export JENKINS_RETAINED_LOCK_TOKEN
  standalone_lock_acquired=1
  export JENKINS_RETAINED_LOCK_HELD=true
fi
retained_state_dir="$("${script_dir}/retained-state.sh" prepare)"

if ! clusters="$(kind get clusters)"; then
  printf 'ERROR: cannot determine whether the Jenkins cluster exists.\n' >&2
  exit 1
fi
if printf '%s\n' "${clusters}" | grep -Fxq "${JENKINS_CLUSTER_NAME}"; then
  printf 'ERROR: refusing to reuse existing cluster %s.\n' "${JENKINS_CLUSTER_NAME}" >&2
  exit 1
fi

"${script_dir}/build-agent.sh"

architecture="$(docker info --format '{{.Architecture}}')"
case "${architecture}" in
  aarch64 | arm64)
    dind_source="${DOCKER_DIND_ARM64_SOURCE}"
    garage_source="${GARAGE_ARM64_SOURCE}"
    socat_source="${SOCAT_ARM64_SOURCE}"
    ;;
  x86_64 | amd64)
    dind_source="${DOCKER_DIND_AMD64_SOURCE}"
    garage_source="${GARAGE_AMD64_SOURCE}"
    socat_source="${SOCAT_AMD64_SOURCE}"
    ;;
  *)
    printf 'ERROR: unsupported Docker architecture %s.\n' "${architecture}" >&2
    exit 1
    ;;
esac
docker pull "${dind_source}"
docker tag "${dind_source}" "${DOCKER_DIND_IMAGE}"
docker pull "${JENKINS_KIND_NODE_IMAGE}"
docker pull "${garage_source}"
docker tag "${garage_source}" "${GARAGE_IMAGE}"
docker pull "${socat_source}"
docker tag "${socat_source}" "${SOCAT_IMAGE}"
if ! docker run --rm \
  --entrypoint sh \
  --volume /var/local/investment-platform/jenkins-retained:/retained:ro \
  "${JENKINS_KIND_NODE_IMAGE}" \
  -c 'test -f /retained/.jenkins-retained-state'; then
  printf 'ERROR: %s is not mounted into the Jenkins Colima VM.\n' \
    "${retained_state_dir}" >&2
  exit 1
fi

cluster_create_attempted=1
kind create cluster \
  --name "${JENKINS_CLUSTER_NAME}" \
  --config "${jenkins_dir}/kind/cluster.yaml" \
  --image "${JENKINS_KIND_NODE_IMAGE}" \
  --wait 180s
kind load docker-image "${JENKINS_AGENT_IMAGE}" --name "${JENKINS_CLUSTER_NAME}"
kind load docker-image "${DOCKER_DIND_IMAGE}" --name "${JENKINS_CLUSTER_NAME}"
kind load docker-image "${GARAGE_IMAGE}" --name "${JENKINS_CLUSTER_NAME}"
kind load docker-image "${SOCAT_IMAGE}" --name "${JENKINS_CLUSTER_NAME}"

kubectl --context "${JENKINS_CONTEXT}" create namespace "${JENKINS_NAMESPACE}"
kubectl --context "${JENKINS_CONTEXT}" label namespace "${JENKINS_NAMESPACE}" \
  pod-security.kubernetes.io/enforce=privileged \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted
printf '%s' "${JENKINS_RETAINED_LOCK_TOKEN}" |
  kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
    create secret generic jenkins-retained-lock-owner \
    --from-file=owner-token=/dev/stdin

"${script_dir}/bootstrap-artifact-store.sh"

admin_password="$(openssl rand -hex 24)"
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  create secret generic jenkins-admin \
  --from-literal=jenkins-admin-user=admin \
  --from-literal="jenkins-admin-password=${admin_password}"

helm repo add jenkins https://charts.jenkins.io --force-update
helm repo update jenkins
jenkins_ready=0
for attempt in 1 2; do
  if helm upgrade --install "${JENKINS_RELEASE}" jenkins/jenkins \
    --version "${JENKINS_CHART_VERSION}" \
    --kube-context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" \
    --values "${jenkins_dir}/helm/values.yaml" \
    --wait \
    --timeout 15m; then
    jenkins_ready=1
    break
  fi
  if (( attempt < 2 )); then
    printf 'Jenkins install attempt %d failed; retrying with cached image layers.\n' \
      "${attempt}" >&2
  fi
done
if (( jenkins_ready == 0 )); then
  printf 'ERROR: Jenkins did not become ready after two bounded attempts.\n' >&2
  exit 1
fi

"${script_dir}/verify.sh"
