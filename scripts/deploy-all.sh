#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check pod status
check_pod_status() {
  local namespace=$1
  local label=$2
  local expected_count=$3
  local timeout=$4
  local retry_interval=5
  local elapsed=0
  
  echo -e "${YELLOW}Waiting for $label pods to be ready in namespace $namespace...${NC}"
  
  while [ $elapsed -lt $timeout ]; do
    local ready_count=$(kubectl get pods -n $namespace -l app=$label -o jsonpath='{.items[?(@.status.phase=="Running")].status.containerStatuses[0].ready}' | grep -o "true" | wc -l)
    
    if [ "$ready_count" -eq "$expected_count" ]; then
      echo -e "${GREEN}All $label pods are ready!${NC}"
      return 0
    fi
    
    echo -e "${YELLOW}Waiting for $label pods to be ready ($ready_count/$expected_count ready). Retrying in $retry_interval seconds...${NC}"
    sleep $retry_interval
    elapsed=$((elapsed + retry_interval))
  done
  
  echo -e "${RED}Timed out waiting for $label pods to be ready.${NC}"
  return 1
}

echo -e "${BLUE}Deploying Real-Time Analytics Platform with security configurations...${NC}"

# Create namespace if it doesn't exist
kubectl apply -f k8s/namespace.yaml

echo -e "${BLUE}Applying RBAC configurations...${NC}"
kubectl apply -f k8s/rbac.yaml

echo -e "${BLUE}Applying secrets and configmaps...${NC}"
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/api-keys-secret.yaml
kubectl apply -f k8s/kafka-secrets.yaml
kubectl apply -f k8s/configmap.yaml

echo -e "${BLUE}Applying network policies...${NC}"
kubectl apply -f k8s/network-policy.yaml

# Clean up old deployments if they exist
echo -e "${BLUE}Cleaning up old statefulsets if they exist...${NC}"
kubectl delete statefulset -n analytics-platform zookeeper --ignore-not-found
kubectl delete statefulset -n analytics-platform kafka --ignore-not-found
kubectl delete pvc -n analytics-platform data-zookeeper-0 --ignore-not-found
kubectl delete pvc -n analytics-platform data-kafka-0 --ignore-not-found

echo -e "${BLUE}Deploying Zookeeper StatefulSet...${NC}"
kubectl apply -f k8s/zookeeper-statefulset.yaml
if ! check_pod_status "analytics-platform" "zookeeper" 1 300; then
  echo -e "${RED}Failed to start Zookeeper. Check logs with: kubectl logs -n analytics-platform -l app=zookeeper${NC}"
  echo -e "${YELLOW}Continuing with deployment, but services may not function correctly.${NC}"
else
  echo -e "${GREEN}Zookeeper started successfully.${NC}"
fi

echo -e "${BLUE}Deploying Kafka StatefulSet...${NC}"
kubectl apply -f k8s/kafka-statefulset.yaml
if ! check_pod_status "analytics-platform" "kafka" 1 300; then
  echo -e "${RED}Failed to start Kafka. Check logs with: kubectl logs -n analytics-platform -l app=kafka${NC}"
  echo -e "${YELLOW}Continuing with deployment, but services may not function correctly.${NC}"
else
  echo -e "${GREEN}Kafka started successfully.${NC}"
  
  echo -e "${BLUE}Creating required Kafka topics...${NC}"
  kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --create --bootstrap-server kafka:9092 --replication-factor 1 --partitions 3 --topic sensor-data || true
  kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --create --bootstrap-server kafka:9092 --replication-factor 1 --partitions 3 --topic processed-data || true
  kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --create --bootstrap-server kafka:9092 --replication-factor 1 --partitions 3 --topic sensor-data-clean || true
fi

echo -e "${BLUE}Deploying storage layer...${NC}"
kubectl apply -f k8s/storage-layer-pvc.yaml
kubectl apply -f k8s/storage-layer-go-deployment.yaml
kubectl apply -f k8s/storage-layer-go-service.yaml
sleep 10

echo -e "${BLUE}Deploying Go microservices...${NC}"
kubectl apply -f k8s/data-ingestion-go-deployment.yaml
kubectl apply -f k8s/data-ingestion-go-service.yaml
kubectl apply -f k8s/clean-ingestion-go-deployment.yaml
kubectl apply -f k8s/clean-ingestion-go-service.yaml
kubectl apply -f k8s/processing-engine-go-deployment.yaml
kubectl apply -f k8s/processing-engine-go-service.yaml
kubectl apply -f k8s/visualization-go-deployment.yaml
kubectl apply -f k8s/visualization-go-service.yaml
kubectl apply -f k8s/tenant-management-go-deployment.yaml
kubectl apply -f k8s/tenant-management-go-service.yaml

echo -e "${BLUE}Deploying monitoring (Prometheus)...${NC}"
kubectl apply -f k8s/prometheus-config.yaml
kubectl apply -f k8s/prometheus-deployment.yaml
kubectl apply -f k8s/prometheus-rbac.yaml

echo -e "${GREEN}Deployment complete!${NC}"

echo -e "${BLUE}Running security check...${NC}"
./scripts/security-check.sh

echo -e "${BLUE}Platform Status${NC}"
echo -e "================="
echo
echo -e "${BLUE}Minikube Status:${NC}"
minikube status
echo
echo -e "${BLUE}Namespace:${NC}"
kubectl get namespace analytics-platform > /dev/null 2>&1 && echo "analytics-platform namespace exists" || echo "analytics-platform namespace does not exist"
echo
echo -e "${BLUE}Pods:${NC}"
kubectl get pods -n analytics-platform
echo
echo -e "${BLUE}Services:${NC}"
kubectl get services -n analytics-platform
echo
echo -e "${BLUE}Deployments:${NC}"
kubectl get deployments -n analytics-platform
echo
echo -e "${BLUE}StatefulSets:${NC}"
kubectl get statefulsets -n analytics-platform
echo
echo -e "${BLUE}Persistent Volumes:${NC}"
kubectl get pv
echo
echo -e "${BLUE}Resource Usage:${NC}"
kubectl top pods -n analytics-platform 2>/dev/null || echo "Metrics server not available" 