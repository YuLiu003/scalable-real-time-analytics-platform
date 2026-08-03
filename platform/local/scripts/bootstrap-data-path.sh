#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${LOCAL_DIR}/../.." && pwd)"

# shellcheck disable=SC1091
source "${LOCAL_DIR}/versions.lock"

"${SCRIPT_DIR}/preflight.sh"
"${SCRIPT_DIR}/verify.sh"
"${SCRIPT_DIR}/build-market-pipeline.sh"

printf 'Installing Strimzi Kafka Operator %s...\n' "${STRIMZI_OPERATOR_VERSION}"
helm repo add strimzi https://strimzi.io/charts/ --force-update
helm repo update strimzi
helm upgrade --install "${STRIMZI_RELEASE}" \
  strimzi/strimzi-kafka-operator \
  --version "${STRIMZI_OPERATOR_VERSION}" \
  --kube-context "${KUBERNETES_CONTEXT}" \
  --namespace "${DATA_NAMESPACE}" \
  --values "${REPO_ROOT}/platform/gitops/addons/strimzi/values.yaml" \
  --wait \
  --timeout 5m

kubectl --context "${KUBERNETES_CONTEXT}" wait \
  --for=condition=Established \
  customresourcedefinition/kafkas.kafka.strimzi.io \
  customresourcedefinition/kafkanodepools.kafka.strimzi.io \
  customresourcedefinition/kafkatopics.kafka.strimzi.io \
  customresourcedefinition/kafkausers.kafka.strimzi.io \
  --timeout=120s

printf 'Installing KEDA %s for Kafka-lag autoscaling...\n' "${KEDA_VERSION}"
helm repo add kedacore https://kedacore.github.io/charts --force-update
helm repo update kedacore
keda_render="$(mktemp "${TMPDIR:-/tmp}/keda-render.XXXXXX")"
helm template "${KEDA_RELEASE}" kedacore/keda \
  --version "${KEDA_VERSION}" \
  --namespace "${KEDA_NAMESPACE}" \
  --values "${REPO_ROOT}/platform/gitops/addons/keda/values.yaml" >"${keda_render}"
if ! grep -Fq -- '--enable-prometheus-metrics=true' "${keda_render}" ||
  ! grep -Fq 'kind: ServiceMonitor' "${keda_render}"; then
  rm -f "${keda_render}"
  printf 'ERROR: pinned KEDA values did not render operator metrics and a ServiceMonitor.\n' >&2
  exit 1
fi
rm -f "${keda_render}"
helm upgrade --install "${KEDA_RELEASE}" \
  kedacore/keda \
  --version "${KEDA_VERSION}" \
  --kube-context "${KUBERNETES_CONTEXT}" \
  --namespace "${KEDA_NAMESPACE}" \
  --values "${REPO_ROOT}/platform/gitops/addons/keda/values.yaml" \
  --wait \
  --timeout 5m
kubectl --context "${KUBERNETES_CONTEXT}" wait \
  --for=condition=Established \
  customresourcedefinition/scaledobjects.keda.sh \
  customresourcedefinition/triggerauthentications.keda.sh \
  --timeout=120s

secret_count=0
for secret_ref in \
  "${DATA_NAMESPACE}/garage-server-config" \
  "${DATA_NAMESPACE}/market-archive-credentials" \
  "${APPLICATION_NAMESPACE}/market-archive-credentials"; do
  namespace="${secret_ref%%/*}"
  secret_name="${secret_ref##*/}"
  if kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${namespace}" \
    get secret "${secret_name}" >/dev/null 2>&1; then
    secret_count=$((secret_count + 1))
  fi
done

