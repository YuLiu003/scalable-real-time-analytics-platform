#!/bin/bash

set -e

# Create namespace if it doesn't exist
kubectl get namespace analytics-platform > /dev/null 2>&1 || kubectl create namespace analytics-platform

echo "🔄 Deploying Prometheus..."
kubectl apply -f k8s/prometheus-config.yaml
kubectl apply -f k8s/prometheus-deployment.yaml

echo "🔄 Deploying Grafana..."
kubectl apply -f k8s/grafana-secret.yaml
kubectl apply -f k8s/grafana-deployment.yaml

echo "🔍 Checking deployment status..."
kubectl rollout status deployment/prometheus -n analytics-platform
kubectl rollout status deployment/grafana -n analytics-platform

echo "✅ Monitoring stack deployed successfully!"
echo "📊 To access Prometheus: kubectl port-forward service/prometheus-service 9090:9090 -n analytics-platform"
echo "📈 To access Grafana: kubectl port-forward service/grafana-service 3000:3000 -n analytics-platform"
echo "   Grafana default credentials:"
echo "   Username: admin"
echo "   Password: check the grafana-secret.yaml file"

# Instructions for setting up Grafana
echo ""
echo "🛠 Next steps for Grafana setup:"
echo "1. Access Grafana at http://localhost:3000 after running the port-forward command"
echo "2. Log in with the admin credentials"
echo "3. Add Prometheus as a data source:"
echo "   - Click 'Data sources' in the configuration menu"
echo "   - Click 'Add data source' and select 'Prometheus'"
echo "   - Set the URL to 'http://prometheus-service:9090'"
echo "   - Click 'Save & Test'"
echo "4. Import dashboards"