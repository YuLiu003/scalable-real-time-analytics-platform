#!/bin/bash
set -e

# Color variables
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}Deploying Processing Engine (Go version)...${NC}"

# Ensure we are connected to minikube's Docker daemon
eval $(minikube docker-env)

# Build the Docker image
echo -e "${YELLOW}Building Docker image...${NC}"
cd "$(dirname "$0")/../processing-engine-go"
docker build -t processing-engine-go:latest .

# Apply Kubernetes manifests
echo -e "${YELLOW}Applying Kubernetes manifests...${NC}"
kubectl apply -f ../k8s/processing-engine-go-deployment.yaml
kubectl apply -f ../k8s/processing-engine-go-service.yaml

# Wait for deployment to be ready
echo -e "${YELLOW}Waiting for deployment to be ready...${NC}"
kubectl rollout status deployment/processing-engine-go -n analytics-platform

echo -e "${GREEN}Processing Engine (Go version) deployed successfully!${NC}"
echo 
echo -e "You can check the status with:"
echo -e "  ${BLUE}kubectl get pods -n analytics-platform -l app=processing-engine-go${NC}"
echo -e "  ${BLUE}kubectl logs -n analytics-platform -l app=processing-engine-go -f${NC}"
echo
echo -e "Check metrics at:"
echo -e "  ${BLUE}kubectl port-forward -n analytics-platform svc/processing-engine-go-service 8000:8000${NC}"
echo -e "  Then visit: http://localhost:8000/metrics" 