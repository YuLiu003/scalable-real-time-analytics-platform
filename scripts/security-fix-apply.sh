#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Applying security fixes to deployments...${NC}"

# Apply security-fixed deployment files
echo -e "${BLUE}Applying fixed Kafka deployment...${NC}"
kubectl apply -f k8s/kafka-deployment.yaml

echo -e "${BLUE}Applying fixed Zookeeper deployment...${NC}"
kubectl apply -f k8s/zookeeper-deployment.yaml

echo -e "${BLUE}Applying fixed Grafana deployment...${NC}"
kubectl apply -f k8s/grafana-deployment.yaml

echo -e "${BLUE}Applying fixed tenant-management-secrets...${NC}"
kubectl apply -f k8s/tenant-management-secrets.yaml

# Restart deployments to apply changes
echo -e "${BLUE}Restarting deployments to apply security fixes...${NC}"
kubectl rollout restart deployment/kafka deployment/zookeeper deployment/grafana -n analytics-platform

echo -e "${GREEN}Security fixes applied!${NC}"
echo -e "${YELLOW}Note: It may take some time for all pods to restart and reflect the changes.${NC}"
echo -e "${YELLOW}Run ./manage.sh status to check the status of the deployments.${NC}"

echo -e "${BLUE}Setting executable permissions for security check script...${NC}"
chmod +x scripts/security-check.sh

echo -e "${GREEN}All set! You can now push your changes to the feature/go-api branch.${NC}"
echo -e "${YELLOW}After applying changes, run './scripts/security-check.sh' again to verify fixes in running pods.${NC}" 