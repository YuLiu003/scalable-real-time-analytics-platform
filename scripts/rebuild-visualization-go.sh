#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Rebuilding visualization-go service${NC}"
echo -e "===============================\n"

# Ensure we're using minikube's Docker daemon
echo -e "${YELLOW}Connecting to minikube Docker daemon...${NC}"
eval $(minikube docker-env)

# Build the visualization-go image
echo -e "${YELLOW}Building visualization-go image...${NC}"
cd visualization-go
docker build -t visualization-go:latest .
if [ $? -ne 0 ]; then
  echo -e "${RED}Failed to build visualization-go image${NC}"
  exit 1
fi
cd ..

# Restart the deployment to use the new image
echo -e "${YELLOW}Restarting visualization-go deployment...${NC}"
kubectl rollout restart deployment/visualization-go -n analytics-platform
if [ $? -ne 0 ]; then
  echo -e "${RED}Failed to restart visualization-go deployment${NC}"
  exit 1
fi

# Wait for the deployment to be ready
echo -e "${YELLOW}Waiting for visualization-go to be ready...${NC}"
kubectl rollout status deployment/visualization-go -n analytics-platform --timeout=120s
if [ $? -ne 0 ]; then
  echo -e "${RED}Visualization-go deployment not ready in time${NC}"
  exit 1
fi

# Get service URL
echo -e "${YELLOW}Getting visualization-go service URL...${NC}"
VISUALIZATION_URL=$(minikube service -n analytics-platform visualization-go-service --url)
echo -e "${GREEN}Visualization service is accessible at: ${VISUALIZATION_URL}${NC}"

echo -e "\n${GREEN}visualization-go successfully rebuilt and redeployed!${NC}" 