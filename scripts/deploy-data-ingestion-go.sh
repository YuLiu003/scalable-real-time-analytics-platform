#!/bin/bash
set -e

echo "🚀 Building and deploying Go-based Data Ingestion Service..."

# Check minikube
echo "Checking minikube status..."
minikube status || minikube start

# Connect to minikube's Docker daemon for local image building
echo "Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Build the Go-based Data Ingestion Service
echo "Building Go-based Data Ingestion Service Docker image..."
docker build -t data-ingestion-go:latest -f data-ingestion-go/Dockerfile ./data-ingestion-go

# Deploy the service
echo "Deploying Go-based Data Ingestion Service..."
kubectl apply -f k8s/data-ingestion-go-deployment.yaml
kubectl apply -f k8s/data-ingestion-go-service.yaml

# Wait for deployment to be ready
echo "Waiting for deployment to be ready..."
kubectl rollout status deployment/data-ingestion-go -n analytics-platform

echo "✅ Go-based Data Ingestion Service deployed successfully!"
echo ""
echo "To test the service:"
echo "kubectl port-forward svc/data-ingestion-go-service -n analytics-platform 5000:80"
echo "curl -X POST http://localhost:5000/api/data -H \"Content-Type: application/json\" -H \"X-API-Key: YOUR_API_KEY\" -d '{\"device_id\": \"test-001\", \"temperature\": 25.5}'" 