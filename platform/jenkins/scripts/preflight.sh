#!/usr/bin/env bash
set -Eeuo pipefail

for command_name in docker kind kubectl helm make openssl; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    printf 'ERROR: required command %s was not found.\n' "${command_name}" >&2
    exit 1
  fi
done

docker info >/dev/null
