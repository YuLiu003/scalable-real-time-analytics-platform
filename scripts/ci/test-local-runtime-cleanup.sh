#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/runtime-cleanup-test.XXXXXX")"
test_root="$(cd "${test_root}" && pwd -P)"
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
      if [[ "${FAKE_FAIL_COLIMA_LIST:-0}" == "1" ]]; then
        exit 1
      fi
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
    elif [[ "${1:-}" == "delete" && "${FAKE_FAIL_COLIMA_DELETE:-0}" == "1" ]]; then
      exit 1
    fi
    ;;
  docker)
    case "${1:-}" in
      info)
        if [[ "${2:-}" == "--format" ]]; then
          printf 'arm64\n'
        fi
        ;;
      build | pull | tag | run) ;;
      *) exit 1 ;;
    esac
    ;;
  git)
    if [[ "${1:-}" == "-C" && "${3:-}" == "rev-parse" ]]; then
      printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n'
    fi
    ;;
  kind)
    if [[ "${1:-}" == "version" ]]; then
      printf 'kind v0.31.0\n'
    elif [[ "${1:-}" == "get" && "${2:-}" == "clusters" ]]; then
      if [[ "${FAKE_FAIL_KIND_LIST:-0}" == "1" ]]; then
        exit 1
      fi
      if [[ "${FAKE_CLUSTER_PRESENT:-0}" == "1" ]] ||
        [[ -n "${FAKE_KIND_CREATED_MARKER:-}" && -e "${FAKE_KIND_CREATED_MARKER}" ]]; then
        printf '%s\n' "${FAKE_CLUSTER_NAME:-investment-platform}"
      fi
    elif [[ "${1:-}" == "create" && "${2:-}" == "cluster" &&
      "${FAKE_KIND_CREATE_PARTIAL:-0}" == "1" ]]; then
      : >"${FAKE_KIND_CREATED_MARKER}"
      exit 1
    elif [[ "${1:-}" == "delete" && "${2:-}" == "cluster" ]]; then
      if [[ "${FAKE_FAIL_KIND_DELETE:-0}" == "1" ]]; then
        exit 1
      fi
      if [[ -n "${FAKE_KIND_CREATED_MARKER:-}" ]]; then
        rm -f "${FAKE_KIND_CREATED_MARKER}"
      fi
    fi
    ;;
  kubectl)
    if [[ " $* " == *" get secret jenkins-retained-lock-owner "* ]]; then
      printf '%s' "${FAKE_RETAINED_LOCK_TOKEN:-}"
    elif [[ " $* " == *" get namespace ${FAKE_JENKINS_NAMESPACE:-jenkins-system} "* ]]; then
      if [[ "${FAKE_FAIL_NAMESPACE_LOOKUP:-0}" == "1" ]]; then
        exit 1
      fi
      if [[ "${FAKE_NAMESPACE_PRESENT:-1}" == "1" ]]; then
        printf 'namespace/%s\n' "${FAKE_JENKINS_NAMESPACE:-jenkins-system}"
      fi
    elif [[ " $* " == *" get statefulset "* ]]; then
      if [[ "${FAKE_FAIL_WORKLOAD_LOOKUP:-0}" == "1" ]]; then
        exit 1
      fi
      if [[ "${FAKE_WORKLOADS_PRESENT:-1}" == "1" ]]; then
        for ((index = 1; index <= $#; index += 1)); do
          if [[ "${!index}" == "statefulset" ]]; then
            name_index=$((index + 1))
            printf 'statefulset.apps/%s\n' "${!name_index}"
            break
          fi
        done
      fi
    elif [[ " $* " == *" get pods "* ]]; then
      if [[ "${FAKE_FAIL_POD_LIST:-0}" == "1" ]]; then
        exit 1
      fi
      if [[ "${FAKE_PODS_PRESENT:-0}" == "1" ]]; then
        printf 'pod/fake-jenkins-pod\n'
      fi
    elif [[ " $* " == *" scale statefulset/"* ]]; then
      if [[ "${FAKE_FAIL_QUIESCE_STAGE:-}" == "scale" ]]; then
        exit 1
      fi
    elif [[ " $* " == *" wait pod "* ]]; then
      if [[ "${FAKE_FAIL_QUIESCE_STAGE:-}" == "wait" ]]; then
        exit 1
      fi
    fi
    ;;
  helm)
    if [[ "${1:-}" == "version" && "${2:-}" == "--short" ]]; then
      printf 'v3.20.0+fake\n'
    fi
    ;;
  openssl)
    if [[ "${1:-}" == "rand" && "${2:-}" == "-hex" ]]; then
      printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n'
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

