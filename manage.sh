#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Display help information
show_help() {
    echo -e "${BLUE}Real-Time Analytics Platform Management Script${NC}"
    echo "Usage: ./manage.sh [command]"
    echo ""
    echo "Commands:"
    echo "  status      - Show the status of all platform components"
    echo "  start       - Start minikube if not running"
    echo "  stop        - Stop minikube cluster"
    echo "  deploy      - Deploy the full platform"
    echo "  monitoring  - Deploy monitoring stack (Prometheus/Grafana)"
    echo "  dashboard   - Open Kubernetes dashboard"
    echo "  prometheus  - Access Prometheus UI (port-forward)"
    echo "  grafana     - Access Grafana UI (port-forward)"
    echo "  test        - Run platform tests"
    echo "  pods        - List all pods"
    echo "  logs <pod>  - View logs for a specific pod"
    echo "  describe <pod> - Get detailed information about a pod"
    echo "  restart <deployment> - Restart a specific deployment"
    echo "  restart-all - Restart all deployments"
    echo "  events      - Show recent events in the cluster"
    echo "  fix         - Attempt to fix common issues"
    echo "  reset       - Delete all resources and clean up the project"
    echo "  cleanup     - Remove all platform resources"
    echo "  help        - Show this help information"
}

# Check minikube status and start if needed
ensure_minikube_running() {
    echo -e "${BLUE}Checking minikube status...${NC}"
    if ! minikube status &> /dev/null; then
        echo -e "${YELLOW}Minikube is not running. Starting minikube...${NC}"
        minikube start
        if [ $? -ne 0 ]; then
            echo -e "${RED}Failed to start minikube. Please check your installation.${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}Minikube is already running.${NC}"
    fi
}

# Check if namespace exists
ensure_namespace_exists() {
    if ! kubectl get namespace analytics-platform &> /dev/null; then
        echo -e "${YELLOW}Creating analytics-platform namespace...${NC}"
        kubectl create namespace analytics-platform
    fi
}

# Show status of all platform components
show_status() {
    echo -e "${BLUE}Platform Status${NC}"
    echo "================="
    
    # Check minikube
    echo -e "\n${BLUE}Minikube Status:${NC}"
    minikube status
    
    # Check namespace
    echo -e "\n${BLUE}Namespace:${NC}"
    kubectl get namespace analytics-platform &> /dev/null
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}analytics-platform namespace exists${NC}"
    else
        echo -e "${RED}analytics-platform namespace does not exist${NC}"
        return
    fi
    
    # Check pods
    echo -e "\n${BLUE}Pods:${NC}"
    kubectl get pods -n analytics-platform
    
    # Check services
    echo -e "\n${BLUE}Services:${NC}"
    kubectl get services -n analytics-platform
    
    # Check deployments
    echo -e "\n${BLUE}Deployments:${NC}"
    kubectl get deployments -n analytics-platform
    
    # Check persistent volumes
    echo -e "\n${BLUE}Persistent Volumes:${NC}"
    kubectl get pv 2>/dev/null || echo "No persistent volumes found"
    
    # Check resource usage
    echo -e "\n${BLUE}Resource Usage:${NC}"
    kubectl top pods -n analytics-platform 2>/dev/null || echo "Metrics server not available"
}

# Deploy the platform
deploy_platform() {
    ensure_minikube_running
    echo -e "${BLUE}Deploying Real-Time Analytics Platform...${NC}"
    ./deploy-platform.sh
    echo -e "${GREEN}Deployment complete!${NC}"
}

# Deploy monitoring stack
deploy_monitoring() {
    ensure_minikube_running
    ensure_namespace_exists
    echo -e "${BLUE}Deploying monitoring stack...${NC}"
    kubectl apply -f k8s/prometheus-config.yaml
    kubectl apply -f k8s/prometheus-deployment.yaml
    kubectl apply -f k8s/grafana-secret.yaml
    kubectl apply -f k8s/grafana-deployment.yaml
    
    echo "🔍 Checking deployment status..."
    kubectl rollout status deployment/prometheus -n analytics-platform
    kubectl rollout status deployment/grafana -n analytics-platform
    
    echo -e "${GREEN}Monitoring deployment complete!${NC}"
    echo -e "${YELLOW}To access Prometheus: kubectl port-forward service/prometheus-service 9090:9090 -n analytics-platform${NC}"
    echo -e "${YELLOW}To access Grafana: kubectl port-forward service/grafana-service 3000:3000 -n analytics-platform${NC}"
}

