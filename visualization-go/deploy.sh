#!/bin/bash

# Script to deploy the visualization-go service to Kubernetes

echo "Building Docker image for visualization-go..."
docker build -t visualization-go:latest .

echo "Deploying to Kubernetes..."
kubectl apply -f ../k8s/visualization-go-deployment.yaml
kubectl apply -f ../k8s/visualization-go-service.yaml

echo "Deployment complete!"
echo "Service should be available shortly."

echo "You can check the status with:"
echo "kubectl get pods -n analytics-platform -l app=visualization-go"
echo "kubectl get services -n analytics-platform | grep visualization-go"
