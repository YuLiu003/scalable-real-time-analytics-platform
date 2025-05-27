#!/bin/bash

# Docker Network Troubleshooting and Fix Script
# Addresses IPv6 connectivity issues with Docker Hub

set -e

echo "🔧 Docker Network Troubleshooting Script"
echo "========================================"

# Function to check Docker daemon configuration
check_docker_daemon() {
    echo "Checking Docker daemon configuration..."
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        echo "❌ Docker is not running. Please start Docker first."
        exit 1
    fi
    
    echo "✅ Docker daemon is running"
}

# Function to fix IPv6 connectivity issues
fix_ipv6_issues() {
    echo "Addressing IPv6 connectivity issues..."
    
    # Add IPv4 DNS fallback
    echo "Setting IPv4 DNS preferences..."
    
    # Create or update Docker daemon.json to prefer IPv4
    DOCKER_CONFIG_DIR="$HOME/.docker"
    DAEMON_JSON="$DOCKER_CONFIG_DIR/daemon.json"
    
    mkdir -p "$DOCKER_CONFIG_DIR"
    
    # Backup existing config if it exists
    if [ -f "$DAEMON_JSON" ]; then
        cp "$DAEMON_JSON" "$DAEMON_JSON.backup.$(date +%s)"
        echo "📦 Backed up existing daemon.json"
    fi
    
    # Create daemon.json with IPv4 preference
    cat > "$DAEMON_JSON" << 'EOF'
{
  "dns": ["8.8.8.8", "8.8.4.4", "1.1.1.1"],
  "registry-mirrors": [
    "https://mirror.gcr.io"
  ],
  "insecure-registries": [],
  "experimental": false,
  "features": {
    "buildkit": true
  }
}
EOF
    
    echo "✅ Updated Docker daemon configuration"
    echo "⚠️  You may need to restart Docker for changes to take effect"
}

# Function to test Docker Hub connectivity
test_connectivity() {
    echo "Testing Docker Hub connectivity..."
    
    # Test basic connectivity
    if docker pull hello-world:latest >/dev/null 2>&1; then
        echo "✅ Docker Hub connectivity working"
        docker rmi hello-world:latest >/dev/null 2>&1
        return 0
    else
        echo "❌ Docker Hub connectivity issues detected"
        return 1
    fi
}

# Function to use alternative registries
setup_fallback_registries() {
    echo "Setting up fallback registries..."
    
    # Create a script for pulling with fallbacks
    cat > "docker-pull-with-fallback.sh" << 'EOF'
#!/bin/bash
# Docker pull with registry fallbacks

IMAGE="$1"
if [ -z "$IMAGE" ]; then
    echo "Usage: $0 <image:tag>"
    exit 1
fi

echo "Attempting to pull $IMAGE with fallbacks..."

# Try official registry first
if docker pull "$IMAGE" 2>/dev/null; then
    echo "✅ Successfully pulled $IMAGE from Docker Hub"
    exit 0
fi

# Try with registry mirrors
MIRRORS=(
    "mirror.gcr.io/library"
    "registry.hub.docker.com/library"
)

for MIRROR in "${MIRRORS[@]}"; do
    MIRROR_IMAGE="$MIRROR/${IMAGE#library/}"
    echo "Trying mirror: $MIRROR_IMAGE"
    if docker pull "$MIRROR_IMAGE" 2>/dev/null; then
        echo "✅ Successfully pulled from mirror: $MIRROR"
        # Tag with original name
        docker tag "$MIRROR_IMAGE" "$IMAGE"
        exit 0
    fi
done

echo "❌ Failed to pull $IMAGE from all registries"
exit 1
EOF
    
    chmod +x "docker-pull-with-fallback.sh"
    echo "✅ Created docker-pull-with-fallback.sh script"
}

# Function to create a local build script with error handling
create_resilient_build_script() {
    echo "Creating resilient build script..."
    
    cat > "build-images-resilient.sh" << 'EOF'
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
EOF
    
    chmod +x "build-images-resilient.sh"
    echo "✅ Created build-images-resilient.sh script"
}

# Main execution
main() {
    echo "Starting Docker network troubleshooting..."
    
    check_docker_daemon
    
    if ! test_connectivity; then
        echo "Network issues detected, applying fixes..."
        fix_ipv6_issues
        setup_fallback_registries
    fi
    
    create_resilient_build_script
    
    echo ""
    echo "🎉 Docker network troubleshooting complete!"
    echo ""
    echo "Next steps:"
    echo "1. Restart Docker if daemon.json was updated"
    echo "2. Run './build-images-resilient.sh' to build with fallbacks"
    echo "3. Use './docker-pull-with-fallback.sh <image>' for problematic pulls"
    echo ""
    echo "If issues persist, consider:"
    echo "- Switching to a different network"
    echo "- Using a VPN"
    echo "- Setting up a local Docker registry mirror"
}

main "$@"
