#!/bin/bash
set -e

echo "🚀 Deploying Real-Time Analytics Platform with Enhanced Security..."

# Make sure minikube is running
echo "Checking minikube status..."
minikube status || minikube start

# Connect to minikube's Docker daemon
echo "Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Build container images with security best practices
echo "Building secure container images..."
docker-compose build

echo "Applying Kubernetes configurations..."

# Create namespace and config resources
echo "Setting up namespace and configurations..."
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml

# Deploy ZooKeeper first
echo "Deploying ZooKeeper..."
kubectl apply -f k8s/zookeeper-service.yaml

# Deploy Kafka with proper configuration
echo "Deploying Kafka messaging system..."
kubectl apply -f k8s/kafka-deployment.yaml
kubectl apply -f k8s/bridge-service.yaml

# Wait for Kafka to start
echo "Waiting for Kafka to start..."
sleep 10
kubectl wait --for=condition=available deployment/kafka -n analytics-platform --timeout=120s || true

# Apply resource limits and security context for microservices
echo "Deploying microservices with security configurations..."

# Deploy data ingestion service
kubectl apply -f k8s/data-ingestion-deployment.yaml
kubectl apply -f k8s/data-ingestion-service.yaml

# Deploy processing engine
kubectl apply -f k8s/processing-engine-deployment.yaml
kubectl apply -f k8s/processing-engine-service.yaml

# Deploy storage layer
kubectl apply -f k8s/storage-layer-deployment.yaml
kubectl apply -f k8s/storage-layer-service.yaml

# Deploy visualization service
kubectl apply -f k8s/visualization-deployment.yaml
kubectl apply -f k8s/visualization-service.yaml

# Create default network policy (optional - uncomment if needed)
# kubectl apply -f k8s/default-network-policy.yaml

# Wait for deployments to be ready
echo "Waiting for deployments to be ready..."
kubectl rollout status deployment/data-ingestion -n analytics-platform --timeout=60s || true
kubectl rollout status deployment/processing-engine -n analytics-platform --timeout=60s || true
kubectl rollout status deployment/storage-layer -n analytics-platform --timeout=60s || true
kubectl rollout status deployment/visualization -n analytics-platform --timeout=60s || true

echo "✅ Deployment completed!"

# Check pod status
echo "Checking pod status:"
kubectl get pods -n analytics-platform

# Print access information
NODE_IP=$(minikube ip)
echo ""
echo "🌐 Access your application:"
echo "- Visualization Dashboard: http://$NODE_IP:30081"
echo "- Data Ingestion API: http://$NODE_IP:30080/api/data"
echo ""
echo "🔐 Security notes:"
echo "- API endpoints are not authenticated - consider adding API keys"
echo "- Kafka connections are not encrypted - consider enabling TLS"
echo "- For production, implement NetworkPolicies and RBAC"
echo ""
echo "To test data flow:"
echo "kubectl port-forward -n analytics-platform svc/data-ingestion-service 8080:80"
echo "curl -X POST http://localhost:8080/api/data -H \"Content-Type: application/json\" -d '{\"device_id\": \"test-001\", \"temperature\": 25.5, \"humidity\": 60}'"