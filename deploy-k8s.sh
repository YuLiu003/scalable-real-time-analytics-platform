#!/bin/bash
set -e

# Set your registry here if using a private registry
REGISTRY=""

# Check if all required YAML files exist
for required_file in k8s/namespace.yaml k8s/configmap.yaml k8s/kafka.yaml k8s/zookeeper.yaml k8s/data-ingestion.yaml k8s/processing-engine.yaml k8s/storage-layer.yaml k8s/visualization.yaml k8s/ingress.yaml k8s/hpa.yaml
do
  if [ ! -f "$required_file" ]; then
    echo "Error: Required file $required_file not found"
    exit 1
  fi
done

# Replace placeholders in YAML files if registry is set
if [ -n "$REGISTRY" ]; then
  find k8s -name "*.yaml" -type f -exec sed -i '' "s|\${YOUR_REGISTRY}|$REGISTRY|g" {} \;
fi

echo "Creating namespace..."
kubectl apply -f k8s/namespace.yaml

echo "Applying ConfigMap..."
kubectl apply -f k8s/configmap.yaml

echo "Deploying ZooKeeper and Kafka..."
kubectl apply -f k8s/zookeeper.yaml
kubectl apply -f k8s/kafka.yaml

echo "Waiting for ZooKeeper and Kafka to be ready..."
kubectl wait --namespace=analytics-platform --for=condition=ready pod --selector=app=zookeeper --timeout=180s || echo "ZooKeeper not ready within timeout, continuing anyway"
kubectl wait --namespace=analytics-platform --for=condition=ready pod --selector=app=kafka --timeout=180s || echo "Kafka not ready within timeout, continuing anyway"

echo "Deploying microservices..."
kubectl apply -f k8s/data-ingestion.yaml
kubectl apply -f k8s/processing-engine.yaml
kubectl apply -f k8s/storage-layer.yaml
kubectl apply -f k8s/visualization.yaml

echo "Applying HPAs..."
kubectl apply -f k8s/hpa.yaml

echo "Applying Ingress..."
kubectl apply -f k8s/ingress.yaml

echo "Deployment completed!"
echo "Check status with: kubectl get pods -n analytics-platform"
echo "Access the visualization dashboard at: http://$(minikube ip)/ (may require 'minikube tunnel' in a separate terminal)"
