#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
python_bin="${PYTHON_BIN:-python3}"
coverage_file="$(mktemp)"
cleanup() {
  rm -f "${coverage_file}"
}
trap cleanup EXIT

bash -n "${jenkins_dir}"/scripts/*.sh
COVERAGE_FILE="${coverage_file}" "${python_bin}" -m coverage run \
  --branch \
  --source="${jenkins_dir}/scripts,${jenkins_dir}/storage" \
  --omit="${jenkins_dir}/scripts/__pycache__/*" \
  -m unittest discover \
  --start-directory "${jenkins_dir}/tests" \
  --pattern 'test_*.py'
COVERAGE_FILE="${coverage_file}" "${python_bin}" -m coverage report \
  --fail-under=100 \
  --show-missing

printf 'Jenkins platform checks passed with 100%% measured Python coverage.\n'
