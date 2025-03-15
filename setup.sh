#!/bin/bash
set -e

echo "🚀 Setting up Real-Time Analytics Platform..."

# Create directories if they don't exist
mkdir -p k8s
mkdir -p data-ingestion/src
mkdir -p processing-engine/src
mkdir -p storage-layer/src
mkdir -p visualization/src

# Install required packages locally
echo "📦 Installing Python dependencies..."
pip install flask kafka-python numpy requests flask-socketio eventlet

# Create Kubernetes namespace
echo "🔧 Creating Kubernetes resources..."

# Check if minikube is running
if ! minikube status >/dev/null 2>&1; then
    echo "Starting minikube..."
    minikube start
fi

# Apply namespace
kubectl apply -f k8s/namespace.yaml

echo "✅ Setup complete!"
echo "Next steps:"
echo "1. Run './fix-dependencies.sh' to build Docker images"
echo "2. Run './deploy.sh' to deploy the platform"
echo "3. Run './open-dashboard.sh' to access the dashboard"