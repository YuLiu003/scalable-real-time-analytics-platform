#!/bin/bash
set -e

echo "🔄 Updating Visualization Service with Real-Time Charts..."

# Update requirements
cat > visualization/requirements.txt << 'REQUIREMENTS'
flask==2.0.1
werkzeug==2.0.2
requests==2.26.0
flask-socketio==5.1.1
python-socketio==5.4.0
python-engineio==4.2.1
REQUIREMENTS

echo "✅ Updated requirements.txt"

# Build the image
echo "🔨 Building visualization image..."
eval $(minikube docker-env)
docker build -t real-time-analytics-platform-visualization:latest ./visualization

echo "�� Restarting visualization deployment..."
kubectl rollout restart deployment/visualization -n analytics-platform

echo "⏳ Waiting for deployment to complete..."
sleep 5
kubectl get pods -n analytics-platform | grep visualization

echo "✨ Visualization service updated with real-time charts!"
echo ""
echo "Access the dashboard using the open-dashboard.sh script"
echo "Then send some test data to see the real-time charts in action."
