#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC1091
source "${script_dir}/../versions.lock"

for command_name in curl docker git kind kubectl helm make openssl python3; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: required command %s was not found.\n' "${command_name}" >&2
    exit 1
  fi
done

installed_kind_version="$(kind version | awk '{print $2}')"
if [[ "${installed_kind_version}" != "${KIND_VERSION}" ]]; then
  printf 'ERROR: kind %s is installed; this baseline requires %s.\n' \
    "${installed_kind_version}" "${KIND_VERSION}" >&2
  exit 1
fi

installed_helm_version="$(helm version --short 2>/dev/null | sed 's/+.*//')"
if [[ ! "${installed_helm_version}" =~ ^v3\. ]]; then
  printf 'ERROR: Helm 3 is required; found %s.\n' \
    "${installed_helm_version:-unknown}" >&2
  exit 1
fi

docker info >/dev/null
