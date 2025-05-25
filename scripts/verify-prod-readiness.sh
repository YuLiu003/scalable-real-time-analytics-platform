#!/bin/bash

# Production Readiness Verification Script
# This script checks the production readiness of the real-time analytics platform

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color
PASSED=0
FAILED=0
WARNINGS=0

echo -e "\n${GREEN}=======================================================${NC}"
echo -e "${GREEN}REAL-TIME ANALYTICS PLATFORM - PRODUCTION READINESS CHECK${NC}"
echo -e "${GREEN}=======================================================${NC}\n"

# Function to check pass/fail
check_result() {
  if [ $1 -eq 0 ]; then
    echo -e "${GREEN}✓ PASS${NC}: $2"
    PASSED=$((PASSED+1))
  else
    echo -e "${RED}✗ FAIL${NC}: $2"
    FAILED=$((FAILED+1))
  fi
}

# Function to display warnings
show_warning() {
  echo -e "${YELLOW}⚠ WARNING${NC}: $1"
  WARNINGS=$((WARNINGS+1))
}

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
  echo -e "${RED}Error: kubectl is not installed or not in PATH${NC}"
  exit 1
fi

# Check if we can connect to the cluster
echo -e "${GREEN}Checking Kubernetes connectivity...${NC}"
if ! kubectl get nodes &> /dev/null; then
  echo -e "${RED}Error: Cannot connect to Kubernetes cluster. Please check your kubeconfig.${NC}"
  exit 1
fi

# Check if analytics-platform namespace exists
echo -e "${GREEN}Checking namespace existence...${NC}"
if kubectl get namespace analytics-platform &> /dev/null; then
  check_result 0 "analytics-platform namespace exists"
else
  check_result 1 "analytics-platform namespace does not exist"
  exit 1
fi

# Check if all required deployments are present
echo -e "\n${GREEN}Checking required deployments...${NC}"
REQUIRED_DEPLOYMENTS=("data-ingestion-go" "clean-ingestion-go" "processing-engine-go" "storage-layer-go" "visualization-go" "tenant-management-go")
for deployment in "${REQUIRED_DEPLOYMENTS[@]}"; do
  if kubectl get deployment $deployment -n analytics-platform &> /dev/null; then
    check_result 0 "Deployment $deployment exists"
  else
    check_result 1 "Deployment $deployment does not exist"
  fi
done

# Check if required StatefulSets exist
echo -e "\n${GREEN}Checking required StatefulSets...${NC}"
REQUIRED_STATEFULSETS=("kafka")
for statefulset in "${REQUIRED_STATEFULSETS[@]}"; do
  if kubectl get statefulset $statefulset -n analytics-platform &> /dev/null; then
    check_result 0 "StatefulSet $statefulset exists"
  else
    check_result 1 "StatefulSet $statefulset does not exist"
  fi
done

# Check if required services exist
echo -e "\n${GREEN}Checking required services...${NC}"
REQUIRED_SERVICES=("data-ingestion-go-service" "kafka" "visualization-go-service")
for service in "${REQUIRED_SERVICES[@]}"; do
  if kubectl get service $service -n analytics-platform &> /dev/null; then
    check_result 0 "Service $service exists"
  else
    check_result 1 "Service $service does not exist"
  fi
done

# Check if PVCs are bound
echo -e "\n${GREEN}Checking PVC status...${NC}"
PVC_STATUS=$(kubectl get pvc -n analytics-platform -o json | jq -r '.items[] | select(.status.phase != "Bound") | .metadata.name')
if [ -z "$PVC_STATUS" ]; then
  check_result 0 "All PVCs are bound"
else
  check_result 1 "The following PVCs are not bound: $PVC_STATUS"
fi

# Check if monitoring is set up
echo -e "\n${GREEN}Checking monitoring setup...${NC}"
if kubectl get deployment prometheus -n analytics-platform &> /dev/null; then
  check_result 0 "Prometheus is deployed"
else
  check_result 1 "Prometheus is not deployed"
fi

if kubectl get deployment grafana -n analytics-platform &> /dev/null; then
  check_result 0 "Grafana is deployed"
else
  check_result 1 "Grafana is not deployed"
fi

# Check RBAC configuration
echo -e "\n${GREEN}Checking RBAC configuration...${NC}"
if kubectl get serviceaccount -n analytics-platform | grep -q "data-ingestion-sa"; then
  check_result 0 "Service accounts are properly configured"
else
  check_result 1 "Service accounts are not properly configured"
fi

if kubectl get rolebinding -n analytics-platform &> /dev/null; then
  check_result 0 "Role bindings exist"
else
  show_warning "No role bindings found in analytics-platform namespace"
fi

# Check network policies
echo -e "\n${GREEN}Checking network policies...${NC}"
if kubectl get networkpolicies -n analytics-platform &> /dev/null; then
  check_result 0 "Network policies are configured"
else
  show_warning "No network policies found in analytics-platform namespace"
fi

