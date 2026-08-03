#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/runtime-cleanup-test.XXXXXX")"
fake_bin="${test_root}/bin"
fake_home="${test_root}/home"
fake_tmp="${test_root}/tmp"
log_file="${test_root}/commands.log"
trap 'rm -rf "${test_root}"' EXIT
mkdir -p "${fake_bin}" "${fake_home}/.docker" "${fake_home}/.kube" "${fake_tmp}"
printf 'caller-docker-config\n' >"${fake_home}/.docker/config.json"
printf 'caller-kubeconfig\n' >"${fake_home}/.kube/config"

cat >"${fake_bin}/fake-command" <<'FAKE'
#!/usr/bin/env bash
set -Eeuo pipefail
command_name="$(basename "$0")"
printf '%s %s\n' "${command_name}" "$*" >>"${FAKE_LOG}"
case "${command_name}" in
  caffeinate)
    [[ "${1:-}" == "-dimsu" ]]
    [[ "${2:-}" == "-w" ]]
    monitored_pid="${3:-}"
    [[ "${monitored_pid}" =~ ^[0-9]+$ ]]
    while kill -0 "${monitored_pid}" >/dev/null 2>&1; do
      sleep 0.05
    done
    exit 0
    ;;
  colima)
    if [[ "${1:-}" == "list" ]]; then
      printf 'PROFILE STATUS ARCH CPUS MEMORY DISK RUNTIME ADDRESS\n'
      if [[ "${FAKE_PROFILE_PRESENT:-0}" == "1" ]]; then
        printf '%s Running aarch64 4 8GiB 30GiB docker -\n' "${FAKE_PROFILE_NAME}"
      fi
    elif [[ "${1:-}" == "start" ]]; then
      if [[ -n "${DOCKER_CONTEXT:-}" || -n "${DOCKER_HOST:-}" ]]; then
        printf 'inherited Docker override reached Colima start\n' >&2
        exit 1
      fi
      docker_config="${DOCKER_CONFIG:-${HOME}/.docker}"
      kubeconfig="${KUBECONFIG:-${HOME}/.kube/config}"
      if [[ "${docker_config}" == "${HOME}/.docker" ||
        "${kubeconfig}" == "${HOME}/.kube/config" ]]; then
        printf 'ephemeral runtime used caller configuration\n' >&2
        exit 1
      fi
      mkdir -p "${docker_config}" "$(dirname "${kubeconfig}")"
      printf 'generated-colima-context\n' >"${docker_config}/colima-context"
      printf 'runtime-config %s %s\n' "${docker_config}" "${kubeconfig}" >>"${FAKE_LOG}"
      if [[ "${FAKE_FAIL_COLIMA_START:-0}" == "1" ]]; then
        exit 1
      fi
    fi
    ;;
  docker)
    [[ "${1:-}" == "info" ]]
    ;;
  kind)
    if [[ "${1:-}" == "get" && "${2:-}" == "clusters" ]]; then
      if [[ "${FAKE_CLUSTER_PRESENT:-0}" == "1" ]]; then
        printf 'investment-platform\n'
      fi
    fi
    ;;
  make)
    target="${3:-}"
    if [[ "${target}" == "bootstrap" ]]; then
      kubeconfig="${KUBECONFIG:-${HOME}/.kube/config}"
      mkdir -p "$(dirname "${kubeconfig}")"
      printf 'generated-kind-context\n' >"${kubeconfig}"
    fi
    if [[ -n "${FAKE_FAIL_TARGET:-}" && "${target}" == "${FAKE_FAIL_TARGET}" ]]; then
      exit 1
    fi
    if [[ -n "${FAKE_BLOCK_TARGET:-}" && "${target}" == "${FAKE_BLOCK_TARGET}" ]]; then
      : >"${FAKE_BLOCK_READY}"
      while [[ ! -e "${FAKE_BLOCK_RELEASE}" ]]; do
        sleep 0.05
      done
    fi
    ;;
esac
FAKE
chmod +x "${fake_bin}/fake-command"
for command_name in caffeinate colima curl docker git go kind kubectl helm make openssl python3; do
  ln -s fake-command "${fake_bin}/${command_name}"
done

