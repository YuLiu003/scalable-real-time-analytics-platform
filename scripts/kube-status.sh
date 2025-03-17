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