# Open Kubernetes dashboard
open_dashboard() {
    ensure_minikube_running
    echo -e "${BLUE}Opening Kubernetes dashboard...${NC}"
    minikube dashboard
}

# Port forward to Prometheus
access_prometheus() {
    ensure_minikube_running
    
    # Check if the service exists
    if kubectl get svc prometheus -n analytics-platform &>/dev/null; then
        SERVICE_NAME="prometheus"
    elif kubectl get svc prometheus-service -n analytics-platform &>/dev/null; then
        SERVICE_NAME="prometheus-service"
    else
        echo -e "${RED}Error: Prometheus service not found${NC}"
        return 1
    fi
    
    echo -e "${BLUE}Setting up port forwarding to Prometheus...${NC}"
    echo -e "${YELLOW}Access Prometheus at: http://localhost:9090${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl port-forward svc/$SERVICE_NAME -n analytics-platform 9090:9090
}

# Port forward to Grafana
access_grafana() {
    ensure_minikube_running
    
    # Check if the service exists
    if kubectl get svc grafana -n analytics-platform &>/dev/null; then
        SERVICE_NAME="grafana"
    elif kubectl get svc grafana-service -n analytics-platform &>/dev/null; then
        SERVICE_NAME="grafana-service"
    else
        echo -e "${RED}Error: Grafana service not found${NC}"
        return 1
    fi
    
    echo -e "${BLUE}Setting up port forwarding to Grafana...${NC}"
    echo -e "${YELLOW}Access Grafana at: http://localhost:3000${NC}"
    echo -e "${YELLOW}Default credentials: admin / admin-secure-password${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl port-forward svc/$SERVICE_NAME -n analytics-platform 3000:3000
}

# Run platform tests
run_tests() {
    ensure_minikube_running
    
    # Setup port forwarding in background
    kubectl port-forward svc/flask-api-service -n analytics-platform 5000:80 &
    API_PF_PID=$!
    
    kubectl port-forward svc/kafka-service -n analytics-platform 9092:9092 &
    KAFKA_PF_PID=$!
    
    # Give port forwarding time to establish
    echo -e "${BLUE}Setting up port forwarding for tests...${NC}"
    sleep 5
    
    echo -e "${BLUE}Running platform tests...${NC}"
    python3 scripts/test-platform.py
    
    # Cleanup
    echo -e "${BLUE}Cleaning up port forwarding...${NC}"
    kill $API_PF_PID $KAFKA_PF_PID 2>/dev/null || true
}

# List available pods
list_pods() {
    ensure_minikube_running
    echo -e "${BLUE}Available pods:${NC}"
    kubectl get pods -n analytics-platform
}

# View logs for a pod
view_logs() {
    if [ -z "$1" ]; then
        echo -e "${RED}Error: Pod name is required${NC}"
        echo "Usage: ./manage.sh logs <pod-name>"
        list_pods
        return 1
    fi
    
    ensure_minikube_running
    
    # Check if pod exists
    if ! kubectl get pod $1 -n analytics-platform &>/dev/null; then
        echo -e "${RED}Error: Pod '$1' not found${NC}"
        list_pods
        return 1
    fi
    
    echo -e "${BLUE}Streaming logs for pod $1...${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl logs -f $1 -n analytics-platform
}

# View events
view_events() {
    ensure_minikube_running
    echo -e "${BLUE}Recent events in the analytics-platform namespace:${NC}"
    kubectl get events -n analytics-platform --sort-by=.metadata.creationTimestamp | tail -20
}

