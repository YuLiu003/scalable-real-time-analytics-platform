#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tofu_bin="${TOFU_BIN:-tofu}"
rendered="$(mktemp)"
trap 'rm -f "${rendered}"' EXIT
export TF_PLUGIN_CACHE_DIR="${TF_PLUGIN_CACHE_DIR:-${TMPDIR:-/tmp}/portfolio-opentofu-plugin-cache}"
mkdir -p "${TF_PLUGIN_CACHE_DIR}"

"${tofu_bin}" fmt -check -recursive "${repo_root}/infra/opentofu/aws"

for stack in bootstrap lab; do
  stack_dir="${repo_root}/infra/opentofu/aws/${stack}"
  "${tofu_bin}" -chdir="${stack_dir}" init -backend=false -input=false -lockfile=readonly
  "${tofu_bin}" -chdir="${stack_dir}" validate
  "${tofu_bin}" -chdir="${stack_dir}" test
done

grep -q 'dynamic "pod_identity_association"' "${repo_root}/infra/opentofu/aws/lab/eks.tf"
grep -q 'service_account = pod_identity_association.value.service_account' "${repo_root}/infra/opentofu/aws/lab/eks.tf"

AWS_REGION=us-west-2 \
S3_BUCKET=iap-aws-lab-123456789012-example \
ECR_REGISTRY=123456789012.dkr.ecr.us-west-2.amazonaws.com \
IMAGE_TAG=sha-0123456789abcdef \
  "${repo_root}/scripts/cloud/aws/render-gitops.sh" >"${rendered}"

if grep -Eq 'AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY|secretKeyRef' "${rendered}"; then
  printf 'AWS desired state contains a static AWS credential reference\n' >&2
  exit 1
fi

grep -q 'name: market-kafka' "${rendered}"
grep -q 'replicas: 3' "${rendered}"
grep -q 'min.insync.replicas: 2' "${rendered}"
grep -q 'topologyKey: topology.kubernetes.io/zone' "${rendered}"
grep -q 'topologyKey: kubernetes.io/hostname' "${rendered}"
grep -q 'CostScope=investment-analytics-aws-lab' "${rendered}"
grep -q 'serviceAccountName: raw-event-archiver' "${rendered}"
grep -q 'serviceAccountName: portfolio-analytics' "${rendered}"
grep -q 'serviceAccountName: portfolio-api' "${rendered}"
grep -q 'name: unauthorized-s3-probe' "${rendered}"

printf 'AWS OpenTofu and Kubernetes desired-state validation passed.\n'
