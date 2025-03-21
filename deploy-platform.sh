#!/bin/bash
set -e

# Improved error handling
handle_error() {
  echo "❌ Error occurred at line $1"
  echo "Command: '${BASH_COMMAND}'"
  echo "Deployment failed. Check logs above for details."
  exit 1
}

trap 'handle_error $LINENO' ERR

echo "🚀 Deploying Real-Time Analytics Platform..."

# Make sure minikube is running
echo "Checking minikube status..."
minikube status || minikube start

# Connect to minikube's Docker daemon for local image building
echo "Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Clean up any existing deployments
echo "Cleaning up existing resources..."
kubectl delete namespace analytics-platform --ignore-not-found

# Wait a moment to ensure resources are deleted
sleep 5

# Build container images with correct names using build-images.sh
echo "Building container images..."
chmod +x build-images.sh
./build-images.sh

echo "Applying Kubernetes configurations..."

# Create namespace
echo "Creating namespace..."
kubectl create namespace analytics-platform

# Apply core infrastructure 
echo "Deploying core infrastructure..."

# Create necessary secrets and configmaps
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/grafana-secret.yaml

echo "Deploying monitoring stack..."
kubectl apply -f k8s/prometheus-config.yaml
kubectl apply -f k8s/prometheus-deployment.yaml

# Deploy KRaft-based Kafka (No zookeeper needed)
echo "Deploying Kafka (KRaft mode)..."

# First apply the secret (should be excluded from version control)
if [ -f k8s/kafka-secrets.yaml ]; then
  kubectl apply -f k8s/kafka-secrets.yaml
else
  # Generate new Kafka KRaft cluster ID
  ENCODED_ID=$(echo -n "$(openssl rand -hex 11)" | base64)
  cat > k8s/kafka-secrets.yaml << EOF
apiVersion: v1
kind: Secret
metadata:
  name: kafka-secrets
  namespace: analytics-platform
type: Opaque
data:
  KAFKA_KRAFT_CLUSTER_ID: "$ENCODED_ID"
EOF
  kubectl apply -f k8s/kafka-secrets.yaml
fi

# Then apply the Kafka deployment and service
kubectl apply -f k8s/kafka-kraft-deployment.yaml
kubectl apply -f k8s/kafka-service.yaml

echo "Waiting for Kafka to start..."
kubectl rollout status deployment/kafka -n analytics-platform --timeout=180s || echo "Still waiting for Kafka..."

# Deploy core services
echo "Deploying core platform services..."
kubectl apply -f k8s/flask-api-deployment.yaml
kubectl apply -f k8s/flask-api-service.yaml
kubectl apply -f k8s/data-ingestion-deployment.yaml
kubectl apply -f k8s/data-ingestion-service.yaml
kubectl apply -f k8s/processing-engine-deployment.yaml
kubectl apply -f k8s/processing-engine-service.yaml
kubectl apply -f k8s/storage-layer-deployment.yaml
kubectl apply -f k8s/storage-layer-service.yaml
kubectl apply -f k8s/visualization-deployment.yaml
kubectl apply -f k8s/visualization-service.yaml

echo "  ./manage.sh status"