# Fix common issues
fix_common_issues() {
    ensure_minikube_running
    echo -e "${BLUE}Attempting to fix common issues...${NC}"
    
    # 1. Find and delete pods in CrashLoopBackOff or ImagePullBackOff
    echo "Checking for problematic pods..."
    local crash_pods=$(kubectl get pods -n analytics-platform -o jsonpath='{range .items[?(@.status.containerStatuses[0].state.waiting.reason=="CrashLoopBackOff")]}{.metadata.name}{"\n"}{end}')
    local pull_pods=$(kubectl get pods -n analytics-platform -o jsonpath='{range .items[?(@.status.containerStatuses[0].state.waiting.reason=="ImagePullBackOff")]}{.metadata.name}{"\n"}{end}')
    
    if [ -n "$crash_pods" ]; then
        echo -e "${YELLOW}Found pods in CrashLoopBackOff state:${NC}"
        echo "$crash_pods"
        echo -e "${YELLOW}Deleting these pods...${NC}"
        echo "$crash_pods" | xargs kubectl delete pod -n analytics-platform
    fi
    
    if [ -n "$pull_pods" ]; then
        echo -e "${YELLOW}Found pods in ImagePullBackOff state:${NC}"
        echo "$pull_pods"
        echo -e "${YELLOW}Deleting these pods...${NC}"
        echo "$pull_pods" | xargs kubectl delete pod -n analytics-platform
    fi
    
    # 2. Check for duplicate deployments with similar names
    echo "Checking for duplicate deployments..."
    kubectl get deployments -n analytics-platform -o name | cut -d '/' -f 2 | sort
    
    echo -e "${GREEN}Finished checking for common issues${NC}"
    echo -e "${YELLOW}You may need to manually check for and resolve duplicate deployments${NC}"
}

