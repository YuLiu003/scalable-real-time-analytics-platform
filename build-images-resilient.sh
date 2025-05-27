#!/bin/bash

# Resilient Docker build script with fallbacks and error handling

set -e

SERVICES=(
    "data-ingestion-go"
    "clean-ingestion-go" 
    "processing-engine-go"
    "storage-layer-go"
    "visualization-go"
    "tenant-management-go"
)

echo "🔨 Building Docker images with resilient fallbacks..."

# Function to build with retries and fallbacks
build_with_fallback() {
    local service=$1
    local attempt=1
    local max_attempts=3
    
    echo "Building $service (attempt $attempt/$max_attempts)..."
    
    cd "$service"
    
    # Try normal build first
    if docker build -t "$service:latest" . 2>/dev/null; then
        echo "✅ Successfully built $service"
        cd ..
        return 0
    fi
    
    echo "⚠️ Normal build failed, trying with BuildKit disabled..."
    export DOCKER_BUILDKIT=0
    
    if docker build -t "$service:latest" . 2>/dev/null; then
        echo "✅ Successfully built $service (BuildKit disabled)"
        cd ..
        return 0
    fi
    
    echo "⚠️ Build failed, trying with no-cache..."
    if docker build --no-cache -t "$service:latest" . 2>/dev/null; then
        echo "✅ Successfully built $service (no-cache)"
        cd ..
        return 0
    fi
    
    echo "❌ Failed to build $service after all attempts"
    cd ..
    return 1
}

# Build each service
successful_builds=0
failed_builds=0

for service in "${SERVICES[@]}"; do
    if [ -d "$service" ]; then
        if build_with_fallback "$service"; then
            ((successful_builds++))
        else
            ((failed_builds++))
            echo "⚠️ Continuing with next service..."
        fi
    else
        echo "⚠️ Directory $service not found, skipping..."
    fi
done

echo ""
echo "📊 Build Summary:"
echo "✅ Successful: $successful_builds"
echo "❌ Failed: $failed_builds"

if [ $failed_builds -eq 0 ]; then
    echo "🎉 All images built successfully!"
    exit 0
else
    echo "⚠️ Some images failed to build. Check logs above."
    exit 1
fi
