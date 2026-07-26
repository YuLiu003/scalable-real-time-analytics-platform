#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
python_bin="${PYTHON_BIN:-python3}"
coverage_file="${PRESUBMIT_COVERAGE_FILE:-${TMPDIR:-/tmp}/presubmit.coverage}"

export COVERAGE_FILE="${coverage_file}"
export PYTHONPATH="${repo_root}/scripts/ci"

"${python_bin}" -m coverage erase
"${python_bin}" -m coverage run \
  --branch \
  --source=presubmit \
  -m unittest discover \
  --start-directory "${repo_root}/scripts/ci/tests" \
  --pattern 'test_*.py'
"${python_bin}" -m coverage report --fail-under=100 --show-missing
