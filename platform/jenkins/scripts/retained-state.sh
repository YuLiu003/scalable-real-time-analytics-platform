#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd -P)"
repo_root="$(cd "${jenkins_dir}/../.." && pwd -P)"
state_dir="${JENKINS_RETAINED_STATE_DIR:-${HOME}/.local/share/investment-platform/jenkins}"
marker_value="investment-platform-jenkins-retained-state-v1"

validate_state_dir() {
  if [[ "${state_dir}" != /* ]] || [[ "${state_dir}" == *$'\n'* ]] ||
    [[ "${state_dir}" == *:* ]]; then
    printf 'ERROR: retained Jenkins state path must be an absolute path without colons or newlines.\n' >&2
    exit 2
  fi
  case "/${state_dir#/}/" in
    *"/./"* | *"/../"*)
      printf 'ERROR: retained Jenkins state path must not contain dot components.\n' >&2
      exit 2
      ;;
  esac
  normalized=""
  candidate=""
  while IFS= read -r component; do
    [[ -z "${component}" ]] && continue
    normalized="${normalized}/${component}"
    candidate="${candidate}/${component}"
    if [[ -L "${candidate}" ]]; then
      printf 'ERROR: retained Jenkins state path must not contain symbolic links.\n' >&2
      exit 2
    fi
  done < <(printf '%s\n' "${state_dir#/}" | tr '/' '\n')
  state_dir="${normalized:-/}"
  case "${state_dir}/" in
    "${repo_root}/"*)
      printf 'ERROR: retained Jenkins state must remain outside the repository.\n' >&2
      exit 2
      ;;
  esac
  case "${repo_root}/" in
    "${state_dir%/}/"*)
      printf 'ERROR: retained Jenkins state must not contain the repository.\n' >&2
      exit 2
      ;;
  esac
}

validate_marker() {
  marker="${state_dir}/.jenkins-retained-state"
  [[ ! -L "${marker}" ]] && [[ -f "${marker}" ]] &&
    [[ "$(<"${marker}")" == "${marker_value}" ]]
}

validate_lock_owner() {
  validate_state_dir
  lock="${state_dir}/.active-lock"
  owner_file="${lock}/owner-token"
  owner_token="${JENKINS_RETAINED_LOCK_TOKEN:-}"
  if [[ -L "${lock}" ]] || [[ ! -d "${lock}" ]] ||
    [[ -L "${owner_file}" ]] || [[ ! -f "${owner_file}" ]]; then
    printf 'ERROR: retained Jenkins lock is invalid or unavailable.\n' >&2
    exit 1
  fi
  if [[ ! "${owner_token}" =~ ^[0-9a-f]{64}$ ]] ||
    [[ "$(<"${owner_file}")" != "${owner_token}" ]]; then
    printf 'ERROR: retained Jenkins lock owner token does not match.\n' >&2
    exit 1
  fi
}

acquire_lock() {
  validate_state_dir
  if [[ -e "${state_dir}" && ! -d "${state_dir}" ]]; then
    printf 'ERROR: retained Jenkins state path is not a directory.\n' >&2
    exit 1
  fi
  if [[ -d "${state_dir}" ]] && ! validate_marker; then
    if [[ -n "$(find "${state_dir}" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
      printf 'ERROR: refusing to lock an unmarked retained-state directory.\n' >&2
      exit 1
    fi
  fi
  umask 077
  mkdir -p "${state_dir}"
  chmod 0700 "${state_dir}"
  owner_token="$(openssl rand -hex 32)"
  if [[ ! "${owner_token}" =~ ^[0-9a-f]{64}$ ]]; then
    printf 'ERROR: failed to generate a retained Jenkins lock owner token.\n' >&2
    exit 1
  fi
  if ! mkdir "${state_dir}/.active-lock" 2>/dev/null; then
    printf 'ERROR: retained Jenkins state is already owned by another run.\n' >&2
    exit 1
  fi
  chmod 0700 "${state_dir}/.active-lock"
  if ! printf '%s\n' "${owner_token}" >"${state_dir}/.active-lock/owner-token"; then
    rm -f -- "${state_dir}/.active-lock/owner-token"
    rmdir "${state_dir}/.active-lock" 2>/dev/null || true
    printf 'ERROR: failed to record retained Jenkins lock ownership.\n' >&2
    exit 1
  fi
  chmod 0600 "${state_dir}/.active-lock/owner-token"
  printf '%s\n' "${owner_token}"
}

release_lock() {
  validate_state_dir
  lock="${state_dir}/.active-lock"
  if [[ ! -e "${lock}" && ! -L "${lock}" ]]; then
    return
  fi
  if [[ -L "${lock}" ]] || [[ ! -d "${lock}" ]]; then
    printf 'ERROR: retained Jenkins lock is invalid.\n' >&2
    exit 1
  fi
  validate_lock_owner
  if [[ -n "$(find "${lock}" -mindepth 1 -maxdepth 1 ! -name owner-token -print -quit)" ]]; then
    printf 'ERROR: retained Jenkins lock contains unexpected metadata.\n' >&2
    exit 1
  fi
  rm -- "${lock}/owner-token"
  if ! rmdir "${lock}"; then
    printf 'ERROR: retained Jenkins lock is not empty.\n' >&2
    exit 1
  fi
}

write_secret() {
  name="$1"
  value="$2"
  target="${state_dir}/secrets/${name}"
  if [[ -L "${target}" ]] || [[ -e "${target}" && ! -f "${target}" ]]; then
    printf 'ERROR: retained secret %s is not a regular file.\n' "${name}" >&2
    exit 1
  fi
  if [[ ! -e "${target}" ]]; then
    umask 077
    printf '%s' "${value}" >"${target}"
  fi
  chmod 0600 "${target}"
}

check_size() {
  validate_state_dir
  if [[ ! -d "${state_dir}" ]]; then
    printf 'ERROR: retained Jenkins state directory is unavailable.\n' >&2
    exit 1
  fi
  used_kib="$(du -sk -- "${state_dir}" | awk '{print $1}')"
  if [[ ! "${used_kib}" =~ ^[0-9]+$ ]] || (( used_kib > 4 * 1024 * 1024 )); then
    printf 'ERROR: retained Jenkins state exceeded its 4 GiB local-disk boundary.\n' >&2
    exit 1
  fi
}

prepare() {
  validate_state_dir
  if [[ -e "${state_dir}" ]] && [[ ! -d "${state_dir}" ]]; then
    printf 'ERROR: retained Jenkins state path is not a directory.\n' >&2
    exit 1
  fi
  validate_lock_owner
  umask 077
  if [[ -d "${state_dir}" ]]; then
    marker="${state_dir}/.jenkins-retained-state"
    if [[ -e "${marker}" || -L "${marker}" ]] && ! validate_marker; then
      printf 'ERROR: refusing to reuse an unmarked retained-state directory.\n' >&2
      exit 1
    fi
    if [[ ! -e "${marker}" ]]; then
      printf '%s\n' "${marker_value}" >"${marker}"
    fi
  else
    printf 'ERROR: retained Jenkins state directory is unavailable.\n' >&2
    exit 1
  fi
  for directory in controller garage reports secrets; do
    target="${state_dir}/${directory}"
    if [[ -L "${target}" ]] || [[ -e "${target}" && ! -d "${target}" ]]; then
      printf 'ERROR: retained Jenkins state contains an invalid directory.\n' >&2
      exit 1
    fi
    mkdir -p "${target}"
  done
  chmod 0700 "${state_dir}" "${state_dir}/reports" "${state_dir}/secrets"
  chmod 0770 "${state_dir}/controller" "${state_dir}/garage"
  find "${state_dir}/reports" -type f -name '*.json' -mmin +4320 -delete
  chmod 0600 "${state_dir}/.jenkins-retained-state"

  write_secret rpc-secret "$(openssl rand -hex 32)"
  write_secret admin-token "$(openssl rand -hex 32)"
  write_secret metrics-token "$(openssl rand -hex 32)"
  write_secret admin-access-key "GK$(openssl rand -hex 16)"
  write_secret admin-secret-key "$(openssl rand -hex 32)"
  check_size
  printf '%s\n' "${state_dir}"
}

read_secret() {
  validate_state_dir
  name="${1:-}"
  case "${name}" in
    rpc-secret | admin-token | metrics-token | admin-access-key | admin-secret-key | runtime-access-key | runtime-secret-key) ;;
    *)
      printf 'ERROR: unsupported retained secret name.\n' >&2
      exit 2
      ;;
  esac
  target="${state_dir}/secrets/${name}"
  if [[ -L "${target}" ]] || [[ ! -f "${target}" ]]; then
    printf 'ERROR: retained secret %s is unavailable.\n' "${name}" >&2
    exit 1
  fi
  value="$(<"${target}")"
  case "${name}" in
    *access-key)
      [[ "${value}" =~ ^GK[A-Za-z0-9]{20,64}$ ]] || {
        printf 'ERROR: retained access key has an invalid format.\n' >&2
        exit 1
      }
      ;;
    *)
      [[ "${value}" =~ ^[A-Za-z0-9+/=_-]{32,128}$ ]] || {
        printf 'ERROR: retained secret has an invalid format.\n' >&2
        exit 1
      }
      ;;
  esac
  printf '%s' "${value}"
}

purge() {
  validate_state_dir
  marker="${state_dir}/.jenkins-retained-state"
  if [[ "${CONFIRM_JENKINS_RETAINED_PURGE:-}" != "investment-platform-jenkins" ]]; then
    printf 'ERROR: set CONFIRM_JENKINS_RETAINED_PURGE=investment-platform-jenkins to purge retained state.\n' >&2
    exit 2
  fi
  if [[ -e "${state_dir}/.active-lock" ]] || [[ -L "${state_dir}/.active-lock" ]]; then
    printf 'ERROR: refusing to purge retained state while a run owns it.\n' >&2
    exit 1
  fi
  if [[ -L "${marker}" ]] || [[ ! -f "${marker}" ]] ||
    [[ "$(<"${marker}")" != "${marker_value}" ]]; then
    printf 'ERROR: refusing to purge an unmarked retained-state directory.\n' >&2
    exit 1
  fi
  rm -rf -- "${state_dir}"
  printf 'Purged retained Jenkins state: %s\n' "${state_dir}"
}

case "${1:-}" in
  path)
    validate_state_dir
    printf '%s\n' "${state_dir}"
    ;;
  acquire)
    acquire_lock
    ;;
  release)
    release_lock
    ;;
  verify-owner)
    validate_state_dir
    if [[ -e "${state_dir}/.active-lock" ]] || [[ -L "${state_dir}/.active-lock" ]]; then
      validate_lock_owner
    fi
    ;;
  prepare)
    prepare
    ;;
  check)
    check_size
    ;;
  read)
    read_secret "${2:-}"
    ;;
  purge)
    purge
    ;;
  *)
    printf 'Usage: %s {path|acquire|release|verify-owner|prepare|check|read SECRET_NAME|purge}\n' "$0" >&2
    exit 2
    ;;
esac
