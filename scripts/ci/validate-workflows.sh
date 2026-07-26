#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
actionlint_version="v1.7.12"
install_dir="${PRESUBMIT_BIN_DIR:-/tmp/presubmit-bin}"
actionlint_bin="${install_dir}/actionlint"

if [[ ! -x "${actionlint_bin}" ]]; then
  mkdir -p "${install_dir}"
  GOBIN="${install_dir}" go install \
    "github.com/rhysd/actionlint/cmd/actionlint@${actionlint_version}"
fi

shopt -s nullglob
workflow_files=(
  "${repo_root}"/.github/workflows/*.yml
  "${repo_root}"/.github/workflows/*.yaml
)
if (( ${#workflow_files[@]} == 0 )); then
  printf 'ERROR: no GitHub workflows found.\n' >&2
  exit 1
fi

"${actionlint_bin}" "${workflow_files[@]}"