if (( secret_count == 0 )); then
  printf 'Generating local-only Garage configuration and archive credentials...\n'
  rpc_secret="$(openssl rand -hex 32)"
  admin_token="$(openssl rand -hex 32)"
  metrics_token="$(openssl rand -hex 32)"
  access_key="GK$(openssl rand -hex 16)"
  secret_key="$(openssl rand -hex 32)"
  garage_config="$(printf '%s\n' \
    'metadata_dir = "/var/lib/garage/meta"' \
    'data_dir = "/var/lib/garage/data"' \
    'db_engine = "sqlite"' \
    'replication_factor = 1' \
    'compression_level = 1' \
    'rpc_bind_addr = "[::]:3901"' \
    'rpc_public_addr = "garage-0.garage-internal.analytics-data.svc.cluster.local:3901"' \
    "rpc_secret = \"${rpc_secret}\"" \
    '[s3_api]' \
    's3_region = "garage"' \
    'api_bind_addr = "[::]:3900"' \
    'root_domain = ".s3.garage.local"' \
    '[admin]' \
    'api_bind_addr = "[::]:3903"' \
    "admin_token = \"${admin_token}\"" \
    "metrics_token = \"${metrics_token}\"")"

  kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
    create secret generic garage-server-config \
    --from-literal="garage.toml=${garage_config}" \
    --dry-run=client --output=yaml \
    | kubectl --context "${KUBERNETES_CONTEXT}" apply -f -

  for namespace in "${DATA_NAMESPACE}" "${APPLICATION_NAMESPACE}"; do
    kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${namespace}" \
      create secret generic market-archive-credentials \
      --from-literal="AWS_ACCESS_KEY_ID=${access_key}" \
      --from-literal="AWS_SECRET_ACCESS_KEY=${secret_key}" \
      --from-literal='AWS_REGION=garage' \
      --from-literal='S3_BUCKET=market-raw' \
      --from-literal='S3_ENDPOINT=http://garage.analytics-data.svc.cluster.local:3900' \
      --dry-run=client --output=yaml \
      | kubectl --context "${KUBERNETES_CONTEXT}" apply -f -
  done
elif (( secret_count == 3 )); then
  printf 'Reusing existing Garage configuration and archive credential Secrets.\n'
else
  printf 'ERROR: Garage Secret set is incomplete (%d of 3 present). Repair it or run the confirmed data-path destroy before retrying.\n' "${secret_count}" >&2
  exit 1
fi

printf 'Applying Kafka, topic, and Garage desired state...\n'
kubectl --context "${KUBERNETES_CONTEXT}" apply \
  -k "${REPO_ROOT}/platform/gitops/platform/local/market-data-services"

kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
  rollout status statefulset/garage --timeout=5m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
  wait kafka/"${KAFKA_CLUSTER_NAME}" --for=condition=Ready --timeout=15m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
  wait kafkatopic/market-prices kafkatopic/ingestion-quarantine kafkatopic/market-prices-scale \
  --for=condition=Ready --timeout=5m

printf 'Publishing the Kafka broker CA to the application namespace...\n'
cluster_ca_file="$(mktemp "${TMPDIR:-/tmp}/market-kafka-cluster-ca.XXXXXX")"
cleanup_cluster_ca() {
  rm -f "${cluster_ca_file}"
}
trap cleanup_cluster_ca EXIT
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${DATA_NAMESPACE}" \
  get secret "${KAFKA_CLUSTER_NAME}-cluster-ca-cert" \
  --output=go-template='{{ index .data "ca.crt" | base64decode }}' >"${cluster_ca_file}"
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  create configmap "${KAFKA_CLUSTER_NAME}-cluster-ca" \
  --from-file="ca.crt=${cluster_ca_file}" \
  --dry-run=client --output=yaml \
  | kubectl --context "${KUBERNETES_CONTEXT}" apply -f -
cleanup_cluster_ca
trap - EXIT

printf 'Applying producer, archiver, identities, and failure fixtures...\n'
kubectl --context "${KUBERNETES_CONTEXT}" apply \
  -k "${REPO_ROOT}/platform/gitops/apps/local/market-pipeline"
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  wait kafkauser/synthetic-market-producer kafkauser/raw-event-archiver \
  kafkauser/scale-load-producer kafkauser/scale-event-archiver \
  --for=condition=Ready --timeout=5m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  rollout status deployment/raw-event-archiver --timeout=5m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  rollout status deployment/scale-event-archiver --timeout=5m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  wait scaledobject/scale-event-archiver --for=condition=Ready --timeout=5m
kubectl --context "${KUBERNETES_CONTEXT}" --namespace "${APPLICATION_NAMESPACE}" \
  wait job/synthetic-market-producer-baseline --for=condition=Complete --timeout=5m

"${SCRIPT_DIR}/verify-data-path.sh"
