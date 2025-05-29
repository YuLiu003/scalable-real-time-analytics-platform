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

# Get the absolute path to the project root
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "🚀 Deploying Visualization Service (Go)..."

# Make sure minikube is running
echo "Checking minikube status..."
minikube status || minikube start

# Connect to minikube's Docker daemon for local image building
echo "Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Build the Docker image
echo "Building Docker image for visualization-go..."
cd "${PROJECT_ROOT}/visualization-go"
docker build -t visualization-go:latest .

# Apply Kubernetes configuration
echo "Applying Kubernetes manifests..."
kubectl apply -f ${PROJECT_ROOT}/k8s/visualization-go-deployment.yaml
kubectl apply -f ${PROJECT_ROOT}/k8s/visualization-go-service.yaml

# Wait for the deployment to be available
echo "Waiting for deployment to be ready..."
kubectl rollout status deployment/visualization-go -n analytics-platform

echo "✅ Visualization Service (Go) deployed successfully!"
echo "🔍 Access the service at http://$(minikube ip):30083" 