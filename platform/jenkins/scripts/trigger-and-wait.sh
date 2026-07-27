#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
jenkins_dir="$(cd "${script_dir}/.." && pwd)"

# shellcheck disable=SC1091
source "${jenkins_dir}/versions.lock"

admin_password="$(kubectl --context "${JENKINS_CONTEXT}" --namespace "${JENKINS_NAMESPACE}" \
  get secret jenkins-admin --output=jsonpath='{.data.jenkins-admin-password}' | base64 --decode)"
port="${JENKINS_LOCAL_PORT:-18080}"
log_file="$(mktemp)"
netrc_file="$(mktemp)"
cookie_file="$(mktemp)"
chmod 0600 "${netrc_file}" "${cookie_file}"
printf 'machine 127.0.0.1 login admin password %s\n' "${admin_password}" >"${netrc_file}"
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
  rm -f "${log_file}" "${netrc_file}" "${cookie_file}"
}
trap cleanup EXIT

ready=0
for _ in {1..60}; do
  if curl "${curl_args[@]}" \
    "http://127.0.0.1:${port}/api/json" >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done
if (( ready == 0 )); then
  curl "${curl_args[@]}" "http://127.0.0.1:${port}/whoAmI/api/json" >&2 || true
  cat "${log_file}" >&2
  printf 'ERROR: Jenkins API did not accept the trusted lab administrator.\n' >&2
  exit 1
fi

crumb_json="$(curl "${curl_args[@]}" \
  "http://127.0.0.1:${port}/crumbIssuer/api/json")"
crumb_field="$(printf '%s' "${crumb_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumbRequestField"])')"
crumb_value="$(printf '%s' "${crumb_json}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["crumb"])')"

curl "${curl_args[@]}" \
  --header "${crumb_field}: ${crumb_value}" \
  --request POST \
  "http://127.0.0.1:${port}/job/investment-platform-presubmit/build"

for _ in {1..180}; do
  result="$(curl "${curl_args[@]}" \
    "http://127.0.0.1:${port}/job/investment-platform-presubmit/lastBuild/api/json" |
    python3 -c 'import json,sys; print(json.load(sys.stdin).get("result") or "RUNNING")')"
  case "${result}" in
    SUCCESS)
      printf 'Jenkins PS0, PS1, and PS2 pipeline passed.\n'
      exit 0
      ;;
    RUNNING)
      sleep 10
      ;;
    *)
      curl "${curl_args[@]}" \
        "http://127.0.0.1:${port}/job/investment-platform-presubmit/lastBuild/consoleText" >&2
      printf 'ERROR: Jenkins pipeline finished with %s.\n' "${result}" >&2
      exit 1
      ;;
  esac
done

printf 'ERROR: Jenkins pipeline did not finish within 30 minutes.\n' >&2
exit 1
