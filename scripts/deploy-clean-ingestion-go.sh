#!/bin/bash
set -e

echo "🚀 Building and deploying Go-based Clean Ingestion Service..."

# Check minikube
echo "Checking minikube status..."
minikube status || minikube start

# Connect to minikube's Docker daemon for local image building
echo "Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Build the Go-based Clean Ingestion Service
echo "Building Go-based Clean Ingestion Service Docker image..."
docker build -t clean-ingestion-go:latest -f clean-ingestion-go/Dockerfile ./clean-ingestion-go

# Deploy the service
echo "Deploying Go-based Clean Ingestion Service..."
kubectl apply -f k8s/clean-ingestion-go-deployment.yaml
kubectl apply -f k8s/clean-ingestion-go-service.yaml

# Wait for deployment to be ready
echo "Waiting for deployment to be ready..."
kubectl rollout status deployment/clean-ingestion-go -n analytics-platform

echo "✅ Go-based Clean Ingestion Service deployed successfully!"
echo ""
echo "To test the service:"
echo "kubectl port-forward svc/clean-ingestion-go-service -n analytics-platform 5000:80"
echo "curl -X POST http://localhost:5000/api/data -H \"Content-Type: application/json\" -d '{\"device_id\": \"test-001\", \"temperature\": 25.5}'" 