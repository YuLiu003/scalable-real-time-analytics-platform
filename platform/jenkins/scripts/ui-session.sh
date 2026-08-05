#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

timeout_seconds="${JENKINS_UI_TIMEOUT_SECONDS:-7200}"
jenkins_port="${JENKINS_LOCAL_PORT:-18080}"
garage_port=13900
jenkins_forward_pid=""
garage_forward_pid=""
jenkins_log=""
garage_log=""

cleanup() {
  trap - EXIT HUP INT TERM
  for process_id in "${jenkins_forward_pid}" "${garage_forward_pid}"; do
    if [[ -n "${process_id}" ]]; then
      kill "${process_id}" >/dev/null 2>&1 || true
      wait "${process_id}" >/dev/null 2>&1 || true
    fi
  done
  for log_path in "${jenkins_log}" "${garage_log}"; do
    if [[ -n "${log_path}" ]]; then
      rm -f -- "${log_path}"
    fi
  done
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ ! "${timeout_seconds}" =~ ^[0-9]+$ ]] ||
  (( timeout_seconds < 1 || timeout_seconds > 14400 )); then
  printf 'ERROR: JENKINS_UI_TIMEOUT_SECONDS must be between 1 and 14400.\n' >&2
  exit 2
fi
if [[ ! "${jenkins_port}" =~ ^[0-9]+$ ]] ||
  (( jenkins_port < 1024 || jenkins_port > 65535 )); then
  printf 'ERROR: JENKINS_LOCAL_PORT must be between 1024 and 65535.\n' >&2
  exit 2
fi
if [[ -z "${KUBECONFIG:-}" ]] || [[ "${KUBECONFIG}" != /* ]] ||
  [[ -L "${KUBECONFIG}" ]] || [[ ! -f "${KUBECONFIG}" ]]; then
  printf 'ERROR: Jenkins UI requires the protected live-runtime kubeconfig.\n' >&2
  exit 1
fi
retained_state_dir="$("${script_dir}/retained-state.sh" path)"
expected_kubeconfig="${retained_state_dir}/.ui-runtime/kubeconfig"
if [[ "${KUBECONFIG}" != "${expected_kubeconfig}" ]] ||
  [[ -L "$(dirname "${KUBECONFIG}")" ]]; then
  printf 'ERROR: Jenkins UI refused a kubeconfig outside its protected runtime.\n' >&2
  exit 1
fi
chmod 0600 "${KUBECONFIG}"

runtime_dir="$(dirname "${KUBECONFIG}")"
jenkins_log="${runtime_dir}/jenkins-port-forward.log"
garage_log="${runtime_dir}/garage-port-forward.log"
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  port-forward service/jenkins "${jenkins_port}:8080" >"${jenkins_log}" 2>&1 &
jenkins_forward_pid=$!
kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  port-forward service/jenkins-artifacts "${garage_port}:3900" \
  >"${garage_log}" 2>&1 &
garage_forward_pid=$!

ready=0
ready_deadline=$((SECONDS + 60))
while (( SECONDS < ready_deadline )); do
  if ! kill -0 "${jenkins_forward_pid}" >/dev/null 2>&1 ||
    ! kill -0 "${garage_forward_pid}" >/dev/null 2>&1; then
    break
  fi
  if curl --connect-timeout 1 --max-time 2 --fail --silent --output /dev/null \
    "http://127.0.0.1:${jenkins_port}/login" &&
    curl --connect-timeout 1 --max-time 2 --silent --output /dev/null \
      "http://127.0.0.1:${garage_port}/jenkins-artifacts"; then
    ready=1
    break
  fi
  sleep 1
done
if (( ready == 0 )); then
  cat "${jenkins_log}" "${garage_log}" >&2
  printf 'ERROR: Jenkins UI port-forwards did not become ready.\n' >&2
  exit 1
fi

printf '\nJenkins UI is ready.\n'
printf 'URL: http://127.0.0.1:%s\n' "${jenkins_port}"
printf 'Username: admin\n'
printf 'Password: run `make -C platform/jenkins ui-password` in another terminal.\n'
printf 'Retained artifact links are available through localhost:%s.\n' \
  "${garage_port}"
printf 'Press Ctrl-C to stop now; automatic cleanup starts after %s seconds.\n\n' \
  "${timeout_seconds}"

deadline=$((SECONDS + timeout_seconds))
while (( SECONDS < deadline )); do
  if ! kill -0 "${jenkins_forward_pid}" >/dev/null 2>&1 ||
    ! kill -0 "${garage_forward_pid}" >/dev/null 2>&1; then
    cat "${jenkins_log}" "${garage_log}" >&2
    printf 'ERROR: a Jenkins UI port-forward stopped unexpectedly.\n' >&2
    exit 1
  fi
  sleep 1
done
printf 'Jenkins UI session reached its timeout.\n'
