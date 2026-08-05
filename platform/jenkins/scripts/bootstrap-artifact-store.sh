#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
state_dir="$("${script_dir}/retained-state.sh" prepare)"
temporary_access_file=""
temporary_secret_file=""
temporary_config_file=""
cleanup() {
  [[ -z "${temporary_access_file}" ]] || rm -f -- "${temporary_access_file}"
  [[ -z "${temporary_secret_file}" ]] || rm -f -- "${temporary_secret_file}"
  [[ -z "${temporary_config_file}" ]] || rm -f -- "${temporary_config_file}"
}
trap cleanup EXIT

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

read_retained_secret() {
  "${script_dir}/retained-state.sh" read "$1"
}

rpc_secret="$(read_retained_secret rpc-secret)"
admin_token="$(read_retained_secret admin-token)"
metrics_token="$(read_retained_secret metrics-token)"
temporary_config_file="$(mktemp "${TMPDIR:-/tmp}/jenkins-garage-config.XXXXXX")"
chmod 0600 "${temporary_config_file}"
printf '%s\n' \
  'metadata_dir = "/var/lib/garage/meta"' \
  'data_dir = "/var/lib/garage/data"' \
  'db_engine = "sqlite"' \
  'replication_factor = 1' \
  'compression_level = 1' \
  'rpc_bind_addr = "[::]:3901"' \
  'rpc_public_addr = "jenkins-artifacts-0.jenkins-artifacts-internal.jenkins-system.svc.cluster.local:3901"' \
  "rpc_secret = \"${rpc_secret}\"" \
  '[s3_api]' \
  's3_region = "us-east-1"' \
  'api_bind_addr = "[::]:3900"' \
  'root_domain = ".s3.garage.local"' \
  '[admin]' \
  'api_bind_addr = "[::]:3903"' \
  "admin_token = \"${admin_token}\"" \
  "metrics_token = \"${metrics_token}\"" >"${temporary_config_file}"
unset rpc_secret admin_token metrics_token

kubectl --context "${JENKINS_CONTEXT}" apply \
  -f "${jenkins_dir}/storage/volumes.yaml"
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  create secret generic jenkins-artifact-garage-config \
  --from-file="garage.toml=${temporary_config_file}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${JENKINS_CONTEXT}" apply -f -
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  create secret generic jenkins-artifact-admin \
  --from-file="access-key=${state_dir}/secrets/admin-access-key" \
  --from-file="secret-key=${state_dir}/secrets/admin-secret-key" \
  --dry-run=client --output=yaml \
  | kubectl --context "${JENKINS_CONTEXT}" apply -f -
kubectl --context "${JENKINS_CONTEXT}" apply \
  -f "${jenkins_dir}/storage/garage.yaml"
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  rollout status statefulset/jenkins-artifacts --timeout=5m

runtime_access_file="${state_dir}/secrets/runtime-access-key"
runtime_secret_file="${state_dir}/secrets/runtime-secret-key"
if [[ -L "${runtime_access_file}" ]] || [[ -L "${runtime_secret_file}" ]]; then
  printf 'ERROR: retained Jenkins runtime credential set contains a symbolic link.\n' >&2
  exit 1
fi
if [[ -f "${runtime_access_file}" && -f "${runtime_secret_file}" ]]; then
  runtime_access_key="$(read_retained_secret runtime-access-key)"
  runtime_secret_key="$(read_retained_secret runtime-secret-key)"
  if ! kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
    exec statefulset/jenkins-artifacts -- \
    /garage key info "${runtime_access_key}" >/dev/null; then
    printf 'ERROR: retained Garage data does not contain the retained Jenkins runtime key.\n' >&2
    exit 1
  fi
elif [[ ! -e "${runtime_access_file}" && ! -e "${runtime_secret_file}" ]]; then
  key_output="$(kubectl --context "${JENKINS_CONTEXT}" \
    --namespace "${JENKINS_NAMESPACE}" exec statefulset/jenkins-artifacts -- \
    /garage key create jenkins-artifacts-runtime)"
  runtime_access_key="$(printf '%s\n' "${key_output}" |
    sed -n 's/^Key ID:[[:space:]]*//p' | head -n 1)"
  runtime_secret_key="$(printf '%s\n' "${key_output}" |
    sed -n 's/^Secret key:[[:space:]]*//p' | head -n 1)"
  unset key_output
  if [[ ! "${runtime_access_key}" =~ ^GK[A-Za-z0-9]{20,64}$ ]] ||
    [[ ! "${runtime_secret_key}" =~ ^[A-Za-z0-9+/=_-]{32,128}$ ]]; then
    printf 'ERROR: Garage returned an invalid runtime credential shape.\n' >&2
    exit 1
  fi
  umask 077
  temporary_access_file="$(mktemp "${state_dir}/secrets/.runtime-access.XXXXXX")"
  temporary_secret_file="$(mktemp "${state_dir}/secrets/.runtime-secret.XXXXXX")"
  printf '%s' "${runtime_access_key}" >"${temporary_access_file}"
  printf '%s' "${runtime_secret_key}" >"${temporary_secret_file}"
  mv "${temporary_access_file}" "${runtime_access_file}"
  mv "${temporary_secret_file}" "${runtime_secret_file}"
  temporary_access_file=""
  temporary_secret_file=""
  chmod 0600 "${runtime_access_file}" "${runtime_secret_file}"
else
  printf 'ERROR: retained Jenkins runtime credential set is incomplete.\n' >&2
  exit 1
fi

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins-artifacts -- \
  /garage key deny --create-bucket "${runtime_access_key}" >/dev/null
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins-artifacts -- \
  /garage bucket allow --read --write \
  --key "${runtime_access_key}" jenkins-artifacts >/dev/null
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins-artifacts -- \
  /garage bucket deny --owner \
  --key "${runtime_access_key}" jenkins-artifacts >/dev/null
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  exec statefulset/jenkins-artifacts -- \
  /garage bucket set-quotas --max-size 1GiB --max-objects 10000 \
  jenkins-artifacts >/dev/null

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  create secret generic jenkins-artifact-runtime \
  --from-file="access-key=${runtime_access_file}" \
  --from-file="secret-key=${runtime_secret_file}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${JENKINS_CONTEXT}" apply -f -
unset runtime_access_key runtime_secret_key

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  create configmap jenkins-artifact-lifecycle \
  --from-file="configure-artifact-lifecycle.py=${jenkins_dir}/storage/configure-artifact-lifecycle.py" \
  --dry-run=client --output=yaml \
  | kubectl --context "${JENKINS_CONTEXT}" apply -f -
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  delete job configure-jenkins-artifact-lifecycle --ignore-not-found --wait=true
kubectl --context "${JENKINS_CONTEXT}" apply \
  -f "${jenkins_dir}/storage/lifecycle-job.yaml"
if ! kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  wait job/configure-jenkins-artifact-lifecycle \
  --for=condition=Complete --timeout=3m; then
  printf 'ERROR: Garage did not accept and verify the required three-day lifecycle.\n' >&2
  exit 1
fi
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  delete job configure-jenkins-artifact-lifecycle --wait=true

printf 'Dedicated Garage artifact storage, quota, and three-day lifecycle are ready.\n'