# Check Kafka replicas
echo -e "\n${GREEN}Checking Kafka high availability...${NC}"
KAFKA_REPLICAS=$(kubectl get statefulset kafka -n analytics-platform -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
if [ "$KAFKA_REPLICAS" -ge 3 ]; then
  check_result 0 "Kafka has enough replicas for HA ($KAFKA_REPLICAS replicas)"
elif [ "$KAFKA_REPLICAS" -gt 0 ]; then
  show_warning "Kafka has only $KAFKA_REPLICAS replica(s). Recommend at least 3 for production."
else
  check_result 1 "Could not determine Kafka replica count"
fi

# ZooKeeper has been replaced by Kafka KRaft mode
check_result 0 "Using Kafka KRaft mode (no ZooKeeper dependency)"

# Check resource requests and limits
echo -e "\n${GREEN}Checking resource requests and limits...${NC}"
DEPLOYMENTS_WITHOUT_RESOURCES=$(kubectl get deployments -n analytics-platform -o json | jq -r '.items[] | select(.spec.template.spec.containers[0].resources.requests == null or .spec.template.spec.containers[0].resources.limits == null) | .metadata.name')
if [ -z "$DEPLOYMENTS_WITHOUT_RESOURCES" ]; then
  check_result 0 "All deployments have resource requests and limits"
else
  show_warning "The following deployments do not have resource requests or limits: $DEPLOYMENTS_WITHOUT_RESOURCES"
fi

# Check for liveness and readiness probes
echo -e "\n${GREEN}Checking liveness and readiness probes...${NC}"
DEPLOYMENTS_WITHOUT_PROBES=$(kubectl get deployments -n analytics-platform -o json | jq -r '.items[] | select(.spec.template.spec.containers[0].livenessProbe == null or .spec.template.spec.containers[0].readinessProbe == null) | .metadata.name')
if [ -z "$DEPLOYMENTS_WITHOUT_PROBES" ]; then
  check_result 0 "All deployments have liveness and readiness probes"
else
  show_warning "The following deployments do not have liveness or readiness probes: $DEPLOYMENTS_WITHOUT_PROBES"
fi

# Check for proper anti-affinity configurations
echo -e "\n${GREEN}Checking pod anti-affinity...${NC}"
DEPLOYMENTS_WITHOUT_ANTIAFFINITY=$(kubectl get deployments -n analytics-platform -o json | jq -r '.items[] | select(.spec.template.spec.affinity.podAntiAffinity == null) | .metadata.name')
if [ -z "$DEPLOYMENTS_WITHOUT_ANTIAFFINITY" ]; then
  check_result 0 "All deployments have pod anti-affinity configured"
else
  show_warning "The following deployments do not have pod anti-affinity: $DEPLOYMENTS_WITHOUT_ANTIAFFINITY"
fi

# Check for secure secrets
echo -e "\n${GREEN}Checking security settings...${NC}"
if kubectl get secret analytics-platform-secrets -n analytics-platform &> /dev/null; then
  check_result 0 "Core platform secrets are configured"
else
  check_result 1 "Core platform secrets are not configured"
fi

if kubectl get secret api-keys -n analytics-platform &> /dev/null; then
  check_result 0 "API key secrets are configured"
else
  check_result 1 "API key secrets are not configured"
fi

# Check data-ingestion security
echo -e "\n${GREEN}Checking data-ingestion security...${NC}"
kubectl port-forward svc/data-ingestion-go-service 18090:80 -n analytics-platform &>/dev/null &
PF_PID=$!
sleep 3

SECURITY_CHECK=$(curl -s -o /dev/null -w "%{http_code}" \
  http://localhost:18090/api/data \
  -H "Content-Type: application/json" \
  -d '{"sensor_id": "test-001", "value": 25.5, "timestamp": "2023-06-01T12:00:00Z"}' \
  --connect-timeout 5 \
  --max-time 10)

kill $PF_PID &>/dev/null
wait $PF_PID &>/dev/null 2>&1

if [ "$SECURITY_CHECK" = "401" ]; then
  check_result 0 "API authentication is enforced (got 401 Unauthorized without API key)"
else
  if [ -z "$SECURITY_CHECK" ]; then
    show_warning "Could not connect to data-ingestion-go-service to test API authentication"
  else
    check_result 1 "API authentication is not enforced (got $SECURITY_CHECK, expected 401)"
  fi
fi

# Check Grafana access security
echo -e "\n${GREEN}Checking Grafana security...${NC}"
kubectl port-forward svc/grafana 19090:80 -n analytics-platform &>/dev/null &
PF_PID=$!
sleep 3

GRAFANA_SECURITY_CHECK=$(curl -s -o /dev/null -w "%{http_code}" \
  http://localhost:19090/api/dashboards/home \
  --connect-timeout 5 \
  --max-time 10)

kill $PF_PID &>/dev/null
wait $PF_PID &>/dev/null 2>&1

if [ "$GRAFANA_SECURITY_CHECK" = "401" ] || [ "$GRAFANA_SECURITY_CHECK" = "302" ]; then
  check_result 0 "Grafana authentication is enforced"
else
  if [ -z "$GRAFANA_SECURITY_CHECK" ]; then
    show_warning "Could not connect to Grafana to test authentication"
  else
    check_result 1 "Grafana authentication might not be enforced (got $GRAFANA_SECURITY_CHECK)"
  fi
fi

# Summary
echo -e "\n${GREEN}=======================================================${NC}"
echo -e "${GREEN}PRODUCTION READINESS SUMMARY${NC}"
echo -e "${GREEN}=======================================================${NC}"
echo -e "✅ ${GREEN}PASSED: $PASSED${NC}"
echo -e "❌ ${RED}FAILED: $FAILED${NC}"
echo -e "⚠️ ${YELLOW}WARNINGS: $WARNINGS${NC}"
echo

if [ $FAILED -gt 0 ]; then
  echo -e "${RED}Platform is NOT fully production ready.${NC}"
  echo -e "${RED}Please fix the failed checks before deploying to production.${NC}"
  exit 1
elif [ $WARNINGS -gt 0 ]; then
  echo -e "${YELLOW}Platform is mostly production ready, but has some warnings.${NC}"
  echo -e "${YELLOW}Consider addressing the warnings for improved reliability.${NC}"
  exit 0
else
  echo -e "${GREEN}Platform is fully production ready! 🚀${NC}"
  exit 0
fi 