#!/bin/bash

# Script to switch from Zookeeper-based Kafka to KRaft mode
set -e

echo "Switching from Zookeeper-based Kafka to KRaft mode"

# Check if running in Kubernetes or Docker Compose mode
if command -v kubectl &> /dev/null; then
    echo "Detected Kubernetes environment"
    
    # Check if we have the namespace
    if kubectl get namespace analytics-platform &> /dev/null; then
        # Stop the old Kafka and Zookeeper
        echo "Stopping old Kafka and Zookeeper deployments..."
        kubectl delete statefulset kafka -n analytics-platform --ignore-not-found=true
        kubectl delete statefulset zookeeper -n analytics-platform --ignore-not-found=true
        kubectl delete service kafka -n analytics-platform --ignore-not-found=true
        kubectl delete service zookeeper -n analytics-platform --ignore-not-found=true
        kubectl delete service zookeeper-headless -n analytics-platform --ignore-not-found=true
        
        # Apply the new Kafka KRaft configuration
        echo "Applying new Kafka KRaft configuration..."
        kubectl apply -f k8s/kafka-kraft-statefulset.yaml -n analytics-platform
        
        echo "Waiting for Kafka StatefulSet to start..."
        kubectl rollout status statefulset/kafka -n analytics-platform --timeout=300s
        
        echo "Kafka KRaft mode enabled successfully in Kubernetes!"
    else
        echo "Error: analytics-platform namespace not found in Kubernetes"
        exit 1
    fi
else
    echo "Detected Docker Compose environment"
    
    # Check if docker-compose is installed
    if ! command -v docker-compose &> /dev/null; then
        echo "Error: docker-compose is not installed"
        exit 1
    fi
    
    # Stop the current services
    echo "Stopping current Docker Compose services..."
    docker-compose down
    
    # Cleanup volumes (if requested)
    read -p "Do you want to clean up Kafka and Zookeeper volumes for a fresh start? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing Kafka and Zookeeper volumes..."
        docker volume rm $(docker volume ls -q | grep -E 'kafka|zookeeper') || true
    fi
    
    # Start with the new KRaft configuration
    echo "Starting services with KRaft mode..."
    docker-compose -f docker-compose-kraft.yml up -d
    
    echo "Kafka KRaft mode enabled successfully with Docker Compose!"
fi

echo "Done! Kafka is now running in KRaft mode without Zookeeper dependency."
echo "If you have any issues, please check the logs."
echo ""
echo "Kafka Logs (Kubernetes): kubectl logs -f -l app=kafka -n analytics-platform"
echo "Kafka Logs (Docker): docker-compose -f docker-compose-kraft.yml logs -f kafka" 