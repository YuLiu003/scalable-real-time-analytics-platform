#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
profile="${JENKINS_COLIMA_PROFILE:-investment-platform-jenkins-ephemeral}"
docker_config=""
profile_reserved=0

if [[ ! "${profile}" =~ ^investment-platform-jenkins-[a-z0-9][a-z0-9-]*$ ]]; then
  printf 'ERROR: invalid Jenkins Colima profile name.\n' >&2
  exit 2
fi
if colima list 2>/dev/null | awk 'NR > 1 { print $1 }' | grep -Fxq "${profile}"; then
  printf 'ERROR: refusing to reuse or delete existing profile %s.\n' "${profile}" >&2
  exit 1
fi

cleanup() {
  status="${1:-$?}"
  trap - EXIT HUP INT TERM
  if (( profile_reserved != 0 )); then
    colima delete "${profile}" --force --data || status=1
  fi
  if [[ -n "${docker_config}" ]]; then
    rm -rf "${docker_config}"
  fi
  exit "${status}"
}
trap 'cleanup $?' EXIT
trap 'cleanup 129' HUP
trap 'cleanup 130' INT
trap 'cleanup 143' TERM

profile_reserved=1
docker_config="$(mktemp -d "${TMPDIR:-/tmp}/jenkins-docker-config.XXXXXX")"
colima start "${profile}" \
  --activate=false \
  --runtime docker \
  --cpus "${JENKINS_COLIMA_CPUS:-8}" \
  --memory "${JENKINS_COLIMA_MEMORY_GIB:-16}" \
  --disk "${JENKINS_COLIMA_DISK_GIB:-50}"

export DOCKER_HOST="unix://${HOME}/.colima/${profile}/docker.sock"
export DOCKER_CONFIG="${docker_config}"

make -C "${jenkins_dir}" bootstrap
make -C "${jenkins_dir}" trigger

printf 'Disposable Jenkins production-like validation passed; deleting %s.\n' "${profile}"