assert_jenkins_quiesce_failure_retains_runtime() {
  local case_name="$1"
  shift
  local profile="investment-platform-jenkins-${case_name}"
  local state="${test_root}/${case_name}-state"

  : >"${log_file}"
  if env \
    JENKINS_COLIMA_PROFILE="${profile}" \
    JENKINS_RETAINED_STATE_DIR="${state}" \
    FAKE_PROFILE_NAME="${profile}" \
    FAKE_PROFILE_PRESENT=0 \
    FAKE_CLUSTER_NAME=jenkins-platform \
    FAKE_CLUSTER_PRESENT=1 \
    JENKINS_STORAGE_METRICS_ENABLED=false \
    DOCKER_CONTEXT=caller-context \
    DOCKER_HOST=unix:///caller/docker.sock \
    "$@" \
      "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
    printf 'Jenkins run-ephemeral accepted %s quiesce failure\n' "${case_name}" >&2
    exit 1
  fi
  if grep -q '^kind delete cluster --name jenkins-platform$' "${log_file}" ||
    grep -q "^colima delete ${profile} " "${log_file}"; then
    printf 'Jenkins run-ephemeral deleted runtime after %s quiesce failure\n' \
      "${case_name}" >&2
    exit 1
  fi
  [[ -d "${state}/.active-lock" ]]
  [[ -d "${HOME}/.colima/.investment-platform-locks/${profile}" ]]
  assert_runtime_config_isolated_and_removed
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
grep -Eq '^caffeinate -dimsu -w [0-9]+$' "${log_file}"
if grep -q '^make -C .*/platform/local bootstrap-analytics$' "${log_file}"; then
  printf 'capacity workflow bootstrapped unrelated analytics services\n' >&2
  exit 1
fi
grep -q '^kind delete cluster --name investment-platform$' "${log_file}"
grep -q '^colima delete investment-platform-capacity-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if EPHEMERAL_WORKFLOW=capacity \
  EPHEMERAL_COLIMA_PROFILE=investment-platform-capacity-ephemeral \
  FAKE_PROFILE_NAME=investment-platform \
  FAKE_PROFILE_PRESENT=1 \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'capacity workflow accepted another running Colima profile\n' >&2
  exit 1
fi
if grep -q '^colima start ' "${log_file}" || grep -q '^colima delete ' "${log_file}"; then
  printf 'capacity isolation failure mutated a Colima profile\n' >&2
  exit 1
fi

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
FAKE_NAMESPACE_PRESENT=0 \
  "${repo_root}/platform/jenkins/scripts/quiesce.sh"
if grep -q ' scale statefulset/' "${log_file}"; then
  printf 'Jenkins quiesce scaled workloads in an absent namespace\n' >&2
  exit 1
fi

: >"${log_file}"
FAKE_NAMESPACE_PRESENT=1 \
FAKE_WORKLOADS_PRESENT=0 \
FAKE_PODS_PRESENT=0 \
  "${repo_root}/platform/jenkins/scripts/quiesce.sh"
if grep -q ' scale statefulset/' "${log_file}"; then
  printf 'Jenkins quiesce scaled absent retained workloads\n' >&2
  exit 1
fi

: >"${log_file}"
FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
FAKE_PROFILE_PRESENT=0 \
FAKE_CLUSTER_NAME=jenkins-platform \
FAKE_CLUSTER_PRESENT=1 \
JENKINS_STORAGE_METRICS_ENABLED=false \
DOCKER_CONTEXT=caller-context \
DOCKER_HOST=unix:///caller/docker.sock \
  "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null
grep -Eq '^caffeinate -dimsu -w [0-9]+$' "${log_file}"
grep -q '^make -C .*/platform/jenkins bootstrap$' "${log_file}"
grep -q '^make -C .*/platform/jenkins trigger$' "${log_file}"
jenkins_scale_line="$(grep -n 'kubectl --context kind-jenkins-platform --namespace jenkins-system scale statefulset/jenkins --replicas=0' "${log_file}" | cut -d: -f1)"
garage_scale_line="$(grep -n 'kubectl --context kind-jenkins-platform --namespace jenkins-system scale statefulset/jenkins-artifacts --replicas=0' "${log_file}" | cut -d: -f1)"
kind_delete_line="$(grep -n '^kind delete cluster --name jenkins-platform$' "${log_file}" | cut -d: -f1)"
if (( jenkins_scale_line >= garage_scale_line || garage_scale_line >= kind_delete_line )); then
  printf 'Jenkins cleanup did not quiesce retained workloads before kind deletion\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-jenkins-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_TARGET=trigger \
  JENKINS_STORAGE_METRICS_ENABLED=false \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Jenkins run-ephemeral ignored a failed phase\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-jenkins-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  JENKINS_STORAGE_METRICS_ENABLED=true \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Jenkins run-ephemeral accepted a failed storage peak monitor\n' >&2
  exit 1
fi
if grep -q '^make -C .*/platform/jenkins trigger$' "${log_file}"; then
  printf 'Jenkins run-ephemeral triggered a build after its storage monitor failed\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-jenkins-ephemeral --force --data$' "${log_file}"
assert_runtime_config_isolated_and_removed

: >"${log_file}"
block_ready="${test_root}/jenkins-block-ready"
block_release="${test_root}/jenkins-block-release"
FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
FAKE_PROFILE_PRESENT=0 \
FAKE_CLUSTER_NAME=jenkins-platform \
FAKE_CLUSTER_PRESENT=1 \
FAKE_BLOCK_TARGET=trigger \
JENKINS_STORAGE_METRICS_ENABLED=false \
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
if JENKINS_COLIMA_PROFILE=investment-platform-jenkins-secondary \
  FAKE_PROFILE_NAME=investment-platform-jenkins-secondary \
  FAKE_PROFILE_PRESENT=0 \
  JENKINS_STORAGE_METRICS_ENABLED=false \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Concurrent Jenkins run acquired retained state already in use\n' >&2
  exit 1
fi
if grep -q '^colima start investment-platform-jenkins-secondary ' "${log_file}"; then
  printf 'Concurrent Jenkins run started a second retained-state writer\n' >&2
  exit 1
fi
if JENKINS_RETAINED_STATE_DIR="${test_root}/other-jenkins-state" \
  FAKE_PROFILE_NAME=investment-platform-jenkins-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  JENKINS_STORAGE_METRICS_ENABLED=false \
  DOCKER_CONTEXT=caller-context \
  DOCKER_HOST=unix:///caller/docker.sock \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Concurrent Jenkins run acquired a Colima profile already in use\n' >&2
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

assert_jenkins_quiesce_failure_retains_runtime \
  namespace-error FAKE_FAIL_NAMESPACE_LOOKUP=1
assert_jenkins_quiesce_failure_retains_runtime \
  workload-lookup-error FAKE_FAIL_WORKLOAD_LOOKUP=1
assert_jenkins_quiesce_failure_retains_runtime \
  pod-list-error FAKE_FAIL_POD_LIST=1
assert_jenkins_quiesce_failure_retains_runtime \
  scale-error FAKE_FAIL_QUIESCE_STAGE=scale
assert_jenkins_quiesce_failure_retains_runtime \
  wait-error FAKE_PODS_PRESENT=1 FAKE_FAIL_QUIESCE_STAGE=wait

owner_state="${test_root}/owner-mismatch-state"
owner_token="$(JENKINS_RETAINED_STATE_DIR="${owner_state}" \
  "${repo_root}/platform/jenkins/scripts/retained-state.sh" acquire)"
wrong_owner_token=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
if JENKINS_RETAINED_STATE_DIR="${owner_state}" \
  JENKINS_RETAINED_LOCK_TOKEN="${wrong_owner_token}" \
    "${repo_root}/platform/jenkins/scripts/retained-state.sh" prepare >/dev/null 2>&1; then
  printf 'Retained-state preparation accepted another owner token\n' >&2
  exit 1
fi
JENKINS_RETAINED_STATE_DIR="${owner_state}" \
JENKINS_RETAINED_LOCK_TOKEN="${owner_token}" \
  "${repo_root}/platform/jenkins/scripts/retained-state.sh" prepare >/dev/null
: >"${log_file}"
if FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_RETAINED_LOCK_TOKEN="${owner_token}" \
  JENKINS_RETAINED_STATE_DIR="${owner_state}" \
  JENKINS_RETAINED_LOCK_TOKEN="${wrong_owner_token}" \
    "${repo_root}/platform/jenkins/scripts/destroy.sh" >/dev/null 2>&1; then
  printf 'Jenkins destroy released another owner token\n' >&2
  exit 1
fi
[[ -d "${owner_state}/.active-lock" ]]
if grep -q '^kind delete cluster --name jenkins-platform$' "${log_file}"; then
  printf 'Jenkins destroy mutated a cluster before owner validation\n' >&2
  exit 1
fi
FAKE_CLUSTER_NAME=jenkins-platform \
FAKE_CLUSTER_PRESENT=1 \
FAKE_RETAINED_LOCK_TOKEN="${owner_token}" \
JENKINS_RETAINED_STATE_DIR="${owner_state}" \
  "${repo_root}/platform/jenkins/scripts/destroy.sh" >/dev/null
grep -q '^kind delete cluster --name jenkins-platform$' "${log_file}"
[[ ! -e "${owner_state}/.active-lock" ]]

bootstrap_failure_state="${test_root}/bootstrap-failure-state"
if FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=1 \
  JENKINS_RETAINED_STATE_DIR="${bootstrap_failure_state}" \
    "${repo_root}/platform/jenkins/scripts/bootstrap.sh" >/dev/null 2>&1; then
  printf 'Jenkins bootstrap accepted a pre-existing cluster\n' >&2
  exit 1
fi
if [[ -e "${bootstrap_failure_state}/.active-lock" ]]; then
  printf 'Failed standalone Jenkins bootstrap stranded its owned lock\n' >&2
  exit 1
fi

bootstrap_list_failure_state="${test_root}/bootstrap-list-failure-state"
: >"${log_file}"
if FAKE_FAIL_KIND_LIST=1 \
  JENKINS_RETAINED_STATE_DIR="${bootstrap_list_failure_state}" \
    "${repo_root}/platform/jenkins/scripts/bootstrap.sh" >/dev/null 2>&1; then
  printf 'Jenkins bootstrap ignored a failed cluster listing\n' >&2
  exit 1
fi
if grep -q '^kind create cluster ' "${log_file}"; then
  printf 'Jenkins bootstrap created a cluster after list failure\n' >&2
  exit 1
fi
[[ ! -e "${bootstrap_list_failure_state}/.active-lock" ]]

partial_cluster_marker="${test_root}/partial-kind-cluster"
partial_bootstrap_state="${test_root}/partial-bootstrap-state"
: >"${log_file}"
if FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=0 \
  FAKE_KIND_CREATE_PARTIAL=1 \
  FAKE_KIND_CREATED_MARKER="${partial_cluster_marker}" \
  JENKINS_RETAINED_STATE_DIR="${partial_bootstrap_state}" \
    "${repo_root}/platform/jenkins/scripts/bootstrap.sh" >/dev/null 2>&1; then
  printf 'Jenkins bootstrap accepted a partial kind creation\n' >&2
  exit 1
fi
grep -q '^kind delete cluster --name jenkins-platform$' "${log_file}"
[[ ! -e "${partial_cluster_marker}" ]]
[[ ! -e "${partial_bootstrap_state}/.active-lock" ]]

failed_bootstrap_quiesce_marker="${test_root}/failed-bootstrap-quiesce-cluster"
failed_bootstrap_quiesce_state="${test_root}/failed-bootstrap-quiesce-state"
: >"${log_file}"
if FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=0 \
  FAKE_KIND_CREATE_PARTIAL=1 \
  FAKE_KIND_CREATED_MARKER="${failed_bootstrap_quiesce_marker}" \
  FAKE_FAIL_NAMESPACE_LOOKUP=1 \
  JENKINS_RETAINED_STATE_DIR="${failed_bootstrap_quiesce_state}" \
    "${repo_root}/platform/jenkins/scripts/bootstrap.sh" >/dev/null 2>&1; then
  printf 'Jenkins bootstrap accepted a partial cluster with failed quiesce\n' >&2
  exit 1
fi
if grep -q '^kind delete cluster --name jenkins-platform$' "${log_file}"; then
  printf 'Jenkins bootstrap deleted a partially quiesced cluster\n' >&2
  exit 1
fi
[[ -e "${failed_bootstrap_quiesce_marker}" ]]
[[ -d "${failed_bootstrap_quiesce_state}/.active-lock" ]]

failed_kind_delete_marker="${test_root}/failed-kind-delete-cluster"
failed_kind_delete_state="${test_root}/failed-kind-delete-state"
: >"${log_file}"
if FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=0 \
  FAKE_KIND_CREATE_PARTIAL=1 \
  FAKE_KIND_CREATED_MARKER="${failed_kind_delete_marker}" \
  FAKE_FAIL_KIND_DELETE=1 \
  JENKINS_RETAINED_STATE_DIR="${failed_kind_delete_state}" \
    "${repo_root}/platform/jenkins/scripts/bootstrap.sh" >/dev/null 2>&1; then
  printf 'Jenkins bootstrap accepted a partial kind creation with failed cleanup\n' >&2
  exit 1
fi
[[ -e "${failed_kind_delete_marker}" ]]
[[ -d "${failed_kind_delete_state}/.active-lock" ]]

list_failure_profile=investment-platform-jenkins-list-failure
list_failure_state="${test_root}/list-failure-state"
: >"${log_file}"
if JENKINS_COLIMA_PROFILE="${list_failure_profile}" \
  JENKINS_RETAINED_STATE_DIR="${list_failure_state}" \
  FAKE_FAIL_COLIMA_LIST=1 \
  JENKINS_STORAGE_METRICS_ENABLED=false \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Jenkins run-ephemeral ignored a failed Colima profile listing\n' >&2
  exit 1
fi
[[ ! -e "${list_failure_state}/.active-lock" ]]
[[ ! -e "${HOME}/.colima/.investment-platform-locks/${list_failure_profile}" ]]
if grep -q "^colima start ${list_failure_profile} " "${log_file}" ||
  grep -q "^colima delete ${list_failure_profile} " "${log_file}"; then
  printf 'Jenkins run-ephemeral mutated a profile after list failure\n' >&2
  exit 1
fi

delete_failure_profile=investment-platform-jenkins-delete-failure
delete_failure_state="${test_root}/delete-failure-state"
: >"${log_file}"
if JENKINS_COLIMA_PROFILE="${delete_failure_profile}" \
  JENKINS_RETAINED_STATE_DIR="${delete_failure_state}" \
  FAKE_PROFILE_NAME="${delete_failure_profile}" \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_NAME=jenkins-platform \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_COLIMA_DELETE=1 \
  JENKINS_STORAGE_METRICS_ENABLED=false \
    "${repo_root}/platform/jenkins/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'Jenkins run-ephemeral ignored failed Colima deletion\n' >&2
  exit 1
fi
grep -q "^colima delete ${delete_failure_profile} --force --data$" "${log_file}"
[[ -d "${delete_failure_state}/.active-lock" ]]
[[ -d "${HOME}/.colima/.investment-platform-locks/${delete_failure_profile}" ]]
assert_runtime_config_isolated_and_removed

printf 'Ephemeral-runtime lifecycle and context-isolation tests passed.\n'
