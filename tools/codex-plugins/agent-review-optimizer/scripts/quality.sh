#!/usr/bin/env bash
set -euo pipefail

PLUGIN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_BIN="${PYTHON_BIN:-python3}"

if [[ -z "${COVERAGE_FILE:-}" ]]; then
  COVERAGE_FILE="${TMPDIR:-/tmp}/agent-review-optimizer-coverage.$$"
  trap 'rm -f "$COVERAGE_FILE"' EXIT
fi

export COVERAGE_FILE
export PYTHONPATH="${PLUGIN_ROOT}"
export PYTHONDONTWRITEBYTECODE=1

"${PYTHON_BIN}" -m coverage erase
"${PYTHON_BIN}" -m coverage run \
  --branch \
  --source=agent_review_optimizer \
  --omit='*/__main__.py' \
  -m unittest discover \
  --start-directory "${PLUGIN_ROOT}/tests" \
  --pattern 'test_*.py'
"${PYTHON_BIN}" -m coverage report --fail-under=100 --show-missing