export PATH="${fake_bin}:${PATH}"
export HOME="${fake_home}"
export TMPDIR="${fake_tmp}"
export FAKE_LOG="${log_file}"
export FAKE_PROFILE_NAME=investment-platform
unset DOCKER_CONFIG KUBECONFIG

assert_runtime_config_isolated_and_removed() {
  local record docker_config kubeconfig runtime_config_dir
  record="$(grep '^runtime-config ' "${log_file}" | tail -n 1)"
  read -r _ docker_config kubeconfig <<<"${record}"
  runtime_config_dir="$(dirname "${docker_config}")"

  [[ "${runtime_config_dir}" == "${TMPDIR}/"* ]]
  [[ "${docker_config}" == "${runtime_config_dir}/docker" ]]
  [[ "${kubeconfig}" == "${runtime_config_dir}/kubeconfig" ]]
  [[ ! -e "${runtime_config_dir}" ]]
  grep -Fxq 'caller-docker-config' "${HOME}/.docker/config.json"
  grep -Fxq 'caller-kubeconfig' "${HOME}/.kube/config"
  [[ ! -e "${HOME}/.docker/colima-context" ]]
}

if CONFIRM_RUNTIME_CLEANUP= FAKE_PROFILE_PRESENT=1 \
  "${repo_root}/platform/local/scripts/cleanup-runtime.sh" </dev/null 2>/dev/null; then
  printf 'cleanup-runtime accepted missing confirmation\n' >&2
  exit 1
fi

: >"${log_file}"
CONFIRM_RUNTIME_CLEANUP=investment-platform \
FAKE_PROFILE_PRESENT=0 \
  "${repo_root}/platform/local/scripts/cleanup-runtime.sh" >/dev/null
if grep -q '^colima delete ' "${log_file}"; then
  printf 'cleanup-runtime deleted an absent profile\n' >&2
  exit 1
fi

: >"${log_file}"
CONFIRM_RUNTIME_CLEANUP=investment-platform \
FAKE_PROFILE_PRESENT=1 \
FAKE_CLUSTER_PRESENT=1 \
  "${repo_root}/platform/local/scripts/cleanup-runtime.sh" >/dev/null
grep -q '^kind delete cluster --name investment-platform$' "${log_file}"
grep -q '^colima delete investment-platform --force --data$' "${log_file}"

: >"${log_file}"
FAKE_PROFILE_NAME=investment-platform-ephemeral \
FAKE_PROFILE_PRESENT=0 \
FAKE_CLUSTER_PRESENT=1 \
DOCKER_CONTEXT=caller-context \
DOCKER_HOST=unix:///caller/docker.sock \
  "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null
grep -q '^make -C .*/platform/local bootstrap$' "${log_file}"
grep -q '^make -C .*/platform/local bootstrap-data-path$' "${log_file}"
if [[ "$(grep -c '^make -C .*/platform/local bootstrap-analytics$' "${log_file}")" != "2" ]]; then
  printf 'run-ephemeral did not rebuild analytics after the scale phase\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local verify-scale-lab$' "${log_file}"
grep -q '^make -C .*/platform/local destroy-analytics$' "${log_file}"
grep -q '^kind delete cluster --name investment-platform$' "${log_file}"
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
EPHEMERAL_WORKFLOW=capacity \
FAKE_PROFILE_NAME=investment-platform-capacity-ephemeral \
FAKE_PROFILE_PRESENT=0 \
FAKE_CLUSTER_PRESENT=1 \
DOCKER_CONTEXT=caller-context \
DOCKER_HOST=unix:///caller/docker.sock \
  "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null
grep -q '^make -C .*/platform/local bootstrap$' "${log_file}"
grep -q '^make -C .*/platform/local bootstrap-data-path$' "${log_file}"
grep -q '^make -C .*/platform/local verify-scale-lab$' "${log_file}"
grep -q '^make -C .*/platform/local verify-capacity-benchmark$' "${log_file}"
if grep -q '^make -C .*/platform/local bootstrap-analytics$' "${log_file}"; then
  printf 'capacity workflow bootstrapped unrelated analytics services\n' >&2
  exit 1
