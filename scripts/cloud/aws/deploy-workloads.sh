#!/usr/bin/env bash
set -Eeuo pipefail

: "${AWS_REGION:?AWS_REGION is required}"
: "${S3_BUCKET:?S3_BUCKET is required}"
: "${ECR_REGISTRY:?ECR_REGISTRY is required}"
: "${IMAGE_TAG:?IMAGE_TAG is required}"
: "${EKS_CLUSTER_NAME:?EKS_CLUSTER_NAME is required}"
: "${EXPECTED_AWS_ACCOUNT_ID:?EXPECTED_AWS_ACCOUNT_ID is required}"

confirmation="${CONFIRM_AWS_LAB_DEPLOY:-}"
if [[ "${confirmation}" != "${EKS_CLUSTER_NAME}" ]]; then
  printf 'Set CONFIRM_AWS_LAB_DEPLOY=%s to authorize workload changes.\n' "${EKS_CLUSTER_NAME}" >&2
  exit 1
fi

actual_account="$(aws sts get-caller-identity --query Account --output text)"
if [[ "${actual_account}" != "${EXPECTED_AWS_ACCOUNT_ID}" ]]; then
  printf 'AWS account mismatch: authenticated to %s, expected %s\n' "${actual_account}" "${EXPECTED_AWS_ACCOUNT_ID}" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
context="arn:aws:eks:${AWS_REGION}:${EXPECTED_AWS_ACCOUNT_ID}:cluster/${EKS_CLUSTER_NAME}"
rendered="$(mktemp)"
analytics_rendered="$(mktemp)"
ca_file="$(mktemp)"
trap 'rm -f "${rendered}" "${analytics_rendered}" "${ca_file}"' EXIT

aws eks update-kubeconfig --region "${AWS_REGION}" --name "${EKS_CLUSTER_NAME}"
kubectl config use-context "${context}"

kubectl apply -k "${repo_root}/platform/gitops/clusters/aws-lab"

helm repo add strimzi https://strimzi.io/charts/
helm repo update strimzi
helm upgrade --install strimzi strimzi/strimzi-kafka-operator \
  --version 1.1.0 \
  --namespace analytics-data \
  --values "${repo_root}/platform/gitops/addons/strimzi/values.yaml" \
  --wait \
  --timeout 10m

kubectl apply -k "${repo_root}/platform/gitops/platform/aws-lab/market-data-services"
kubectl wait --namespace analytics-data --for=condition=Ready kafka/market-kafka --timeout=20m

kubectl --namespace analytics-data get secret market-kafka-cluster-ca-cert \
  --output='jsonpath={.data.ca\.crt}' | base64 --decode >"${ca_file}"
kubectl --namespace analytics-apps create configmap market-kafka-cluster-ca \
  --from-file="ca.crt=${ca_file}" \
  --dry-run=client \
  --output=yaml | kubectl apply --filename=-

AWS_REGION="${AWS_REGION}" S3_BUCKET="${S3_BUCKET}" ECR_REGISTRY="${ECR_REGISTRY}" IMAGE_TAG="${IMAGE_TAG}" \
  "${repo_root}/scripts/cloud/aws/render-gitops.sh" market-pipeline >"${rendered}"

kubectl --namespace analytics-apps delete job \
  --selector='app.kubernetes.io/name=synthetic-market-producer' \
  --ignore-not-found --wait
kubectl --namespace analytics-apps delete job unauthorized-s3-probe \
  --ignore-not-found --wait
kubectl apply --filename="${rendered}"
kubectl --namespace analytics-apps rollout status deployment/raw-event-archiver --timeout=5m
kubectl --namespace analytics-apps wait --for=condition=Complete job/synthetic-market-producer-baseline --timeout=5m
kubectl --namespace analytics-apps wait --for=condition=Complete job/synthetic-market-producer-portfolio-complete --timeout=5m

deadline=$((SECONDS + 300))
until kubectl --namespace analytics-apps exec deployment/raw-event-archiver -- \
  /archive-inspector \
  --prefix 'bronze/market.price.observed/v1/date=2026-07-21/source=synthetic/instrument=DEMO-BENCH-D/synthetic:price:demo-bench-d:20260721t000130z.json' \
  --expected 1 >/dev/null 2>&1; do
  if ((SECONDS >= deadline)); then
    printf 'Timed out waiting for the baseline DEMO-BENCH-D archive effect\n' >&2
    exit 1
  fi
  sleep 5
done

AWS_REGION="${AWS_REGION}" S3_BUCKET="${S3_BUCKET}" ECR_REGISTRY="${ECR_REGISTRY}" IMAGE_TAG="${IMAGE_TAG}" \
  "${repo_root}/scripts/cloud/aws/render-gitops.sh" portfolio-analytics >"${analytics_rendered}"
kubectl --namespace analytics-apps delete job \
  --selector='app.kubernetes.io/name=portfolio-analytics' \
  --ignore-not-found --wait
kubectl apply --filename="${analytics_rendered}"
kubectl --namespace analytics-apps wait --for=condition=Complete job/portfolio-analytics-baseline --timeout=10m
kubectl --namespace analytics-apps rollout status deployment/portfolio-api --timeout=5m

kubectl --namespace analytics-apps patch job unauthorized-s3-probe \
  --type=merge --patch='{"spec":{"suspend":false}}'
kubectl --namespace analytics-apps wait --for=condition=Failed job/unauthorized-s3-probe --timeout=5m

printf 'Authorized workloads reached Kafka and S3; the unassociated S3 probe failed as required.\n'