# Restart all deployments
restart_all() {
    ensure_minikube_running
    echo -e "${YELLOW}Are you sure you want to restart all deployments? This may cause temporary downtime. (y/n)${NC}"
    read -r confirmation
    if [[ $confirmation =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Restarting all deployments...${NC}"
        kubectl get deployments -n analytics-platform -o name | xargs -I{} kubectl rollout restart {} -n analytics-platform
        echo -e "${GREEN}All deployments restarted${NC}"
    else
        echo -e "${BLUE}Operation cancelled${NC}"
    fi
}

# Reset the entire project
reset_project() {
    echo -e "${RED}Warning: This will delete ALL Kubernetes resources and clean up unnecessary files. Are you sure? (y/n)${NC}"
    read -r confirmation
    if [[ $confirmation =~ ^[Yy]$ ]]; then
        # Step 1: Delete all Kubernetes resources
        echo -e "${BLUE}Deleting all Kubernetes resources...${NC}"
        kubectl delete namespace analytics-platform --ignore-not-found=true
        
        # Step 2: Clean up Docker images
        echo -e "${BLUE}Cleaning Docker images...${NC}"
        if minikube status &>/dev/null; then
            eval $(minikube docker-env)
            docker system prune -af --volumes
        else
            echo "Minikube not running, skipping Docker cleanup"
        fi
        
        echo -e "${GREEN}Project reset complete!${NC}"
        echo -e "Run ${YELLOW}./manage.sh deploy${NC} to redeploy the platform."
    else
        echo -e "${BLUE}Reset cancelled${NC}"
    fi
}

# Clean up all resources
cleanup_resources() {
    ensure_minikube_running
    
    echo -e "${RED}Warning: This will remove all platform resources. Are you sure? (y/n)${NC}"
    read -r confirmation
    if [[ $confirmation =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Removing all platform resources...${NC}"
        kubectl delete namespace analytics-platform --ignore-not-found=true
        echo -e "${GREEN}Cleanup complete!${NC}"
    else
        echo -e "${BLUE}Cleanup cancelled.${NC}"
    fi
}

fix_config_issues() {
  echo -e "${BLUE}Fixing configuration issues...${NC}"
  
  # Fix ConfigMap
  kubectl create configmap platform-config -n analytics-platform \
    --from-literal=kafka-broker="kafka:9092" \
    --from-literal=kafka-topic="sensor-data" \
    --from-literal=data-ingestion-port="5000" \
    --from-literal=processing-engine-port="5001" \
    --from-literal=storage-layer-port="5002" \
    --from-literal=visualization-port="5003" \
    --from-literal=db-host="localhost" \
    --from-literal=db-port="5432" \
    --from-literal=db-name="analytics" \
    --from-literal=db-user="user" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  # Build flask-api image
  echo -e "${BLUE}Building flask-api image...${NC}"
  eval $(minikube docker-env)
  docker build -t flask-api:latest -f flask-api/Dockerfile ./flask-api
  
  echo -e "${BLUE}Restarting deployments...${NC}"
  kubectl rollout restart deployment -n analytics-platform
  
  echo -e "${GREEN}Configuration fixes applied. Checking status...${NC}"
  sleep 5
  kubectl get pods -n analytics-platform
}

check_config_consistency() {
  echo -e "${BLUE}Checking configuration consistency...${NC}"
  
  # Check if deployment references match configmap keys
  echo -e "\n${BLUE}ConfigMap Keys:${NC}"
  kubectl get configmap platform-config -n analytics-platform -o jsonpath='{.data}' | jq
  
  echo -e "\n${BLUE}Secret Keys:${NC}"
  kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data}' | jq
  
  echo -e "\n${BLUE}Deployment Environment Variable Sources:${NC}"
  kubectl get deployment flask-api -n analytics-platform -o jsonpath='{.spec.template.spec.containers[0].env}'
  
  echo -e "\n${YELLOW}Checking for key mismatches...${NC}"
  # Check secret keys
  API_KEY_1_EXISTS=$(kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data.API_KEY_1}' 2>/dev/null)
  api_key_1_EXISTS=$(kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data.api-key-1}' 2>/dev/null)
  
  if [ -z "$API_KEY_1_EXISTS" ] && [ -z "$api_key_1_EXISTS" ]; then
    echo -e "${RED}Neither API_KEY_1 nor api-key-1 found in secret${NC}"
  elif [ -n "$API_KEY_1_EXISTS" ] && [ -n "$api_key_1_EXISTS" ]; then
    echo -e "${YELLOW}Both API_KEY_1 and api-key-1 found in secret (potential confusion)${NC}"
  elif [ -n "$API_KEY_1_EXISTS" ]; then
    echo -e "${GREEN}API_KEY_1 found in secret${NC}"
  else
    echo -e "${GREEN}api-key-1 found in secret${NC}"
  fi
  
  # Check configmap keys
  KAFKA_BROKER_EXISTS=$(kubectl get configmap platform-config -n analytics-platform -o jsonpath='{.data.kafka-broker}' 2>/dev/null)
  if [ -z "$KAFKA_BROKER_EXISTS" ]; then
    echo -e "${RED}kafka-broker not found in ConfigMap${NC}"
  else
    echo -e "${GREEN}kafka-broker found in ConfigMap: $KAFKA_BROKER_EXISTS${NC}"
  fi
  
  # Check if Flask API env vars are correctly set
  echo -e "\n${BLUE}Flask API Container Environment:${NC}"
  FLASK_POD=$(kubectl get pods -n analytics-platform -l app=flask-api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  if [ -n "$FLASK_POD" ]; then
    kubectl exec -it $FLASK_POD -n analytics-platform -- env | sort
  else
    echo -e "${RED}No Flask API pod found${NC}"
  fi
}

standardize_config() {
  echo -e "${BLUE}Standardizing configuration naming conventions...${NC}"
  
  # 1. Update ConfigMap with standardized naming
  echo -e "${BLUE}Updating ConfigMap with standardized naming...${NC}"
  kubectl create configmap platform-config -n analytics-platform \
    --from-literal=KAFKA_BROKER="kafka:9092" \
    --from-literal=KAFKA_TOPIC="sensor-data" \
    --from-literal=DATA_INGESTION_PORT="5000" \
    --from-literal=PROCESSING_ENGINE_PORT="5001" \
    --from-literal=STORAGE_LAYER_PORT="5002" \
    --from-literal=VISUALIZATION_PORT="5003" \
    --from-literal=DB_HOST="localhost" \
    --from-literal=DB_PORT="5432" \
    --from-literal=DB_NAME="analytics" \
    --from-literal=DB_USER="user" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  # 2. Clean up the Secret to use only uppercase keys
  echo -e "${BLUE}Updating Secret with standardized naming...${NC}"
  kubectl create secret generic api-keys -n analytics-platform \
    --from-literal=API_KEY_1="test-key-1" \
    --from-literal=API_KEY_2="test-key-2" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  # 3. Update deployments to reference the correct key format
  echo -e "${BLUE}Updating deployments to use standardized config keys...${NC}"
  
  # Update flask-api deployment
  kubectl patch deployment flask-api -n analytics-platform --type=json -p '[
    {
      "op": "replace", 
      "path": "/spec/template/spec/containers/0/env/0/valueFrom/configMapKeyRef/key", 
      "value": "KAFKA_BROKER"
    },
    {
      "op": "replace", 
      "path": "/spec/template/spec/containers/0/env/1/valueFrom/configMapKeyRef/key", 
      "value": "KAFKA_TOPIC"
    },
    {
      "op": "replace", 
      "path": "/spec/template/spec/containers/0/env/2/valueFrom/secretKeyRef/key", 
      "value": "API_KEY_1"
    },
    {
      "op": "replace", 
      "path": "/spec/template/spec/containers/0/env/3/valueFrom/secretKeyRef/key", 
      "value": "API_KEY_2"
    }
  ]'
  
  # Update any other deployments that reference these environment variables
  # Add similar patches for other deployments if needed
  
  # 4. Restart deployments to apply changes
  echo -e "${BLUE}Restarting deployments to apply changes...${NC}"
  kubectl rollout restart deployment -n analytics-platform
  
  echo -e "${GREEN}Configuration standardization complete!${NC}"
  echo -e "${YELLOW}Naming convention standard: UPPERCASE_WITH_UNDERSCORES for environment variables${NC}"
  
  # 5. Document the changes
  mkdir -p docs
  cat > docs/naming-conventions.md << 'EOF'
# Naming Conventions for Real-Time Analytics Platform

## Environment Variables

**Standard: UPPERCASE_WITH_UNDERSCORES**

Example:
- `KAFKA_BROKER`
- `KAFKA_TOPIC`
- `API_KEY_1`

## Kubernetes Resources

**Standard: lowercase-with-hyphens**

Example:
- Deployments: `flask-api`, `data-ingestion`
- Services: `kafka`, `flask-api-service`

## Docker Images

**Standard: lowercase-with-hyphens:tag**

Example:
- `flask-api:latest`
- `data-ingestion:latest`

## Ports

**Standard: Port numbers should be consistent between services**

Example:
- Flask API: 5000
- Data Ingestion: 5001

## Documentation

All new components should follow these naming conventions to maintain consistency across the platform.
EOF
  
  echo -e "${GREEN}Naming conventions documented in docs/naming-conventions.md${NC}"
}


# Main logic
case "$1" in
    status)
        show_status
        ;;
    start)
        ensure_minikube_running
        ;;
    stop)
        echo -e "${BLUE}Stopping minikube...${NC}"
        minikube stop
        echo -e "${GREEN}Minikube stopped.${NC}"
        ;;
    deploy)
        deploy_platform
        ;;
    monitoring)
        deploy_monitoring
        ;;
    dashboard)
        open_dashboard
        ;;
    prometheus)
        access_prometheus
        ;;
    grafana)
        access_grafana
        ;;
    test)
        run_tests
        ;;
    pods)
        list_pods
        ;;
    logs)
        view_logs "$2"
        ;;
    describe)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Resource name is required${NC}"
            echo "Usage: ./manage.sh describe <resource-name>"
            exit 1
        fi
        kubectl describe pod "$2" -n analytics-platform
        ;;
    restart)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Deployment name is required${NC}"
            echo "Usage: ./manage.sh restart <deployment-name>"
            echo -e "\n${BLUE}Available deployments:${NC}"
            kubectl get deployments -n analytics-platform
            exit 1
        fi
        kubectl rollout restart deployment/"$2" -n analytics-platform
        echo -e "${GREEN}Deployment $2 restarted${NC}"
        ;;
    restart-all)
        restart_all
        ;;
    events)
        view_events
        ;;
    fix)
        fix_common_issues
        ;;
    fix-config)
        fix_config_issues
        ;;     
    check-config)
        check_config_consistency
        ;;        
    standardize-config)
        standardize_config
        ;;           
    reset)
        reset_project
        ;;
    cleanup)
        cleanup_resources
        ;;
    help|*)
        show_help
        ;;
esac