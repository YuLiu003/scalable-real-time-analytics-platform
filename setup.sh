#!/bin/bash
# This script sets up the development environment for the analytics platform

set -e

echo "🔧 Setting up Real-Time Analytics Platform development environment..."

# Check if minikube is installed
if ! command -v minikube &> /dev/null; then
    echo "❌ minikube is not installed. Please install it first."
    echo "   Visit: https://minikube.sigs.k8s.io/docs/start/"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed. Please install it first."
    echo "   Visit: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ docker is not installed. Please install it first."
    echo "   Visit: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check if docker-compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose is not installed. Please install it first."
    echo "   Visit: https://docs.docker.com/compose/install/"
    exit 1
fi

# Make scripts executable
echo "Making scripts executable..."
chmod +x *.sh
chmod +x scripts/*.sh
chmod +x scripts/*.py 2>/dev/null || true

# Start minikube if not running
echo "Starting minikube..."
minikube status &> /dev/null || minikube start

# Enable minikube addons
echo "Enabling necessary minikube addons..."
minikube addons enable metrics-server
minikube addons enable dashboard

echo "✅ Setup complete!"
echo ""
echo "Next steps:"
echo "1. Deploy the platform: ./deploy-platform.sh"
echo "2. Check the status: ./manage.sh status"
echo ""
echo "Use ./manage.sh help for more information on available commands"