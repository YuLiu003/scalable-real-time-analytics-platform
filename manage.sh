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
    echo "  monitoring  - Deploy monitoring stack (Prometheus/Grafana)"
    echo "  dashboard   - Open Kubernetes dashboard"
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

# Run minikube tunnel in background
start_minikube_tunnel() {
    ensure_minikube_operational || return 1
    
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
full_minikube_reset() {
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
ensure_namespace_exists() {
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

# Run minikube tunnel in background
start_minikube_tunnel() {
    ensure_minikube_operational || return 1
    
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
full_minikube_reset() {
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

# Add this function to your manage.sh script:
build_images() {
    # First check if minikube is operational
    ensure_minikube_operational || {
        echo -e "${RED}Cannot build images because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    }
    
    echo -e "${BLUE}Building platform images...${NC}"
    
    # Connect to minikube's Docker daemon
    echo -e "${BLUE}Connecting to minikube's Docker daemon...${NC}"
    eval $(minikube docker-env)
    
    # Check if build-images.sh exists and is executable
    if [ ! -f "./build-images.sh" ]; then
        echo -e "${RED}Error: build-images.sh script not found${NC}"
        return 1
    fi
    
    if [ ! -x "./build-images.sh" ]; then
        echo -e "${YELLOW}Making build-images.sh executable...${NC}"
        chmod +x ./build-images.sh
    fi
    
    # Run the build-images.sh script
    echo -e "${BLUE}Running build-images.sh...${NC}"
    ./build-images.sh
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}All images built successfully!${NC}"
        echo -e "${YELLOW}You can now run './manage.sh deploy' to deploy the platform.${NC}"
    else
        echo -e "${RED}Image building failed. Please check the error messages above.${NC}"
        return 1
    fi
}

# Deploy the platform
deploy_platform() {
    # Use the enhanced check that ensures API server is responding
    ensure_minikube_operational || {
        echo -e "${RED}Cannot deploy because Minikube is not fully operational.${NC}"
        echo -e "${YELLOW}Try running './manage.sh fix-minikube' or './manage.sh reset-minikube' to fix issues.${NC}"
        return 1
    }
    
    echo -e "${BLUE}Deploying Real-Time Analytics Platform...${NC}"
    
    # Check if images exist
    eval $(minikube docker-env)
    if ! docker images | grep -q "flask-api"; then
        echo -e "${YELLOW}Required images not found. Building images first...${NC}"
        build_images || {
            echo -e "${RED}Failed to build images. Deployment aborted.${NC}"
            return 1
        }
    else
        echo -e "${GREEN}Using existing Docker images...${NC}"
    fi

    # Instead of just executing deploy-platform.sh, we'll add more control here
    
    # Make sure namespace exists
    ensure_namespace_exists
    
    # Apply core infrastructure 
    echo "Creating necessary secrets and configmaps..."
    kubectl apply -f k8s/configmap.yaml
    kubectl apply -f k8s/secrets.yaml
    kubectl apply -f k8s/grafana-secret.yaml
    
    # Deploy Kafka first
    echo "Deploying Kafka..."
    kubectl apply -f k8s/kafka-secrets.yaml
    kubectl apply -f k8s/kafka-pvc.yaml
    kubectl apply -f k8s/kafka-kraft-deployment.yaml
    kubectl apply -f k8s/kafka-service.yaml
    
    # Wait for Kafka to be ready before continuing
    wait_for_resource deployment kafka analytics-platform 300
    
    # Deploy remaining services
    echo "Deploying platform services..."
    kubectl apply -f k8s/flask-api-deployment.yaml
    kubectl apply -f k8s/flask-api-service.yaml
    kubectl apply -f k8s/data-ingestion-deployment.yaml
    kubectl apply -f k8s/data-ingestion-service.yaml
    
    # Wait for key services before deploying dependent services
    wait_for_resource deployment flask-api analytics-platform
    wait_for_resource deployment data-ingestion analytics-platform
    
    # Deploy processing engine and dependent services
    kubectl apply -f k8s/processing-engine-deployment.yaml
    kubectl apply -f k8s/processing-engine-service.yaml
    kubectl apply -f k8s/storage-layer-deployment.yaml
    kubectl apply -f k8s/storage-layer-service.yaml
    kubectl apply -f k8s/visualization-deployment.yaml
    kubectl apply -f k8s/visualization-service.yaml
    
    echo -e "${GREEN}Deployment complete!${NC}"
    show_status
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

# Tenant management functions

# List tenants
list_tenants() {
    ensure_minikube_running
    ensure_namespace_exists
    
    echo -e "${BLUE}Tenant management is not yet fully integrated into the platform.${NC}"
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
    
    echo -e "\n${YELLOW}To enable full tenant management, deploy the tenant-management service:${NC}"
    echo -e "  ./manage.sh deploy-tenant-mgmt"
}

# Enable tenant isolation
enable_tenant_isolation() {
    ensure_minikube_running
    ensure_namespace_exists
    
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
    kubectl rollout restart deployment/flask-api -n analytics-platform
    kubectl rollout restart deployment/data-ingestion -n analytics-platform
    
    echo -e "${GREEN}Tenant isolation enabled!${NC}"
    echo -e "${YELLOW}Use ./manage.sh list-tenants to see current tenant mappings${NC}"
}

# Deploy tenant management service
deploy_tenant_mgmt() {
    ensure_minikube_running
    ensure_namespace_exists
    
    echo -e "${BLUE}Deploying tenant management service...${NC}"
    
    # Check if tenant-management directory exists
    if [ ! -d "platform/tenant-management" ]; then
        echo -e "${RED}Error: tenant-management directory not found${NC}"
        echo -e "${YELLOW}Creating basic directory structure...${NC}"
        
        mkdir -p platform/tenant-management/src
        
        # Create tenant model
        cat > platform/tenant-management/src/tenant_model.py << 'EOF'
from datetime import datetime

class Tenant:
    """Represents a platform tenant with isolation settings."""
    
    def __init__(self, tenant_id, name, plan="basic"):
        self.tenant_id = tenant_id
        self.name = name
        self.plan = plan  # basic, standard, premium
        self.resource_limits = self._get_plan_limits(plan)
        self.created_at = datetime.now()
        
    def _get_plan_limits(self, plan):
        """Get resource limits based on plan tier."""
        limits = {
            "basic": {
                "max_data_points": 100000,
                "max_queries_per_min": 60,
                "retention_days": 30,
                "max_dashboards": 5,
                "cpu_limit": "2",
                "memory_limit": "2Gi"
            },
            "standard": {
                "max_data_points": 1000000,
                "max_queries_per_min": 300,
                "retention_days": 90,
                "max_dashboards": 20,
                "cpu_limit": "4",
                "memory_limit": "8Gi"
            },
            "premium": {
                "max_data_points": 10000000,
                "max_queries_per_min": 1000,
                "retention_days": 365,
                "max_dashboards": 50,
                "cpu_limit": "8",
                "memory_limit": "16Gi"
            }
        }
        return limits.get(plan, limits["basic"])
EOF

        # Create tenant service
        cat > platform/tenant-management/src/tenant_service.py << 'EOF'
from flask import Flask, request, jsonify
import os
import json
import sys
from datetime import datetime

# Add the current directory to the path
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from src.tenant_model import Tenant

app = Flask(__name__)

# In-memory tenant store for testing
tenants = {}

@app.route('/api/tenants', methods=['GET'])
def list_tenants():
    """List all tenants."""
    return jsonify({"tenants": [
        {
            "tenant_id": t.tenant_id,
            "name": t.name,
            "plan": t.plan,
            "created_at": t.created_at.isoformat()
        } for t in tenants.values()
    ]})

@app.route('/api/tenants/<tenant_id>', methods=['GET'])
def get_tenant(tenant_id):
    """Get tenant by ID."""
    tenant = tenants.get(tenant_id)
    if not tenant:
        return jsonify({"error": "Tenant not found"}), 404
    
    return jsonify({
        "tenant_id": tenant.tenant_id,
        "name": tenant.name,
        "plan": tenant.plan,
        "resource_limits": tenant.resource_limits,
        "created_at": tenant.created_at.isoformat()
    })

@app.route('/api/tenants', methods=['POST'])
def create_tenant():
    """Create a new tenant."""
    data = request.json
    tenant_id = data.get('tenant_id')
    name = data.get('name')
    plan = data.get('plan', 'basic')
    
    # Validate input
    if not tenant_id or not name:
        return jsonify({"error": "Missing required fields"}), 400
        
    # Check if tenant already exists
    if tenant_id in tenants:
        return jsonify({"error": "Tenant ID already exists"}), 409
    
    # Create tenant
    tenant = Tenant(tenant_id, name, plan)
    tenants[tenant_id] = tenant
    
    return jsonify({
        "tenant_id": tenant.tenant_id,
        "name": tenant.name,
        "plan": tenant.plan,
        "created_at": tenant.created_at.isoformat(),
        "status": "created"
    }), 201

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({"status": "healthy"}), 200

if __name__ == '__main__':
    # Create default tenants
    tenants['tenant1'] = Tenant('tenant1', 'Demo Corp', 'premium')
    tenants['tenant2'] = Tenant('tenant2', 'Test Inc', 'basic')
    
    # Print tenant info for debugging
    print("Created default tenants:")
    for tenant_id, tenant in tenants.items():
        print(f"- {tenant_id}: {tenant.name} ({tenant.plan})")
    
    app.run(host='0.0.0.0', port=5010, debug=True)
EOF

        # Create requirements.txt
        cat > platform/tenant-management/requirements.txt << 'EOF'
flask==2.0.1
pyyaml==6.0
kubernetes==23.6.0
pytest==7.0.1
requests==2.27.1
EOF

        # Create Dockerfile
        cat > platform/tenant-management/Dockerfile << 'EOF'
FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Create non-root user
RUN addgroup --system --gid 1001 appuser \
    && adduser --system --uid 1001 --gid 1001 --no-create-home appuser

# Set permissions
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 5010

CMD ["python", "src/tenant_service.py"]
EOF
    fi
    
    # Build and deploy the tenant management service
    echo -e "${BLUE}Building tenant management service image...${NC}"
    eval $(minikube docker-env)
    docker build -t tenant-management:latest -f platform/tenant-management/Dockerfile ./platform/tenant-management
    
    # Create Kubernetes deployment and service
    cat > k8s/tenant-management-deployment.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tenant-management
  namespace: analytics-platform
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tenant-management
  template:
    metadata:
      labels:
        app: tenant-management
    spec:
      containers:
      - name: tenant-management
        image: tenant-management:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 5010
        securityContext:
          runAsUser: 1001
          runAsNonRoot: true
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
        readinessProbe:
          httpGet:
            path: /health
            port: 5010
          initialDelaySeconds: 10
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /health
            port: 5010
          initialDelaySeconds: 15
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: tenant-management-service
  namespace: analytics-platform
spec:
  selector:
    app: tenant-management
  ports:
  - port: 80
    targetPort: 5010
    nodePort: 30090
  type: NodePort
EOF

    # Apply Kubernetes resources
    kubectl apply -f k8s/tenant-management-deployment.yaml
    
    echo -e "${GREEN}Tenant management service deployed!${NC}"
    echo -e "${YELLOW}Access the service at: http://$(minikube ip):30090/api/tenants${NC}"
}

# Create a new tenant
create_tenant() {
    ensure_minikube_running
    ensure_namespace_exists
    
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
  ensure_minikube_running
  
  echo -e "${RED}Warning: This will remove all platform resources. Are you sure? (y/n)${NC}"
  read -r confirmation
  if [[ $confirmation =~ ^[Yy]$ ]]; then
    echo -e "${BLUE}Removing all platform resources...${NC}"
    
    # Remove specific resources first to avoid dependency issues
    echo "Removing deployments first..."
    kubectl delete deployment --all -n analytics-platform --ignore-not-found=true
    
    echo "Removing services..."
    kubectl delete service --all -n analytics-platform --ignore-not-found=true
    
    echo "Removing configmaps and secrets..."
    kubectl delete configmap --all -n analytics-platform --ignore-not-found=true
    kubectl delete secret --all -n analytics-platform --ignore-not-found=true
    
    echo "Removing persistent volumes and claims..."
    kubectl delete pvc --all -n analytics-platform --ignore-not-found=true
    kubectl delete pv --all --ignore-not-found=true
    
    echo "Finally removing namespace..."
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
    build)
        build_images
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
    fix-minikube)
        fix_minikube
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
    list-tenants)
        list_tenants
        ;;
    enable-tenant-isolation)
        enable_tenant_isolation
        ;;
    deploy-tenant-mgmt)
        deploy_tenant_mgmt
        ;;
    create-tenant)
        create_tenant "$2" "$3" "$4"
        ;;
    tunnel-start)
        start_minikube_tunnel
        ;;
    tunnel-stop)
        stop_minikube_tunnel
        ;;
    reset-minikube)
        full_minikube_reset
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