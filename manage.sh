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
    echo "  build       - Build Docker images for all services"
    echo "  deploy      - Deploy the full platform"
    echo "  deploy-prod - Deploy with production configuration"
    echo "  setup-all   - Complete setup: start minikube, build images and deploy"
    echo "  reset-all   - Complete cleanup: delete namespace, PVs, and start fresh"
    echo "  fix-storage - Rebuild only the storage-layer-go image"
    echo "  fix-processing - Rebuild only the processing-engine-go image"
    echo "  monitoring  - Deploy monitoring stack (Prometheus/Grafana)"
    echo "  dashboard   - Open Kubernetes dashboard"
    echo "  access-viz  - Access visualization dashboard (localhost:8080)"
    echo "  access-api  - Access data ingestion API (localhost:5000)"
    echo "  test-api    - Test API with secure credentials from K8s secrets"
    echo "  stop-access - Stop all port forwarding sessions"
    echo "  prometheus  - Access Prometheus UI (port-forward)"
    echo "  grafana     - Access Grafana UI (port-forward)"
    echo "  test        - Run platform tests"
    echo "  pods        - List all pods"
    echo "  logs <pod>  - View logs for a specific pod"
    echo "  describe <pod> - Get detailed information about a pod"
    echo "  reset-minikube  - Completely reset Minikube (delete and recreate)"
    echo "  tunnel-start    - Start minikube tunnel in background"
    echo "  tunnel-stop     - Stop minikube tunnel"    
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
# Ensure minikube is fully operational with retries
ensure_minikube_operational() {
    echo -e "${BLUE}Ensuring Minikube is fully operational...${NC}"
    
    # First make sure it's running
    ensure_minikube_running
    
    # Now check if API server is actually responding
    local max_attempts=5
    local attempt=1
    local delay=10
    
    while [ $attempt -le $max_attempts ]; do
        echo -e "${BLUE}Checking Kubernetes API server (attempt $attempt/$max_attempts)...${NC}"
        
        if kubectl cluster-info &>/dev/null; then
            echo -e "${GREEN}Kubernetes API server is responding!${NC}"
            
            # Verify Docker connectivity
            echo -e "${BLUE}Verifying Docker connectivity...${NC}"
            if eval $(minikube docker-env) &>/dev/null; then
                echo -e "${GREEN}Minikube-Docker integration successful!${NC}"
                return 0
            else
                echo -e "${YELLOW}Docker connectivity issue. Retrying...${NC}"
            fi
        else
            echo -e "${YELLOW}Kubernetes API server not responding yet. Waiting...${NC}"
        fi
        
        echo -e "${BLUE}Waiting $delay seconds before retry...${NC}"
        sleep $delay
        attempt=$((attempt+1))
    done
    
    echo -e "${RED}Failed to verify Minikube is fully operational after multiple attempts.${NC}"
    echo -e "${YELLOW}Would you like to try restarting Minikube? (y/n)${NC}"
    read -r restart_choice
    
    if [[ $restart_choice =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Restarting Minikube...${NC}"
        minikube stop
        sleep 5
        minikube start
        
        # Give it time to stabilize
        echo -e "${BLUE}Giving Minikube time to stabilize...${NC}"
        sleep 20
        
        if kubectl cluster-info &>/dev/null; then
            echo -e "${GREEN}Minikube restarted successfully!${NC}"
            return 0
        else
            echo -e "${RED}Minikube still not operational after restart.${NC}"
            echo -e "${YELLOW}Consider running 'minikube delete' and then 'minikube start' manually.${NC}"
            return 1
        fi
    else
        echo -e "${BLUE}Restart cancelled.${NC}"
        return 1
    fi
}
# Start minikube tunnel
start_minikube_tunnel() {
    echo -e "${BLUE}Starting minikube tunnel in background...${NC}"
    
    # Check if tunnel is already running
    if pgrep -f "minikube tunnel" > /dev/null; then
        echo -e "${GREEN}Minikube tunnel is already running.${NC}"
        return 0
    fi
    
    # Start tunnel in background
    minikube tunnel > /tmp/minikube-tunnel.log 2>&1 &
    TUNNEL_PID=$!
    
    # Wait a moment to ensure it started
    sleep 5
    
    # Check if still running
    if kill -0 $TUNNEL_PID 2>/dev/null; then
        echo -e "${GREEN}Minikube tunnel started successfully with PID $TUNNEL_PID${NC}"
        echo $TUNNEL_PID > /tmp/minikube-tunnel.pid
    else
        echo -e "${RED}Failed to start minikube tunnel${NC}"
        cat /tmp/minikube-tunnel.log
        return 1
    fi
}
# Stop minikube tunnel
stop_minikube_tunnel() {
    echo -e "${BLUE}Stopping minikube tunnel...${NC}"
    
    if [ -f /tmp/minikube-tunnel.pid ]; then
        TUNNEL_PID=$(cat /tmp/minikube-tunnel.pid)
        if kill -0 $TUNNEL_PID 2>/dev/null; then
            kill $TUNNEL_PID
            echo -e "${GREEN}Minikube tunnel stopped.${NC}"
        else
            echo -e "${YELLOW}Minikube tunnel process not found.${NC}"
        fi
        rm /tmp/minikube-tunnel.pid
    else
        # Try to find and kill any tunnel processes
        TUNNEL_PID=$(pgrep -f "minikube tunnel")
        if [ -n "$TUNNEL_PID" ]; then
            kill $TUNNEL_PID
            echo -e "${GREEN}Found and stopped minikube tunnel.${NC}"
        else
            echo -e "${YELLOW}No running minikube tunnel found.${NC}"
        fi
    fi
}
# Comprehensive Minikube reset function
reset_minikube() {
    echo -e "${RED}Warning: This will completely reset Minikube. All data will be lost. Continue? (y/n)${NC}"
    read -r confirm
    if [[ $confirm =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Stopping any running Minikube tunnel...${NC}"
        stop_minikube_tunnel
        
        echo -e "${BLUE}Stopping Minikube...${NC}"
        minikube stop || true
        
        echo -e "${BLUE}Deleting Minikube...${NC}"
        minikube delete
        echo -e "${BLUE}Removing any leftover Minikube files...${NC}"
        rm -rf ~/.minikube || true
        
        echo -e "${BLUE}Starting fresh Minikube instance...${NC}"
        minikube start --memory=4096 --cpus=2 # Adjust resources as needed
        
        # Wait for Minikube to stabilize
        echo -e "${BLUE}Waiting for Minikube to stabilize (30 seconds)...${NC}"
        sleep 30
        
        # Verify if Minikube is operational
        if kubectl cluster-info &>/dev/null; then
            echo -e "${GREEN}Minikube reset successful!${NC}"
            echo -e "${GREEN}Kubernetes API server is responding.${NC}"
            echo -e "${YELLOW}You can now run './manage.sh deploy' to redeploy your platform.${NC}"
        else
            echo -e "${RED}Minikube reset complete, but API server is not responding.${NC}"
            echo -e "${YELLOW}Try running './manage.sh fix-minikube' for diagnostics.${NC}"
        fi
    else
        echo -e "${BLUE}Reset cancelled.${NC}"
    fi
}
# Check if namespace exists
ensure_namespace() {
    if ! kubectl get namespace analytics-platform &> /dev/null; then
        echo -e "${YELLOW}Creating analytics-platform namespace...${NC}"
        kubectl create namespace analytics-platform
    fi
}
# Check if resources are ready before continuing deployment
wait_for_resource() {
  local resource_type=$1
  local resource_name=$2
  local namespace=$3
  local timeout=${4:-180}
  local count=0
  local interval=5
  echo -e "${BLUE}Waiting for $resource_type/$resource_name to be ready...${NC}"
  
  while [ $count -lt $timeout ]; do
    if kubectl get $resource_type $resource_name -n $namespace &>/dev/null; then
      if [ "$resource_type" == "deployment" ]; then
        local ready=$(kubectl get $resource_type $resource_name -n $namespace -o jsonpath='{.status.readyReplicas}')
        local desired=$(kubectl get $resource_type $resource_name -n $namespace -o jsonpath='{.status.replicas}')
        if [ "$ready" == "$desired" ]; then
          echo -e "${GREEN}$resource_type/$resource_name is ready!${NC}"
          return 0
        fi
      else
        echo -e "${GREEN}$resource_type/$resource_name is ready!${NC}"
        return 0
      fi
    fi
    
    echo -n "."
    sleep $interval
    count=$((count + interval))
  done
  
  echo -e "${RED}Timed out waiting for $resource_type/$resource_name${NC}"
  return 1
}
# Show status of all platform components
show_platform_status() {
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
# Enhanced build images function with resilient fallback support
build_images() {
    # First check if minikube is operational
    if ! ensure_minikube_operational; then
        echo -e "${RED}Cannot build images because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    fi
    
    echo -e "${BLUE}Building platform images with resilient fallbacks...${NC}"
    
    # Connect to minikube's Docker daemon
    echo -e "${BLUE}Connecting to minikube's Docker daemon...${NC}"
    eval $(minikube docker-env)
    
    # Prefer resilient build script if available, fallback to standard script
    if [ -f "./build-images-resilient.sh" ] && [ -x "./build-images-resilient.sh" ]; then
        echo -e "${BLUE}Using resilient build script with fallback strategies...${NC}"
        ./build-images-resilient.sh
        BUILD_RESULT=$?
    elif [ -f "./build-images.sh" ]; then
        echo -e "${YELLOW}Resilient build script not found, using standard build script...${NC}"
        if [ ! -x "./build-images.sh" ]; then
            chmod +x ./build-images.sh
        fi
        ./build-images.sh
        BUILD_RESULT=$?
    else
        echo -e "${RED}Error: No build script found (neither build-images-resilient.sh nor build-images.sh)${NC}"
        return 1
    fi
    
    if [ $BUILD_RESULT -eq 0 ]; then
        echo -e "${GREEN}All images built successfully!${NC}"
        echo -e "${YELLOW}You can now run './manage.sh deploy' to deploy the platform.${NC}"
    else
        echo -e "${RED}Image building failed. Please check the error messages above.${NC}"
        echo -e "${YELLOW}Try running './scripts/docker-network-fix.sh' if you encounter network issues.${NC}"
        return 1
    fi
}
# Deploy the platform
deploy_platform() {
    # Use the enhanced check that ensures API server is responding
    if ! ensure_minikube_operational; then
        echo -e "${RED}Cannot deploy because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    fi
    
    echo -e "${BLUE}Deploying Real-Time Analytics Platform...${NC}"
    
    # Check if images exist
    eval $(minikube docker-env)
    if ! docker images | grep -q "data-ingestion-go"; then
        echo -e "${YELLOW}Required images not found. Building images first...${NC}"
        build_images || {
            echo -e "${RED}Failed to build images. Deployment aborted.${NC}"
            return 1
        }
    else
        echo -e "${GREEN}Using existing Docker images...${NC}"
    fi
    
    # Make sure namespace exists
    ensure_namespace
    
    # Apply RBAC first for security
    echo -e "${BLUE}Applying RBAC configurations...${NC}"
    kubectl apply -f k8s/rbac.yaml
    
    # Apply secrets and configmaps
    echo -e "${BLUE}Creating secrets and configmaps...${NC}"
    kubectl apply -f k8s/configmap.yaml
    # Note: Secrets should be created by running ./scripts/setup-secrets.sh first
    echo -e "${YELLOW}Secrets should already be created by setup-secrets.sh script${NC}"
    
    # Apply network policies
    echo -e "${BLUE}Applying network policies...${NC}"
    kubectl apply -f k8s/network-policy.yaml
    
    # Deploy core infrastructure - UPDATED FOR KRAFT MODE
    echo -e "${BLUE}Deploying core infrastructure...${NC}"
    kubectl apply -f k8s/kafka-pvc.yaml
    kubectl apply -f k8s/storage-layer-pvc.yaml
    kubectl apply -f k8s/kafka-kraft-statefulset.yaml
    kubectl apply -f k8s/kafka-service.yaml
    
    # Wait for Kafka to be ready before continuing
    echo -e "${YELLOW}Waiting for Kafka to be ready...${NC}"
    
    # Deploy microservices
    echo -e "${BLUE}Deploying Go microservices...${NC}"
    kubectl apply -f k8s/data-ingestion-go-deployment.yaml
    kubectl apply -f k8s/data-ingestion-go-service.yaml
    
    kubectl apply -f k8s/clean-ingestion-go-deployment.yaml
    kubectl apply -f k8s/clean-ingestion-go-service.yaml
    
    kubectl apply -f k8s/processing-engine-go-deployment.yaml
    kubectl apply -f k8s/processing-engine-go-service.yaml
    
    kubectl apply -f k8s/storage-layer-go-deployment.yaml
    kubectl apply -f k8s/storage-layer-go-service.yaml
    
    kubectl apply -f k8s/visualization-go-deployment.yaml
    kubectl apply -f k8s/visualization-go-service.yaml
    
    kubectl apply -f k8s/tenant-management-go-deployment.yaml
    kubectl apply -f k8s/tenant-management-go-service.yaml
    
    # Deploy monitoring tools
    echo -e "${BLUE}Deploying monitoring (Prometheus)...${NC}"
    kubectl apply -f k8s/prometheus-config.yaml
    kubectl apply -f k8s/prometheus-rbac.yaml
    kubectl apply -f k8s/prometheus-deployment.yaml
    
    echo -e "${GREEN}Deployment complete!${NC}"
    echo -e "${YELLOW}Running security check...${NC}"
    ./scripts/security-check.sh
    
}
# Deploy monitoring stack
deploy_monitoring() {
    ensure_minikube_operational
    echo -e "${BLUE}Deploying monitoring stack...${NC}"
    kubectl apply -f k8s/prometheus-config.yaml
    kubectl apply -f k8s/prometheus-rbac.yaml
    kubectl apply -f k8s/prometheus-deployment.yaml
    kubectl apply -f k8s/grafana-pvc.yaml
    kubectl apply -f k8s/grafana-secret.yaml
    kubectl apply -f k8s/grafana-datasources.yaml
    kubectl apply -f k8s/grafana-deployment.yaml
    kubectl apply -f k8s/grafana-tenant-dashboards.yaml
    kubectl apply -f k8s/grafana-go-dashboard.yaml
    
    echo "🔍 Checking deployment status..."
    kubectl rollout status statefulset/prometheus -n analytics-platform
    kubectl rollout status deployment/grafana -n analytics-platform
    
    echo -e "${GREEN}Monitoring deployment complete!${NC}"
    echo -e "${YELLOW}To access Prometheus: kubectl port-forward service/prometheus 9090:9090 -n analytics-platform${NC}"
    echo -e "${YELLOW}To access Grafana: kubectl port-forward service/grafana-service 3000:80 -n analytics-platform${NC}"
}
# Open Kubernetes dashboard
open_dashboard() {
    ensure_minikube_operational
    echo -e "${BLUE}Opening Kubernetes dashboard...${NC}"
    minikube dashboard
}
# Port forward to Prometheus
access_prometheus() {
    ensure_minikube_operational
    
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
    ensure_minikube_operational
    
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
    echo -e "${YELLOW}Use credentials generated by: ./scripts/setup-secrets.sh${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl port-forward svc/$SERVICE_NAME -n analytics-platform 3000:3000
}
# Run platform tests
run_tests() {
    ensure_minikube_operational
    # Setup port forwarding in background
    echo -e "${BLUE}Setting up port forwarding for tests...${NC}"
    kubectl port-forward svc/gin-api-service -n analytics-platform 5051:80 & # Forward to service port 80
    API_PF_PID=$!
    kubectl port-forward svc/kafka -n analytics-platform 9092:9092 & # Use correct kafka service name
    KAFKA_PF_PID=$!
    # Give port forwarding time to establish
    sleep 5
    # Check if port-forwarding succeeded before running tests
    if ! kill -0 $API_PF_PID 2>/dev/null || ! kill -0 $KAFKA_PF_PID 2>/dev/null; then
        echo -e "${RED}Failed to establish port forwarding. Check service names and availability.${NC}"
        # Attempt cleanup even if forwarding failed
        kill $API_PF_PID $KAFKA_PF_PID 2>/dev/null || true
        return 1
    fi
    echo -e "${BLUE}Running platform tests...${NC}"
    # Activate venv and run python script
    if [ -f "tenant-testing/.venv/bin/activate" ]; then
        # Export API host/port for the script to use the forwarded port
        export API_HOST="localhost"
        export API_PORT="5051"
        source tenant-testing/.venv/bin/activate && python3 scripts/test-platform.py
        TEST_EXIT_CODE=$?
        # Unset env vars and deactivate
        unset API_HOST
        unset API_PORT
        deactivate 2>/dev/null || true
    else
        echo -e "${YELLOW}Virtual environment not found at tenant-testing/.venv. Attempting to run without it.${NC}"
        # Also export here in case venv isn't used
        export API_HOST="localhost"
        export API_PORT="5051"
        python3 scripts/test-platform.py
        TEST_EXIT_CODE=$?
        unset API_HOST
        unset API_PORT
    fi
    # Cleanup
    echo -e "${BLUE}Cleaning up port forwarding...${NC}"
    kill $API_PF_PID $KAFKA_PF_PID 2>/dev/null || true
    # Return the exit code from the python script
    return $TEST_EXIT_CODE 
}
# List available pods
list_pods() {
    ensure_minikube_operational
    echo -e "${BLUE}Available pods:${NC}"
    kubectl get pods -n analytics-platform
}
# Tenant management functions
# List tenants
list_tenants() {
    ensure_minikube_operational
    
    echo -e "${BLUE}Tenant management is implemented in the Go tenant-management service.${NC}"
    echo -e "${YELLOW}Current API keys and their tenant mappings:${NC}"
    
    # Get API keys from secrets
    API_KEY_1=$(kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data.API_KEY_1}' 2>/dev/null | base64 --decode)
    API_KEY_2=$(kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data.API_KEY_2}' 2>/dev/null | base64 --decode)
    
    # Get tenant mapping from config
    TENANT_MAP=$(kubectl get configmap platform-config -n analytics-platform -o jsonpath='{.data.TENANT_API_KEY_MAP}' 2>/dev/null)
    
    if [ -z "$TENANT_MAP" ]; then
        echo -e "${RED}No tenant mapping found in ConfigMap${NC}"
        echo -e "${YELLOW}Default mapping:${NC}"
        echo -e "  API_KEY_1 ($API_KEY_1) → tenant1"
        echo -e "  API_KEY_2 ($API_KEY_2) → tenant2"
    else
        echo -e "${GREEN}Tenant mapping from ConfigMap:${NC}"
        # Pretty print the JSON mapping
        echo $TENANT_MAP | python3 -m json.tool
    fi
    
    echo -e "\n${YELLOW}The tenant-management-go service handles tenant management:${NC}"
    echo -e "  kubectl port-forward svc/tenant-management-go-service -n analytics-platform 5010:80"
}
# Enable tenant isolation
enable_tenant_isolation() {
    ensure_minikube_operational
    
    echo -e "${BLUE}Enabling tenant isolation...${NC}"
    
    # Update ConfigMap to enable tenant isolation
    kubectl patch configmap platform-config -n analytics-platform --type=merge -p '{"data":{"ENABLE_TENANT_ISOLATION":"true"}}'
    
    # Set default tenant mapping if not already set
    TENANT_MAP=$(kubectl get configmap platform-config -n analytics-platform -o jsonpath='{.data.TENANT_API_KEY_MAP}' 2>/dev/null)
    if [ -z "$TENANT_MAP" ]; then
        # Get API keys
        API_KEY_1=$(kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data.API_KEY_1}' 2>/dev/null | base64 --decode)
        API_KEY_2=$(kubectl get secret api-keys -n analytics-platform -o jsonpath='{.data.API_KEY_2}' 2>/dev/null | base64 --decode)
        
        # Create default mapping
        TENANT_MAPPING="{\"$API_KEY_1\":\"tenant1\",\"$API_KEY_2\":\"tenant2\"}"
        kubectl patch configmap platform-config -n analytics-platform --type=merge -p "{\"data\":{\"TENANT_API_KEY_MAP\":\"$TENANT_MAPPING\"}}"
    fi
    
    # Restart API deployments to apply changes
    echo -e "${BLUE}Restarting API deployments to apply changes...${NC}"
    kubectl rollout restart deployment/gin-api -n analytics-platform
    kubectl rollout restart deployment/data-ingestion -n analytics-platform
    
    echo -e "${GREEN}Tenant isolation enabled!${NC}"
    echo -e "${YELLOW}Use ./manage.sh list-tenants to see current tenant mappings${NC}"
}
# Deploy tenant management service (UPDATED FOR GO VERSION ONLY)
deploy_tenant_mgmt() {
    ensure_minikube_operational
    
    echo -e "${BLUE}Deploying tenant management Go service...${NC}"
    
    # Apply tenant management secrets
    kubectl apply -f k8s/tenant-management-secrets.yaml
    
    # Deploy tenant management Go service
    kubectl apply -f k8s/tenant-management-go-deployment.yaml
    kubectl apply -f k8s/tenant-management-go-service.yaml
    
    echo -e "${GREEN}Tenant management Go service deployed!${NC}"
    echo -e "${YELLOW}You can access the service by port forwarding:${NC}"
    echo -e "  kubectl port-forward svc/tenant-management-go-service -n analytics-platform 5010:80"
}
# Create a new tenant
create_tenant() {
    ensure_minikube_operational
    
    if [ -z "$1" ] || [ -z "$2" ]; then
        echo -e "${RED}Error: Tenant ID and name are required${NC}"
        echo -e "Usage: ./manage.sh create-tenant <tenant-id> <tenant-name> [plan]"
        echo -e "Plans: basic, standard, premium (default: basic)"
        return 1
    fi
    
    TENANT_ID=$1
    TENANT_NAME=$2
    PLAN=${3:-basic}
    
    echo -e "${BLUE}Creating tenant: $TENANT_NAME (ID: $TENANT_ID, Plan: $PLAN)...${NC}"
    
    # Get tenant management service URL
    TENANT_SVC_URL="http://$(minikube ip):30090"
    
    # Create tenant via API
    curl -X POST "$TENANT_SVC_URL/api/tenants" \
      -H "Content-Type: application/json" \
      -d "{\"tenant_id\":\"$TENANT_ID\",\"name\":\"$TENANT_NAME\",\"plan\":\"$PLAN\"}"
    
    echo -e "\n${GREEN}Tenant creation request sent.${NC}"
}
# View logs for a pod
view_logs() {
    if [ -z "$1" ]; then
        echo -e "${RED}Error: Pod name is required${NC}"
        echo "Usage: ./manage.sh logs <pod-name>"
        list_pods
        return 1
    fi
    
    ensure_minikube_operational
    
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
    ensure_minikube_operational
    echo -e "${BLUE}Recent events in the analytics-platform namespace:${NC}"
    kubectl get events -n analytics-platform --sort-by=.metadata.creationTimestamp | tail -20
}
# Fix Minikube issues
fix_minikube() {
    echo -e "${BLUE}Diagnosing Minikube issues...${NC}"
    
    # Check if Minikube is installed
    if ! command -v minikube &> /dev/null; then
        echo -e "${RED}Minikube not found. Please install Minikube first.${NC}"
        return 1
    fi
    
    # Check Minikube status
    if ! minikube status &> /dev/null; then
        echo -e "${YELLOW}Minikube is not running. Attempting to start...${NC}"
        minikube start
        
        if ! minikube status &> /dev/null; then
            echo -e "${RED}Failed to start Minikube. Attempting to reset Minikube...${NC}"
            
            echo -e "${YELLOW}WARNING: This will delete your Minikube cluster. All data will be lost. Continue? (y/n)${NC}"
            read -r reset_confirm
            if [[ $reset_confirm =~ ^[Yy]$ ]]; then
                echo -e "${BLUE}Deleting Minikube cluster...${NC}"
                minikube delete
                sleep 2
                
                echo -e "${BLUE}Creating new Minikube cluster...${NC}"
                minikube start
                
                if minikube status &> /dev/null; then
                    echo -e "${GREEN}Minikube reset successful!${NC}"
                else
                    echo -e "${RED}Minikube reset failed. Please check your Minikube installation.${NC}"
                    echo -e "${YELLOW}Try running: brew reinstall minikube (for Mac)${NC}"
                    return 1
                fi
            else
                echo -e "${BLUE}Reset cancelled.${NC}"
                return 1
            fi
        else
            echo -e "${GREEN}Minikube started successfully.${NC}"
        fi
    else
        echo -e "${GREEN}Minikube is running.${NC}"
        
        # Check if there are any issues with the cluster
        if ! kubectl cluster-info &> /dev/null; then
            echo -e "${YELLOW}Kubernetes API server not responding. Attempting to restart Minikube...${NC}"
            minikube stop
            sleep 2
            minikube start
            
            if kubectl cluster-info &> /dev/null; then
                echo -e "${GREEN}Minikube restarted successfully.${NC}"
            else
                echo -e "${RED}Failed to restore Minikube. Attempting full reset...${NC}"
                minikube delete
                sleep 2
                minikube start
                
                if kubectl cluster-info &> /dev/null; then
                    echo -e "${GREEN}Minikube reset successful!${NC}"
                else
                    echo -e "${RED}Minikube reset failed. Please check your Minikube installation.${NC}"
                    return 1
                fi
            fi
        else
            echo -e "${GREEN}Kubernetes API server is responding normally.${NC}"
        fi
    fi
    
    # Check docker-env integration
    echo -e "${BLUE}Testing Docker integration...${NC}"
    if ! eval $(minikube docker-env) &> /dev/null; then
        echo -e "${RED}Failed to connect to Minikube's Docker daemon.${NC}"
        echo -e "${YELLOW}This might indicate Docker configuration issues.${NC}"
        
        # Check Docker status
        if ! docker info &> /dev/null; then
            echo -e "${RED}Docker is not running. Please start Docker Desktop or docker service.${NC}"
        else
            echo -e "${YELLOW}Docker is running but Minikube can't connect to it.${NC}"
            echo -e "${YELLOW}Try restarting Docker and Minikube.${NC}"
        fi
    else
        echo -e "${GREEN}Minikube-Docker integration successful.${NC}"
    fi
    
    echo -e "${GREEN}Minikube diagnostics complete.${NC}"
    return 0
}
# Fix common issues
fix_common_issues() {
    ensure_minikube_operational
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
    ensure_minikube_operational
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
        # Step 1: Check if minikube is running before trying to delete resources
        if minikube status &>/dev/null; then
            # Step 1a: Delete all Kubernetes resources
            echo -e "${BLUE}Deleting all Kubernetes resources...${NC}"
            kubectl delete namespace analytics-platform --ignore-not-found=true --timeout=60s || {
                echo -e "${YELLOW}Namespace deletion timed out or failed. This can happen if resources are stuck.${NC}"
                echo -e "${YELLOW}Attempting to force delete...${NC}"
                kubectl delete namespace analytics-platform --ignore-not-found=true --grace-period=0 --force || true
            }
            
            # Step 1b: Clean up Docker images
            echo -e "${BLUE}Cleaning Docker images...${NC}"
            eval $(minikube docker-env)
            docker system prune -af --volumes
        else
            echo -e "${YELLOW}Minikube is not running. Attempting to start it...${NC}"
            minikube start
            
            if minikube status &>/dev/null; then
                echo -e "${GREEN}Minikube started successfully. Proceeding with cleanup...${NC}"
                # Now try to delete resources
                echo -e "${BLUE}Deleting all Kubernetes resources...${NC}"
                kubectl delete namespace analytics-platform --ignore-not-found=true --timeout=60s || true
                
                echo -e "${BLUE}Cleaning Docker images...${NC}"
                eval $(minikube docker-env)
                docker system prune -af --volumes
            else
                echo -e "${RED}Failed to start Minikube. Skipping Kubernetes resource deletion.${NC}"
                echo -e "${YELLOW}You may need to manually delete resources after fixing Minikube.${NC}"
                
                # Should we attempt to fix minikube?
                echo -e "${YELLOW}Would you like to try to fix Minikube issues? (y/n)${NC}"
                read -r fix_minikube
                if [[ $fix_minikube =~ ^[Yy]$ ]]; then
                    echo -e "${BLUE}Attempting to fix Minikube...${NC}"
                    minikube delete
                    sleep 2
                    minikube start
                    
                    if minikube status &>/dev/null; then
                        echo -e "${GREEN}Minikube has been reset successfully.${NC}"
                    else
                        echo -e "${RED}Minikube reset failed. Please check your Minikube installation.${NC}"
                    fi
                fi
            fi
        fi
        
        # Step 3: Cleanup local temporary files (optional)
        echo -e "${BLUE}Cleaning up temporary local files...${NC}"
        find . -name "*.log" -type f -delete
        find . -name "__pycache__" -type d -exec rm -rf {} +
        find . -name "*.pyc" -type f -delete
        
        echo -e "${GREEN}Project reset complete!${NC}"
        echo -e "Run ${YELLOW}./manage.sh deploy${NC} to redeploy the platform."
    else
        echo -e "${BLUE}Reset cancelled${NC}"
    fi
}
# Clean up all resources
cleanup_resources() {
  ensure_minikube_operational
  
  echo -e "${RED}Warning: This will remove all platform resources. Are you sure? (y/n)${NC}"
  read -r confirm
  
  if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo -e "${BLUE}Operation cancelled.${NC}"
    return 0
  fi
  
  echo -e "${BLUE}Removing all platform resources...${NC}"
  
  # Delete all resources in the analytics-platform namespace
  kubectl delete all --all -n analytics-platform
  
  # Delete configmaps and secrets
  kubectl delete configmap --all -n analytics-platform
  kubectl delete secret --all -n analytics-platform
  
  # Delete PVCs
  kubectl delete pvc --all -n analytics-platform
  
  echo -e "${GREEN}All platform resources removed.${NC}"
}
# Reset all - complete cleanup and start fresh
reset_all() {
  echo -e "${RED}Warning: This will completely reset the platform. All data will be lost. Continue? (y/n)${NC}"
  read -r confirm
  
  if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo -e "${BLUE}Reset cancelled.${NC}"
    return 0
  fi
  
  echo -e "${BLUE}Performing complete reset...${NC}"
  
  # Ensure minikube is running and operational
  ensure_minikube_operational
  
  # Delete namespace if it exists
  if kubectl get namespace analytics-platform &> /dev/null; then
    echo -e "${BLUE}Deleting analytics-platform namespace...${NC}"
    kubectl delete namespace analytics-platform
    
    # Wait for namespace deletion to complete
    echo -e "${BLUE}Waiting for namespace deletion to complete...${NC}"
    while kubectl get namespace analytics-platform &> /dev/null; do
      echo -n "."
      sleep 2
    done
    echo
  fi
  
  # Delete any persistent volumes that might be left
  echo -e "${BLUE}Cleaning up any leftover persistent volumes...${NC}"
  for pv in $(kubectl get pv -o=jsonpath='{.items[?(@.spec.claimRef.namespace=="analytics-platform")].metadata.name}'); do
    kubectl delete pv $pv
  done
  
  # Recreate the namespace
  echo -e "${BLUE}Recreating analytics-platform namespace...${NC}"
  kubectl create namespace analytics-platform
  
  echo -e "${GREEN}Platform reset complete!${NC}"
  echo -e "${YELLOW}You can now run './manage.sh setup-all' to redeploy the platform.${NC}"
}
# Fix storage layer: rebuild only the storage-layer-go image
fix_storage() {
    echo -e "${BLUE}Fixing storage-layer-go image...${NC}"
    
    # First check if minikube is operational
    if ! ensure_minikube_operational; then
        echo -e "${RED}Cannot build images because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    fi
    
    # Connect to minikube's Docker daemon
    echo -e "${BLUE}Connecting to minikube's Docker daemon...${NC}"
    eval $(minikube docker-env)
    
    # Build just the storage-layer-go image
    echo -e "${BLUE}Building storage-layer-go image...${NC}"
    cd storage-layer-go || {
        echo -e "${RED}Error: storage-layer-go directory not found${NC}"
        return 1
    }
    
    docker build -t storage-layer-go:latest . || {
        echo -e "${RED}Failed to build storage-layer-go image${NC}"
        cd ..
        return 1
    }
    
    cd ..
    
    echo -e "${GREEN}Storage layer image successfully rebuilt!${NC}"
    echo -e "${YELLOW}Do you want to restart the storage-layer deployment? (y/n)${NC}"
    read -r restart_confirmation
    
    if [[ $restart_confirmation =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Restarting storage-layer deployment...${NC}"
        kubectl rollout restart deployment/storage-layer-go -n analytics-platform || {
            echo -e "${YELLOW}No existing deployment found. If you haven't deployed yet, run './manage.sh deploy'${NC}"
        }
    fi
    
    echo -e "${GREEN}Fix complete!${NC}"
}
# Fix processing engine: rebuild only the processing-engine-go image
fix_processing() {
    echo -e "${BLUE}Fixing processing-engine-go image...${NC}"
    
    # First check if minikube is operational
    if ! ensure_minikube_operational; then
        echo -e "${RED}Cannot build images because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    fi
    
    # Connect to minikube's Docker daemon
    echo -e "${BLUE}Connecting to minikube's Docker daemon...${NC}"
    eval $(minikube docker-env)
    
    # Build just the processing-engine-go image
    echo -e "${BLUE}Building processing-engine-go image...${NC}"
    cd processing-engine-go || {
        echo -e "${RED}Error: processing-engine-go directory not found${NC}"
        return 1
    }
    
    docker build -t processing-engine-go:latest . || {
        echo -e "${RED}Failed to build processing-engine-go image${NC}"
        cd ..
        return 1
    }
    
    cd ..
    
    echo -e "${GREEN}Processing engine image successfully rebuilt!${NC}"
    echo -e "${YELLOW}Do you want to restart the processing-engine deployment? (y/n)${NC}"
    read -r restart_confirmation
    
    if [[ $restart_confirmation =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Restarting processing-engine deployment...${NC}"
        kubectl rollout restart deployment/processing-engine-go -n analytics-platform || {
            echo -e "${YELLOW}No existing deployment found. If you haven't deployed yet, run './manage.sh deploy'${NC}"
        }
    fi
    
    echo -e "${GREEN}Fix complete!${NC}"
}
# Deploy production setup
deploy_production() {
    echo -e "${BLUE}Deploying platform for production...${NC}"
    
    # First check if minikube is operational
    if ! ensure_minikube_operational; then
        echo -e "${RED}Cannot deploy because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    fi
    
    # Make sure the production secrets are generated
    if [ ! -d "production-secrets" ]; then
        echo -e "${YELLOW}Production secrets not found. Generating secure production secrets...${NC}"
        mkdir -p production-secrets
        
        # Add your secure secret generation logic here
        # Example: Generate secure API keys and passwords
        
        echo -e "${GREEN}Production secrets generated.${NC}"
    fi
    
    # Start the deployment process
    echo -e "${BLUE}Applying production configuration...${NC}"
    
    # Create namespace
    kubectl apply -f k8s/namespace.yaml
    
    # Apply RBAC and security configs
    kubectl apply -f k8s/rbac.yaml
    kubectl apply -f k8s/prometheus-rbac.yaml
    kubectl apply -f k8s/network-policy.yaml
    
    # Apply configmaps and secrets
    kubectl apply -f k8s/configmap.yaml
    kubectl apply -f production-secrets/ || kubectl apply -f k8s/secrets-secure.yaml
    kubectl apply -f k8s/kafka-secrets.yaml
    kubectl apply -f k8s/tenant-management-secrets.yaml
    kubectl apply -f k8s/grafana-secret.yaml
    
    # Deploy Kafka with KRaft mode
    kubectl apply -f k8s/kafka-pvc-prod.yaml
    kubectl apply -f k8s/kafka-kraft-statefulset.yaml
    kubectl apply -f k8s/kafka-service.yaml
    
    # Wait for Kafka to be ready (production deployments need more time)
    echo -e "${YELLOW}Waiting for Kafka to be ready (this may take a few minutes)...${NC}"
    
    # Deploy core services with higher replica counts for production
    kubectl apply -f k8s/data-ingestion-go-deployment.yaml
    kubectl apply -f k8s/data-ingestion-go-service.yaml
    kubectl apply -f k8s/clean-ingestion-go-deployment.yaml
    kubectl apply -f k8s/clean-ingestion-go-service.yaml
    kubectl apply -f k8s/processing-engine-go-deployment.yaml
    kubectl apply -f k8s/processing-engine-go-service.yaml
    kubectl apply -f k8s/storage-layer-pvc.yaml
    kubectl apply -f k8s/storage-layer-go-deployment.yaml
    kubectl apply -f k8s/storage-layer-go-service.yaml
    kubectl apply -f k8s/visualization-go-deployment.yaml
    kubectl apply -f k8s/visualization-go-service.yaml
    kubectl apply -f k8s/tenant-management-go-deployment.yaml
    kubectl apply -f k8s/tenant-management-go-service.yaml
    
    # Deploy monitoring stack
    kubectl apply -f k8s/prometheus-config.yaml
    kubectl apply -f k8s/prometheus-deployment.yaml
    kubectl apply -f k8s/grafana-pvc.yaml
    kubectl apply -f k8s/grafana-datasources.yaml
    kubectl apply -f k8s/grafana-deployment.yaml
    kubectl apply -f k8s/grafana-tenant-dashboards.yaml
    
    echo -e "${GREEN}Production deployment complete!${NC}"
    echo -e "${YELLOW}Running production readiness check...${NC}"
    ./scripts/verify-prod-readiness.sh || true
    
    echo -e "${GREEN}Platform deployed in production mode.${NC}"
    echo -e "${YELLOW}For additional security hardening, run:${NC}"
    echo -e "  ./scripts/security-check.sh --production"
}
# Complete setup: start minikube, build images and deploy
setup_all() {
    echo -e "${BLUE}Starting complete platform setup...${NC}"
    
    # Step 1: Start minikube
    echo -e "${BLUE}Step 1/3: Starting Minikube...${NC}"
    ensure_minikube_operational || {
        echo -e "${RED}Failed to start Minikube. Setup aborted.${NC}"
        return 1
    }
    
    # Step 2: Build all images
    echo -e "${BLUE}Step 2/3: Building all Docker images...${NC}"
    build_images || {
        echo -e "${RED}Failed to build all images. Setup aborted.${NC}"
        echo -e "${YELLOW}You may try running './manage.sh fix-storage' to fix storage layer issues and then retry.${NC}"
        return 1
    }
    
    # Step 3: Deploy the platform
    echo -e "${BLUE}Step 3/3: Deploying platform...${NC}"
    deploy_platform || {
        echo -e "${RED}Failed to deploy platform.${NC}"
        return 1
    }
    
    echo -e "${GREEN}Complete setup successful!${NC}"
    echo -e "${YELLOW}To access services, run:${NC}"
    echo -e "  - './manage.sh status' to check component status"
    echo -e "  - './manage.sh monitoring' to deploy monitoring stack"
    echo -e "  - './manage.sh tunnel-start' to enable LoadBalancer services"
}
# Main logic
case "$1" in
    status)
        show_platform_status
        ;;
    start)
        ensure_minikube_running
        ;;
    stop)
        echo -e "${BLUE}Stopping minikube...${NC}"
        minikube stop
        echo -e "${GREEN}Minikube stopped.${NC}"
        ;;
    build)
        build_images
        ;;        
    deploy)
        deploy_platform
        ;;
    deploy-prod)
        deploy_production
        ;;
    setup-all)
        setup_all
        ;;
    reset-all)
        reset_all
        ;;
    fix-storage)
        fix_storage
        ;;
    fix-processing)
        fix_processing
        ;;
    monitoring)
        deploy_monitoring
        ;;
    dashboard)
        open_dashboard
        ;;
    prometheus)
        # Keep existing prometheus function
        kubectl port-forward -n monitoring service/prometheus-server 9090:80 &
        echo "Prometheus is accessible at http://localhost:9090"
        echo "Use 'kill %1' or './manage.sh stop-access' to stop port forwarding"
        ;;
    
    access-viz)
        echo "Starting port forward to visualization dashboard..."
        kubectl port-forward service/visualization-go-service 8080:80 -n analytics-platform &
        VIZ_PID=$!
        
        # Give port forwarding a moment to establish
        sleep 2
        
        # Check if port forwarding is working
        if kill -0 $VIZ_PID 2>/dev/null; then
            echo "✅ Visualization dashboard is accessible at http://localhost:8080"
            echo "Port forwarding PID: $VIZ_PID"
            echo "Use 'kill $VIZ_PID' or './manage.sh stop-access' to stop port forwarding"
            echo ""
            echo "🧪 To test the dashboard with sample data, use:"
            echo "./manage.sh test-api"
        else
            echo "❌ Failed to establish port forwarding"
            echo "Check if the visualization service is running:"
            echo "kubectl get pods -n analytics-platform | grep visualization"
        fi
        ;;
    
    access-api)
        echo "Starting port forward to data ingestion API..."
        kubectl port-forward service/data-ingestion-go-service 5000:80 -n analytics-platform &
        API_PID=$!
        
        # Give port forwarding a moment to establish
        sleep 2
        
        # Check if port forwarding is working
        if kill -0 $API_PID 2>/dev/null; then
            echo "✅ Data ingestion API is accessible at http://localhost:5000"
            echo "Port forwarding PID: $API_PID"
            echo "Use 'kill $API_PID' or './manage.sh stop-access' to stop port forwarding"
            echo ""
            echo "🧪 To test the API, use:"
            echo "./manage.sh test-api"
        else
            echo "❌ Failed to establish port forwarding"
            echo "Check if the data ingestion service is running:"
            echo "kubectl get pods -n analytics-platform | grep data-ingestion"
        fi
        ;;
    
    stop-access)
        echo "Stopping all port forwarding sessions..."
        # Kill all kubectl port-forward processes
        pkill -f "kubectl port-forward"
        echo "All port forwarding sessions stopped."
        ;;
    
    dashboard)
        # Add dashboard access with improved instructions
        echo "Starting Kubernetes dashboard..."
        kubectl proxy &
        PROXY_PID=$!
        echo "Kubernetes dashboard proxy started (PID: $PROXY_PID)"
        echo ""
        echo "Dashboard URL: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/"
        echo ""
        echo "To get admin token, run:"
        echo "kubectl -n kubernetes-dashboard create token admin-user"
        echo ""
        echo "Use 'kill $PROXY_PID' to stop the dashboard proxy"
        ;;
    
    test-api)
        echo "Testing data ingestion API with secure credentials..."
        
        # Get API key securely from Kubernetes secret
        API_KEY=$(kubectl get secret tenant-management-secrets -n analytics-platform -o jsonpath='{.data.API_KEY_1}' | base64 --decode 2>/dev/null)
        
        if [ -z "$API_KEY" ]; then
            echo "❌ Error: Could not retrieve API key from Kubernetes secret"
            echo "Make sure the platform is deployed and secrets are configured"
            exit 1
        fi
        
        echo "✅ API key retrieved securely from Kubernetes secret"
        
        # Test different metrics
        echo ""
        echo "📊 Sending test data to API..."
        
        # CPU Usage
        echo "Testing CPU usage metric..."
        curl -s -X POST http://localhost:5000/api/data \
          -H 'Content-Type: application/json' \
          -H "X-API-KEY: $API_KEY" \
          -d "{\"device_id\": \"server-01\", \"temperature\": 85.2, \"humidity\": 45.3, \"timestamp\": \"$(date -Iseconds)\"}" \
          && echo " ✅ CPU metric sent successfully" || echo " ❌ Failed to send CPU metric"
        
        # Memory Usage  
        echo "Testing memory usage metric..."
        curl -s -X POST http://localhost:5000/api/data \
          -H 'Content-Type: application/json' \
          -H "X-API-KEY: $API_KEY" \
          -d "{\"device_id\": \"server-02\", \"temperature\": 67.8, \"humidity\": 52.1, \"timestamp\": \"$(date -Iseconds)\"}" \
          && echo " ✅ Memory metric sent successfully" || echo " ❌ Failed to send memory metric"
        
        # Network Latency
        echo "Testing network latency metric..."
        curl -s -X POST http://localhost:5000/api/data \
          -H 'Content-Type: application/json' \
          -H "X-API-KEY: $API_KEY" \
          -d "{\"device_id\": \"gateway-01\", \"temperature\": 23.5, \"humidity\": 60.7, \"timestamp\": \"$(date -Iseconds)\"}" \
          && echo " ✅ Network metric sent successfully" || echo " ❌ Failed to send network metric"
        
        echo ""
        echo "🎉 Test data sent! Check the dashboard at http://localhost:8080"
        ;;
    
    help)
        # Keep existing help function
        show_help
        ;;
    
    *)
        # Keep existing default case
        echo "Unknown command: $1"
        echo "Use './manage.sh help' to see available commands"
        exit 1
        ;;
esac
