#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/capacity-observability-test.XXXXXX")"
trap 'rm -rf "${test_root}"' EXIT
mkdir -p "${test_root}/bin"

cat >"${test_root}/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' "$*" >>"${FAKE_LOG}"
if [[ "$*" == *" get pods "* ]]; then
  printf '%s\n' "${FAKE_PODS}"
  exit 0
fi
count=0
if [[ -f "${FAKE_COUNT}" ]]; then
  count="$(<"${FAKE_COUNT}")"
fi
count=$((count + 1))
printf '%d\n' "${count}" >"${FAKE_COUNT}"
if [[ "${FAKE_MODE}" == "retry" && "${count}" == "1" ]]; then
  printf '{"partial":'
  exit 1
fi
if [[ "${FAKE_MODE}" == "retry" ]] && ! grep -Fxq 'stale' "${FAKE_PROTECTED_OUTPUT}"; then
  printf 'Prometheus retry exposed a partial response\n' >&2
  exit 1
fi
if [[ "${FAKE_MODE}" == "invalid" ]]; then
  printf '{"status":"success","data":{"result":[]}}\n'
  exit 0
fi
printf '{"status":"success","data":{"result":[{"metric":{"outcome":"created"},"value":[1,"2"]}]}}\n'
FAKE
chmod +x "${test_root}/bin/kubectl"

export PATH="${test_root}/bin:${PATH}"
export FAKE_LOG="${test_root}/kubectl.log"
export FAKE_COUNT="${test_root}/count"
# shellcheck disable=SC1091
source "${repo_root}/platform/local/scripts/capacity-observability-lib.sh"

capacity_duration_supports_window 100 125.001 25
if capacity_duration_supports_window 100 125 25; then
  printf 'equal resource window boundary was accepted\n' >&2
  exit 1
fi
if capacity_duration_supports_window 100 124.999 25; then
  printf 'short resource window boundary was accepted\n' >&2
  exit 1
fi

output="${test_root}/vector.json"
printf 'stale\n' >"${output}"
export FAKE_MODE=retry
export FAKE_PROTECTED_OUTPUT="${output}"
capacity_prometheus_query_to_file kind-test observability query outcome "${output}" 2 0 1
grep -Fq '"outcome":"created"' "${output}"
[[ "$(<"${FAKE_COUNT}")" == "2" ]]
grep -Eq -- '--request-timeout=[1-9][0-9]*s get --raw' "${FAKE_LOG}"
if find "${test_root}" -name 'vector.json.tmp.*' -print -quit | grep -q .; then
  printf 'successful Prometheus query left a staged output behind\n' >&2
  exit 1
fi

printf 'preserve\n' >"${output}"
: >"${FAKE_COUNT}"
export FAKE_MODE=invalid
if capacity_prometheus_query_to_file kind-test observability query outcome "${output}" 1 1 1 2>/dev/null; then
  printf 'invalid Prometheus vectors were accepted\n' >&2
  exit 1
fi
grep -Fxq 'preserve' "${output}"
if find "${test_root}" -name 'vector.json.tmp.*' -print -quit | grep -q .; then
  printf 'failed Prometheus query left a staged output behind\n' >&2
  exit 1
fi

export FAKE_PODS=$'z/pod|uid-z|app:0,\na/pod|uid-a|app:0,'
baseline="${test_root}/pods-before.txt"
observed="${test_root}/pods-after.txt"
capacity_capture_pod_stability kind-test "${baseline}"
[[ "$(head -n 1 "${baseline}")" == 'a/pod|uid-a|app:0,' ]]
cp "${baseline}" "${observed}"
capacity_assert_pod_stability "${baseline}" "${observed}"
export FAKE_PODS=$'z/pod|uid-z|app:1,\na/pod|uid-a|app:0,'
capacity_capture_pod_stability kind-test "${observed}"
if capacity_assert_pod_stability "${baseline}" "${observed}" 2>/dev/null; then
  printf 'pod restart was accepted as stable capacity evidence\n' >&2
  exit 1
fi
export FAKE_PODS=''
if capacity_capture_pod_stability kind-test "${observed}" 2>/dev/null; then
  printf 'empty pod snapshot was accepted\n' >&2
  exit 1
fi

printf 'Capacity observability retry and stability tests passed.\n'
