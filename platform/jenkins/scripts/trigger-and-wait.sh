#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${jenkins_dir}/../.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

admin_password="$(kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get secret jenkins-admin --output=jsonpath='{.data.jenkins-admin-password}' | base64 --decode)"
port="${JENKINS_LOCAL_PORT:-18080}"
log_file="$(mktemp)"
netrc_file="$(mktemp)"
cookie_file="$(mktemp)"
headers_file="$(mktemp)"
chmod 0600 "${netrc_file}" "${cookie_file}" "${headers_file}"
printf 'machine 127.0.0.1 login admin password %s\n' "${admin_password}" >"${netrc_file}"
base_url="http://127.0.0.1:${port}"
curl_args=(
  --fail-with-body
  --silent
  --show-error
  --netrc-file "${netrc_file}"
  --cookie "${cookie_file}"
  --cookie-jar "${cookie_file}"
)

kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  port-forward service/jenkins "${port}:8080" >"${log_file}" 2>&1 &
port_forward_pid=$!
cleanup() {
  kill "${port_forward_pid}" >/dev/null 2>&1 || true
  rm -f "${log_file}" "${netrc_file}" "${cookie_file}" "${headers_file}"
}
trap cleanup EXIT

ready=0
for _ in {1..60}; do
  if curl "${curl_args[@]}" \
    "${base_url}/api/json" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if (( ready == 0 )); then
  curl "${curl_args[@]}" "${base_url}/whoAmI/api/json" >&2 || true
  cat "${log_file}" >&2
  printf 'ERROR: Jenkins API did not accept the trusted lab administrator.\n' >&2
  exit 1
fi

crumb_json="$(curl "${curl_args[@]}" \
  "${base_url}/crumbIssuer/api/json")"
crumb_field="$(printf '%s' "${crumb_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumbRequestField"])')"
crumb_value="$(printf '%s' "${crumb_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumb"])')"

expected_commit="${JENKINS_EXPECTED_COMMIT:-$(git -C "${repo_root}" rev-parse HEAD)}"
if [[ ! "${expected_commit}" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'ERROR: expected commit must be a 40-character lowercase SHA.\n' >&2
  exit 2
fi

curl "${curl_args[@]}" \
  --header "${crumb_field}: ${crumb_value}" \
  --dump-header "${headers_file}" \
  --output /dev/null \
  --request POST \
  --data-urlencode "EXPECTED_COMMIT=${expected_commit}" \
  "${base_url}/job/investment-platform-presubmit/buildWithParameters"

queue_url="$(awk 'BEGIN { IGNORECASE=1 } /^Location:/ { print $2 }' "${headers_file}" |
  tr -d '\r' | tail -n 1)"
if [[ ! "${queue_url}" =~ ^"${base_url}"/queue/item/[0-9]+/$ ]]; then
  printf 'ERROR: Jenkins returned an invalid queue location: %s\n' "${queue_url}" >&2
  exit 1
fi

build_number=""
for _ in {1..120}; do
  queue_state="$(curl "${curl_args[@]}" "${queue_url}api/json" |
    python3 -c 'import json,sys
item=json.load(sys.stdin)
print("CANCELED" if item.get("cancelled") else item.get("executable", {}).get("number", "QUEUED"))')"
  case "${queue_state}" in
    CANCELED)
      printf 'ERROR: Jenkins canceled the queued pipeline.\n' >&2
      exit 1
      ;;
    QUEUED)
      sleep 2
      ;;
    *)
      build_number="${queue_state}"
      break
      ;;
  esac
done
if [[ ! "${build_number}" =~ ^[0-9]+$ ]]; then
  printf 'ERROR: Jenkins did not start the queued pipeline within four minutes.\n' >&2
  exit 1
fi

build_url="${base_url}/job/investment-platform-presubmit/${build_number}"
build_timeout_seconds="${JENKINS_BUILD_TIMEOUT_SECONDS:-7800}"
if [[ ! "${build_timeout_seconds}" =~ ^[0-9]+$ ]] ||
  (( build_timeout_seconds < 60 || build_timeout_seconds > 9000 )); then
  printf 'ERROR: JENKINS_BUILD_TIMEOUT_SECONDS must be between 60 and 9000.\n' >&2
  exit 2
fi
build_deadline=$((SECONDS + build_timeout_seconds))
while (( SECONDS < build_deadline )); do
  result="$(curl "${curl_args[@]}" \
    "${build_url}/api/json" |
    python3 -c 'import json,sys; print(json.load(sys.stdin).get("result") or "RUNNING")')"
  case "${result}" in
    SUCCESS)
      printf 'Jenkins PS0, PS1, and PS2 pipeline passed for %s.\n' \
        "${expected_commit}"
      exit 0
      ;;
    RUNNING)
      sleep 10
      ;;
    *)
      curl "${curl_args[@]}" \
        "${build_url}/consoleText" >&2
      printf 'ERROR: Jenkins pipeline finished with %s.\n' "${result}" >&2
      exit 1
      ;;
  esac
done

printf 'ERROR: Jenkins pipeline did not finish within %s seconds.\n' \
  "${build_timeout_seconds}" >&2
exit 1
