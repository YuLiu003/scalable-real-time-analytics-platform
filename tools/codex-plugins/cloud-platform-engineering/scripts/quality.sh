#!/usr/bin/env bash
set -euo pipefail

PLUGIN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_BIN="${PYTHON_BIN:-python3}"

if [[ -z "${COVERAGE_FILE:-}" ]]; then
  COVERAGE_FILE="${TMPDIR:-/tmp}/cloud-platform-engineering-coverage.$$"
  trap 'rm -f "$COVERAGE_FILE"' EXIT
fi

export COVERAGE_FILE
export PYTHONPATH="$PLUGIN_ROOT/mcp"

"$PYTHON_BIN" -m coverage erase
"$PYTHON_BIN" -m coverage run \
  --branch \
  --source=server \
  -m unittest discover \
  --start-directory "$PLUGIN_ROOT/mcp" \
  --pattern 'test_*.py'
"$PYTHON_BIN" -m coverage report --fail-under=100 --show-missing
