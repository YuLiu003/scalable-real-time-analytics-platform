#!/bin/bash
# Build images with correct names for Kubernetes
set -e  # Exit immediately if a command fails

echo "🔄 Connecting to minikube's Docker daemon..."
eval $(minikube docker-env)

# Function to build and verify an image
build_image() {
  local service=$1
  echo "🔨 Building $service image..."
  docker build -t "$service:latest" -f "$service/Dockerfile" "./$service"
  
  # Verify the image was created
  if docker images "$service:latest" --format "{{.Repository}}:{{.Tag}}" | grep -q "$service:latest"; then
    echo "✅ Successfully built $service:latest"
  else
    echo "❌ Failed to build $service:latest"
    exit 1
  fi
}

# Build all service images
build_image "flask-api"
build_image "data-ingestion"
build_image "processing-engine"
build_image "storage-layer"
build_image "visualization"

echo ""
echo "🎉 All images built successfully!"
echo ""
echo "Available images:"
docker images | grep -E '(flask-api|data-ingestion|processing-engine|storage-layer|visualization)'