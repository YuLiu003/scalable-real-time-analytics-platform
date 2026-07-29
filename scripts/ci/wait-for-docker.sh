#!/usr/bin/env bash
set -Eeuo pipefail

for _ in {1..60}; do
  if docker info >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

printf 'ERROR: Docker daemon was not ready after 60 seconds.\n' >&2
exit 1
