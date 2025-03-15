#!/bin/bash
set -e

echo "🚀 Deploying Real-Time Analytics Platform..."

# Make sure minikube is running
echo "Checking minikube status..."
minikube status || minikube start

# Connect to minikube's Docker daemon
echo "Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Update dependencies and build images
echo "Building container images..."
./fix-dependencies.sh

# Apply Kubernetes configurations
echo "Applying Kubernetes configurations..."

# Create namespace if it doesn't exist
kubectl apply -f k8s/namespace.yaml

# Deploy Kafka first
echo "Deploying Kafka..."
kubectl apply -f k8s/kafka-deployment.yaml
kubectl apply -f k8s/kafka-service.yaml

# Wait for Kafka to start
echo "Waiting for Kafka to start..."
kubectl rollout status deployment/kafka -n analytics-platform --timeout=120s || true

# Deploy other components
echo "Deploying microservices..."
kubectl apply -f k8s/data-ingestion-deployment.yaml
kubectl apply -f k8s/data-ingestion-service.yaml
kubectl apply -f k8s/processing-engine-deployment.yaml
kubectl apply -f k8s/processing-engine-service.yaml
kubectl apply -f k8s/storage-layer-deployment.yaml
kubectl apply -f k8s/storage-layer-service.yaml
kubectl apply -f k8s/visualization-deployment.yaml
kubectl apply -f k8s/visualization-service.yaml
kubectl apply -f k8s/ingress.yaml || echo "Ingress not applied - make sure ingress controller is enabled"

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
echo "- To open the dashboard automatically, run: ./open-dashboard.sh"
echo ""
echo "💡 For better access, enable the ingress controller:"
echo "   minikube addons enable ingress"
echo "   Then access via: http://$NODE_IP/"