#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/runtime-cleanup-test.XXXXXX")"
fake_bin="${test_root}/bin"
fake_home="${test_root}/home"
log_file="${test_root}/commands.log"
trap 'rm -rf "${test_root}"' EXIT
mkdir -p "${fake_bin}" "${fake_home}"

cat >"${fake_bin}/fake-command" <<'FAKE'
#!/usr/bin/env bash
set -Eeuo pipefail
command_name="$(basename "$0")"
printf '%s %s\n' "${command_name}" "$*" >>"${FAKE_LOG}"
case "${command_name}" in
  colima)
    if [[ "${1:-}" == "list" ]]; then
      printf 'PROFILE STATUS ARCH CPUS MEMORY DISK RUNTIME ADDRESS\n'
      if [[ "${FAKE_PROFILE_PRESENT:-0}" == "1" ]]; then
        printf '%s Running aarch64 4 8GiB 30GiB docker -\n' "${FAKE_PROFILE_NAME}"
      fi
    elif [[ "${1:-}" == "start" && "${FAKE_FAIL_COLIMA_START:-0}" == "1" ]]; then
      exit 1
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
    if [[ -n "${FAKE_FAIL_TARGET:-}" && "${target}" == "${FAKE_FAIL_TARGET}" ]]; then
      exit 1
    fi
    ;;
esac
FAKE
chmod +x "${fake_bin}/fake-command"
for command_name in colima docker kind kubectl helm make; do
  ln -s fake-command "${fake_bin}/${command_name}"
done

export PATH="${fake_bin}:${PATH}"
export HOME="${fake_home}"
export FAKE_LOG="${log_file}"
export FAKE_PROFILE_NAME=investment-platform

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
  "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null
grep -q '^make -C .*/platform/local bootstrap$' "${log_file}"
grep -q '^make -C .*/platform/local bootstrap-data-path$' "${log_file}"
grep -q '^make -C .*/platform/local bootstrap-analytics$' "${log_file}"
grep -q '^kind delete cluster --name investment-platform$' "${log_file}"
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"

: >"${log_file}"
if FAKE_PROFILE_NAME=investment-platform-ephemeral \
  FAKE_PROFILE_PRESENT=0 \
  FAKE_CLUSTER_PRESENT=1 \
  FAKE_FAIL_TARGET=bootstrap-data-path \
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral ignored a failed platform phase\n' >&2
  exit 1
fi
if grep -q '^make -C .*/platform/local bootstrap-analytics$' "${log_file}"; then
  printf 'run-ephemeral continued after a failed platform phase\n' >&2
  exit 1
fi
grep -q '^make -C .*/platform/local diagnose$' "${log_file}"
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"

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
    "${repo_root}/platform/local/scripts/run-ephemeral.sh" >/dev/null 2>&1; then
  printf 'run-ephemeral ignored a failed Colima start\n' >&2
  exit 1
fi
if grep -q '^kind delete ' "${log_file}"; then
  printf 'run-ephemeral targeted another Docker daemon after a failed Colima start\n' >&2
  exit 1
fi
grep -q '^colima delete investment-platform-ephemeral --force --data$' "${log_file}"

printf 'Local ephemeral-runtime lifecycle tests passed.\n'
