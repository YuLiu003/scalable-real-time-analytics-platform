#!/bin/bash

# This script switches between Python and Go implementations of the tenant management service
# Usage: ./switch-tenant-management.sh [python|go]

set -e

IMPLEMENTATION=${1:-"go"}  # Default to Go if no argument provided

if [[ "$IMPLEMENTATION" != "python" && "$IMPLEMENTATION" != "go" ]]; then
  echo "❌ Error: Implementation must be either 'python' or 'go'"
  echo "Usage: $0 [python|go]"
  exit 1
fi

# Ensure we're in the right Kubernetes context
echo "ℹ️ Using Kubernetes context: $(kubectl config current-context)"

# Namespace where services are deployed
NAMESPACE="analytics-platform"

# Check if namespace exists
if ! kubectl get namespace $NAMESPACE &>/dev/null; then
  echo "⚠️ Namespace $NAMESPACE doesn't exist. Creating it now..."
  kubectl create namespace $NAMESPACE
fi

echo "🔄 Switching tenant management service to $IMPLEMENTATION implementation..."

# Function to deploy service
deploy_service() {
  local implementation=$1
  local file=""
  local service_name=""
  
  if [[ "$implementation" == "python" ]]; then
    file="k8s/tenant-management-deployment.yaml"
    service_name="tenant-management"
  else
    file="k8s/tenant-management-go-deployment.yaml"
    service_name="tenant-management-go"
  fi
  
  # Check if deployment file exists
  if [[ ! -f "$file" ]]; then
    echo "❌ Error: Deployment file $file not found"
    exit 1
  }
  
  # Apply the deployment
  echo "📦 Deploying $implementation tenant management service..."
  kubectl apply -f "$file"
  
  # Wait for deployment to be ready
  echo "⏳ Waiting for deployment to be ready..."
  kubectl rollout status deployment/$service_name -n $NAMESPACE
}

# Delete existing deployments
echo "🗑️ Removing existing tenant management services..."
kubectl delete deployment tenant-management -n $NAMESPACE --ignore-not-found
kubectl delete deployment tenant-management-go -n $NAMESPACE --ignore-not-found
kubectl delete service tenant-management-service -n $NAMESPACE --ignore-not-found
kubectl delete service tenant-management-go-service -n $NAMESPACE --ignore-not-found

# Wait a moment for resources to be removed
sleep 2

# Deploy the selected implementation
deploy_service $IMPLEMENTATION

echo "✅ Tenant management service switched to $IMPLEMENTATION implementation."

# Show running pods
echo ""
echo "📋 Current tenant management pods:"
kubectl get pods -n $NAMESPACE -l "app in (tenant-management,tenant-management-go)" -o wide 