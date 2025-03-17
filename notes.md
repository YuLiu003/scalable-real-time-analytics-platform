# Essential Kubernetes Commands for Managing Our Project

You're absolutely right - having a quick set of commands to check the status of our Kubernetes resources is essential for daily operations. Just like how `docker ps` is indispensable for Docker work, we need equivalent commands for our Kubernetes cluster.

## Essential Commands When Opening the Project

When you first open the project, run these commands to get a quick overview:

```bash
# Check if minikube is running
minikube status

# If not running, start it
minikube start

# Check all pods in our namespace
kubectl get pods -n analytics-platform

# Check all services in our namespace
kubectl get services -n analytics-platform

# Check deployments
kubectl get deployments -n analytics-platform

# Quick view of resource usage
kubectl top pods -n analytics-platform
```

Let's create a simple script to encapsulate these checks:

```bash
#!/bin/bash

echo "🔍 Checking Kubernetes Status for Real-Time Analytics Platform"
echo "=========================================================="

# Check minikube status
echo -e "\n📊 Minikube Status:"
minikube status
if [ $? -ne 0 ]; then
    echo "🚨 Minikube is not running. Start with 'minikube start'"
    exit 1
fi

# Check namespace exists
echo -e "\n📊 Checking namespace:"
kubectl get namespace analytics-platform > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "🚨 Namespace 'analytics-platform' does not exist. Deploy the platform first."
    exit 1
fi

# Check pods
echo -e "\n📊 Pods Status:"
kubectl get pods -n analytics-platform

# Check services
echo -e "\n📊 Services Status:"
kubectl get services -n analytics-platform

# Check deployments
echo -e "\n📊 Deployments Status:"
kubectl get deployments -n analytics-platform

# Check resource usage if metrics server is available
echo -e "\n📊 Resource Usage (if available):"
kubectl top pods -n analytics-platform 2>/dev/null || echo "Metrics server not available. Run 'minikube addons enable metrics-server' to enable."

echo -e "\n✅ Status check complete."
echo "Use 'kubectl logs -f <pod-name> -n analytics-platform' to view logs for a specific pod."
echo "Use 'kubectl describe pod <pod-name> -n analytics-platform' for detailed pod information."
```

Make the script executable:

```bash
chmod +x scripts/kube-status.sh
```

## Commands for Managing the Project

Here's a set of common commands you'll use when working with this project:

### For checking status:

```bash
# Full cluster status check (using our script)
./scripts/kube-status.sh

# Check logs of a specific pod
kubectl logs -f <pod-name> -n analytics-platform

# Check details of a specific pod
kubectl describe pod <pod-name> -n analytics-platform

# Check persistent volumes
kubectl get pv -n analytics-platform

# Check persistent volume claims
kubectl get pvc -n analytics-platform
```

### For managing the platform:

```bash
# Deploy/update the platform
./deploy-platform.sh

# Deploy/update the monitoring stack
./scripts/deploy-monitoring.sh

# Access Prometheus UI
kubectl port-forward svc/prometheus -n analytics-platform 9090:9090

# Access Grafana UI
kubectl port-forward svc/grafana -n analytics-platform 3000:3000

# Execute a command inside a pod
kubectl exec -it <pod-name> -n analytics-platform -- /bin/bash

# Restart a deployment
kubectl rollout restart deployment/<deployment-name> -n analytics-platform
```

### For troubleshooting:

```bash
# Check events (useful for debugging issues)
kubectl get events -n analytics-platform --sort-by=.metadata.creationTimestamp

# Check ConfigMaps
kubectl get configmaps -n analytics-platform

# Check Secrets
kubectl get secrets -n analytics-platform

# Describe a service to check endpoints
kubectl describe service <service-name> -n analytics-platform

# Check network policies
kubectl get networkpolicy -n analytics-platform
```

## When Not Working on the Project

When you're not actively working on the project, you can:

```bash
# Stop the minikube cluster to free up resources
minikube stop

# To completely delete the cluster (use with caution)
minikube delete
```

## Let's Create a Project Management Script

Let's create a comprehensive management script that provides a simple interface for common operations:

```bash
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
    echo "  logs <pod>  - View logs for a specific pod"
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
    ./scripts/deploy-monitoring.sh
    echo -e "${GREEN}Monitoring deployment complete!${NC}"
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
    echo -e "${BLUE}Setting up port forwarding to Prometheus...${NC}"
    echo -e "${YELLOW}Access Prometheus at: http://localhost:9090${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl port-forward svc/prometheus -n analytics-platform 9090:9090
}

# Port forward to Grafana
access_grafana() {
    ensure_minikube_running
    echo -e "${BLUE}Setting up port forwarding to Grafana...${NC}"
    echo -e "${YELLOW}Access Grafana at: http://localhost:3000${NC}"
    echo -e "${YELLOW}Default credentials: admin / admin-secure-password${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl port-forward svc/grafana -n analytics-platform 3000:3000
}

# Run platform tests
run_tests() {
    ensure_minikube_running
    echo -e "${BLUE}Setting up port forwarding for tests...${NC}"
    ./scripts/setup-test-env.sh &
    PF_PID=$!
    
    # Give port forwarding time to establish
    sleep 5
    
    echo -e "${BLUE}Running platform tests...${NC}"
    ./scripts/test-platform.py
    
    # Cleanup
    kill $PF_PID 2>/dev/null
}

# View logs for a pod
view_logs() {
    if [ -z "$1" ]; then
        echo -e "${RED}Error: Pod name is required${NC}"
        echo "Usage: ./manage.sh logs <pod-name>"
        
        echo -e "\n${BLUE}Available pods:${NC}"
        kubectl get pods -n analytics-platform
        return 1
    fi
    
    ensure_minikube_running
    echo -e "${BLUE}Streaming logs for pod $1...${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    kubectl logs -f $1 -n analytics-platform
}

# Clean up all resources
cleanup_resources() {
    ensure_minikube_running
    
    echo -e "${RED}Warning: This will remove all platform resources. Are you sure? (y/n)${NC}"
    read -r confirmation
    if [[ $confirmation =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Removing all platform resources...${NC}"
        kubectl delete namespace analytics-platform
        echo -e "${GREEN}Cleanup complete!${NC}"
    else
        echo -e "${BLUE}Cleanup cancelled.${NC}"
    fi
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
    logs)
        view_logs $2
        ;;
    cleanup)
        cleanup_resources
        ;;
    help|*)
        show_help
        ;;
esac
```

Make this script executable:

```bash
chmod +x manage.sh
```

Now you can manage your entire platform with simple commands:

```bash
# Show status
./manage.sh status

# Deploy the platform
./manage.sh deploy

# Deploy monitoring
./manage.sh monitoring

# Access Grafana
./manage.sh grafana

# View logs for a specific pod
./manage.sh logs flask-api-5678-abc
```

This script provides a convenient interface to manage all aspects of your platform, similar to how Docker CLI works for container management.