fi
grep -q '^kind delete cluster --name investment-platform$' "${log_file}"
grep -q '^colima delete investment-platform-capacity-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if EPHEMERAL_WORKFLOW=capacity \
  FAKE_PROFILE_NAME=investment-platform-capacity-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_TARGET=verify-capacity-benchmark \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'capacity workflow ignored a failed benchmark\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local verify-capacity-benchmark$' "${log_file}"
grep -q '^make -C .*/platform/local diagnose-data-path$' "${log_file}"
if grep -q '^make -C .*/platform/local diagnose-analytics$' "${log_file}"; then
  printf 'capacity workflow diagnosed analytics that it did not install\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-capacity-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_TARGET=bootstrap-data-path \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral ignored a failed platform phase\n' >&2
  exit 1
fi
if grep -q '^make -C .*/platform/local bootstrap-analytics$' "${log_file}"; then
  printf 'run-ephemeral continued after a failed platform phase\n' >&2
  exit 1
fi
if grep -q '^make -C .*/platform/local verify-scale-lab$' "${log_file}"; then
  printf 'run-ephemeral continued to the scale lab after a failed platform phase\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local diagnose$' "${log_file}"
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_TARGET=verify-scale-lab \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral ignored a failed scale verification phase\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local verify-scale-lab$' "${log_file}"
if grep -q '^make -C .*/platform/local destroy-analytics$' "${log_file}"; then
  printf 'run-ephemeral continued after a failed scale verification phase\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local diagnose-data-path$' "${log_file}"
grep -q '^kind delete cluster --name investment-platform$' "${log_file}"
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_TARGET=destroy-analytics \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral ignored a failed analytics isolation phase\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local verify-scale-lab$' "${log_file}"
grep -q '^make -C .*/platform/local destroy-analytics$' "${log_file}"
if [[ "$(grep -c '^make -C .*/platform/local bootstrap-analytics$' "${log_file}")" != "1" ]]; then
  printf 'run-ephemeral continued after analytics isolation failed\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local diagnose-analytics$' "${log_file}"
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-ephemeral \
  FAKE_PROFILE_PRESENT=1 \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral accepted a pre-existing profile\n' >&2
  exit 1
fi
if grep -q '^colima delete ' "${log_file}"; then
  printf 'run-ephemeral deleted a profile it did not create\n' >&2
  exit 1
fi

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_COLIMA_START=1 \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral ignored a failed Colima start\n' >&2
  exit 1
fi
if grep -q '^kind delete ' "${log_file}"; then
  printf 'run-ephemeral targeted another Docker daemon after a failed Colima start\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
FAKE_PROFILE_PRESENT=0 \
DOCKER_CONTEXT=caller-context \
DOCKER_HOST=unix:///caller/docker.sock \
  "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null
grep -Eq '^caffeinate -dimsu -w [0-9]+$' "${log_file}"
grep -q '^make -C .*/platform/jenkins bootstrap$' "${log_file}"
grep -q '^make -C .*/platform/jenkins trigger$' "${log_file}"
grep -q '^colima delete investment-platform-jenkins-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_FAIL_TARGET=trigger \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Jenkins run-ephemeral ignored a failed phase\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-jenkins-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
block_ready="${test_root}/jenkins-block-ready"
block_release="${test_root}/jenkins-block-release"
FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
FAKE_PROFILE_PRESENT=0 \
FAKE_BLOCK_TARGET=trigger \
FAKE_BLOCK_READY="${block_ready}" \
FAKE_BLOCK_RELEASE="${block_release}" \
DOCKER_CONTEXT=caller-context \
DOCKER_HOST=unix:///caller/docker.sock \
  "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1 &
jenkins_pid=$!
for _ in {1..100}; do
  [[ -e "${block_ready}" ]] && break
  sleep 0.05
done
if [[ ! -e "${block_ready}" ]]; then
  printf 'Jenkins run-ephemeral did not reach the blocking phase\n' >&2
  exit 1
fi
kill -TERM "${jenkins_pid}"
: >"${block_release}"
set +e
wait "${jenkins_pid}"
jenkins_status=$?
set -e
if [[ "${jenkins_status}" != 143 ]]; then
  printf 'Jenkins run-ephemeral did not preserve TERM status\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-jenkins-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

printf 'Ephemeral-runtime lifecycle and context-isolation tests passed.\n'
