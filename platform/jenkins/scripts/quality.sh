#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
python_bin="${PYTHON_BIN:-python3}"

bash -n "${jenkins_dir}"/scripts/*.sh
"${python_bin}" "${jenkins_dir}/tests/test_contract.py"

printf 'Jenkins platform contract checks passed.\n'
