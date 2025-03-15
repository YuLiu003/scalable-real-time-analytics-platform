#!/bin/bash

# Function to check if service is running
check_service() {
    local name=$1
    local cmd=$2
    echo -n "Setting up access to $name... "
    url=$(eval "$cmd" | grep -o "http://[0-9.]*:[0-9]*" | head -1)
    
    if [ -z "$url" ]; then
        echo "❌ Failed to create service URL"
        return 1
    else
        echo "✅ Available at $url"
        return 0
    fi
}

echo "📊 Setting up access to Real-Time Analytics Platform Services..."
echo "This will set up direct access to services through minikube tunnels."

# Set up direct access to visualization service
VIS_URL=$(minikube service visualization-service -n analytics-platform --url)
DATA_URL=$(minikube service data-ingestion-service -n analytics-platform --url)

echo ""
echo "📊 Service URLs:"
echo "Visualization Dashboard: $VIS_URL"
echo "Data Ingestion API:      $DATA_URL"
echo ""

echo "Opening visualization dashboard..."
open "$VIS_URL"

echo ""
echo "Example API calls:"
echo "# Send test data"
echo "curl -X POST $DATA_URL/api/data \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -d '{\"device_id\": \"test-001\", \"temperature\": 25.5, \"humidity\": 60}'"

echo ""
echo "# Check service health"
echo "curl $DATA_URL/health"
echo "curl $VIS_URL/health"
