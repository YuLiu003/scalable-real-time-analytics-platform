#!/usr/bin/env bash

capacity_prometheus_vector_valid() {
  local input_file="$1"
  local required_label="$2"
  python3 - "${input_file}" "${required_label}" <<'PY'
import json
import math
import pathlib
import sys

try:
    response = json.loads(pathlib.Path(sys.argv[1]).read_text())
    result = response["data"]["result"]
    valid = response.get("status") == "success" and isinstance(result, list) and result
    for item in result:
        metric = item["metric"]
        value = item["value"]
        valid = valid and sys.argv[2] in metric and len(value) == 2 and math.isfinite(float(value[1]))
except (KeyError, OSError, TypeError, ValueError, json.JSONDecodeError):
    valid = False
raise SystemExit(0 if valid else 1)
PY
}

capacity_prometheus_query_to_file() {
  local context="$1"
  local observability_namespace="$2"
  local query="$3"
  local required_label="$4"
  local output_file="$5"
  local timeout_seconds="$6"
  local sample_interval_seconds="$7"
  local request_timeout_cap_seconds="$8"
  local encoded deadline remaining request_timeout sleep_seconds staged

  encoded="$(python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "${query}")"
  deadline=$((SECONDS + timeout_seconds))
  staged="$(mktemp "${output_file}.tmp.XXXXXX")"
  while true; do
    remaining=$((deadline - SECONDS))
    if (( remaining <= 0 )); then
      break
    fi
    request_timeout="${request_timeout_cap_seconds}"
    if (( request_timeout > remaining )); then
      request_timeout="${remaining}"
    fi
    if kubectl --context "${context}" --request-timeout="${request_timeout}s" get --raw \
      "/api/v1/namespaces/${observability_namespace}/services/http:monitoring-kube-prometheus-prometheus:9090/proxy/api/v1/query?query=${encoded}" \
      >"${staged}" 2>/dev/null && capacity_prometheus_vector_valid "${staged}" "${required_label}"; then
      mv "${staged}" "${output_file}"
      return 0
    fi
    remaining=$((deadline - SECONDS))
    (( remaining > 0 )) || break
    sleep_seconds="${sample_interval_seconds}"
    if (( sleep_seconds > remaining )); then
      sleep_seconds="${remaining}"
    fi
    sleep "${sleep_seconds}"
  done
  rm -f "${staged}"
  printf 'ERROR: Prometheus did not return a valid %s vector within %d seconds.\n' \
    "${required_label}" "${timeout_seconds}" >&2
  return 1
}

capacity_capture_pod_stability() {
  local context="$1"
  local output_file="$2"
  local staged
  staged="$(mktemp "${output_file}.tmp.XXXXXX")"
  if kubectl --context "${context}" --request-timeout=10s get pods --all-namespaces \
    --field-selector=status.phase!=Succeeded,status.phase!=Failed \
    --output=jsonpath='{range .items[*]}{.metadata.namespace}{"/"}{.metadata.name}{"|"}{.metadata.uid}{"|"}{range .status.initContainerStatuses[*]}{"init:"}{.name}{":"}{.restartCount}{","}{end}{range .status.containerStatuses[*]}{.name}{":"}{.restartCount}{","}{end}{"\n"}{end}' \
    | LC_ALL=C sort >"${staged}" && grep -q '[^[:space:]]' "${staged}"; then
    mv "${staged}" "${output_file}"
    return 0
  fi
  rm -f "${staged}"
  printf 'ERROR: unable to capture the benchmark pod-stability snapshot.\n' >&2
  return 1
}

capacity_assert_pod_stability() {
  local baseline_file="$1"
  local observed_file="$2"
  if cmp -s "${baseline_file}" "${observed_file}"; then
    return 0
  fi
  printf 'ERROR: a baseline pod was replaced or restarted during the capacity suite.\n' >&2
  diff -u "${baseline_file}" "${observed_file}" >&2 || true
  return 1
}
