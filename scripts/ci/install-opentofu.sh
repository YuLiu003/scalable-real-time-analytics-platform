#!/usr/bin/env bash
set -Eeuo pipefail

if (( $# != 1 )); then
  printf 'Usage: %s OUTPUT_DIRECTORY\n' "$0" >&2
  exit 2
fi

if [[ "$(uname -s)" != "Linux" || "$(uname -m)" != "x86_64" ]]; then
  printf 'ERROR: this CI installer currently supports Linux amd64 only.\n' >&2
  exit 1
fi

install_dir="$1"
version="1.12.3"
archive="tofu_${version}_linux_amd64.zip"
expected_sha256="46b48c3438c65cf479fc076c9281422ffa2f493548d1e813d154c835c5986a08"
download_dir="$(mktemp -d)"
trap 'rm -rf "${download_dir}"' EXIT

mkdir -p "${install_dir}"
curl --fail --silent --show-error --location \
  "https://github.com/opentofu/opentofu/releases/download/v${version}/${archive}" \
  --output "${download_dir}/${archive}"

printf '%s  %s\n' "${expected_sha256}" "${download_dir}/${archive}" | sha256sum --check --status
unzip -q "${download_dir}/${archive}" -d "${download_dir}"
install -m 0755 "${download_dir}/tofu" "${install_dir}/tofu"
"${install_dir}/tofu" version
