#!/usr/bin/env bash
set -Eeuo pipefail

: "${AWS_REGION:?AWS_REGION is required}"
: "${S3_BUCKET:?S3_BUCKET is required}"
: "${ECR_REGISTRY:?ECR_REGISTRY is required}"
: "${IMAGE_TAG:?IMAGE_TAG is required}"

[[ "${AWS_REGION}" =~ ^[a-z]{2}-[a-z]+-[0-9]$ ]] || {
  printf 'Invalid AWS_REGION: %s\n' "${AWS_REGION}" >&2
  exit 1
}
[[ "${S3_BUCKET}" =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || {
  printf 'Invalid S3_BUCKET\n' >&2
  exit 1
}
[[ "${ECR_REGISTRY}" =~ ^[0-9]{12}\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com$ ]] || {
  printf 'Invalid ECR_REGISTRY: %s\n' "${ECR_REGISTRY}" >&2
  exit 1
}
[[ "${IMAGE_TAG}" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || {
  printf 'Invalid IMAGE_TAG\n' >&2
  exit 1
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
rendered="$(mktemp)"
trap 'rm -f "${rendered}"' EXIT

component="${1:-all}"
case "${component}" in
  all)
    paths=(
      platform/gitops/clusters/aws-lab
      platform/gitops/platform/aws-lab/market-data-services
      platform/gitops/apps/aws-lab/market-pipeline
      platform/gitops/apps/aws-lab/portfolio-analytics
    )
    ;;
  market-pipeline)
    paths=(platform/gitops/apps/aws-lab/market-pipeline)
    ;;
  portfolio-analytics)
    paths=(platform/gitops/apps/aws-lab/portfolio-analytics)
    ;;
  *)
    printf 'Unknown component %s; use all, market-pipeline, or portfolio-analytics.\n' "${component}" >&2
    exit 1
    ;;
esac

for path in "${paths[@]}"; do
  kubectl kustomize "${repo_root}/${path}" | sed \
    -e "s|AWS_REGION_PLACEHOLDER|${AWS_REGION}|g" \
    -e "s|S3_BUCKET_PLACEHOLDER|${S3_BUCKET}|g" \
    -e "s|ECR_REGISTRY_PLACEHOLDER|${ECR_REGISTRY}|g" \
    -e "s|IMAGE_TAG_PLACEHOLDER|${IMAGE_TAG}|g" \
    >>"${rendered}"
  printf '%s\n' '---' >>"${rendered}"
done

if grep -q 'PLACEHOLDER' "${rendered}"; then
  printf 'Rendered AWS desired state still contains placeholders\n' >&2
  exit 1
fi

cat "${rendered}"
