#!/bin/bash
set -e

# Print header
echo "📊 Real-Time Analytics Platform Dashboard"
echo "⏳ Setting up access to services..."

# Function to get service URL with timeout
get_service_url_with_timeout() {
    local service=$1
    local namespace=$2
    local timeout=5
    
    # Run minikube service command with a timeout
    timeout $timeout minikube service $service -n $namespace --url 2>/dev/null || echo ""
}

# Use direct port-forwarding method
use_port_forwarding() {
    echo "Using port-forwarding method..."
    
    # Kill any existing port forwards
    pkill -f "kubectl port-forward" || true
    
    # Set up port forwarding for visualization
    echo "Setting up port forwarding for visualization service to localhost:8081..."
    kubectl port-forward service/visualization-service 8081:80 -n analytics-platform &
    VIS_PF_PID=$!
    
    # Set up port forwarding for data ingestion
    echo "Setting up port forwarding for data ingestion service to localhost:8080..."
    kubectl port-forward service/data-ingestion-service 8080:80 -n analytics-platform &
    DATA_PF_PID=$!
    
    # Wait for port forwarding to be established
    sleep 2
    
    VIS_URL="http://localhost:8081"
    DATA_URL="http://localhost:8080"
    
    echo "✅ Port forwarding set up"
    
    # Register cleanup function to kill port forwarding on exit
    trap "echo 'Cleaning up port forwarding...'; kill $VIS_PF_PID $DATA_PF_PID 2>/dev/null || true" EXIT
}

# Try to get URLs with timeout
echo "Trying minikube service approach (with 5s timeout)..."
VIS_URL=$(get_service_url_with_timeout visualization-service analytics-platform)
DATA_URL=$(get_service_url_with_timeout data-ingestion-service analytics-platform)

# If URLs couldn't be obtained, use port-forwarding
if [ -z "$VIS_URL" ] || [ -z "$DATA_URL" ]; then
    echo "⚠️  Could not access services via minikube. Using port-forwarding instead."
    use_port_forwarding
fi

# Print access information
echo ""
echo "📊 Service Access Information:"
echo "- Visualization Dashboard: $VIS_URL"
echo "- Data Ingestion API:      $DATA_URL"

# Test service health
echo ""
echo "🔍 Testing services..."
echo -n "Checking Visualization Dashboard... "
VIS_HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" $VIS_URL/health 2>/dev/null || echo "000")
echo -n "Checking Data Ingestion API... "
DATA_HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" $DATA_URL/health 2>/dev/null || echo "000")

echo ""
if [ "$VIS_HEALTH_STATUS" = "200" ]; then
    echo "- Visualization Dashboard: ✅ Running"
else
    echo "- Visualization Dashboard: ❌ Not responding (HTTP $VIS_HEALTH_STATUS)"
fi

if [ "$DATA_HEALTH_STATUS" = "200" ]; then
    echo "- Data Ingestion API: ✅ Running"
else
    echo "- Data Ingestion API: ❌ Not responding (HTTP $DATA_HEALTH_STATUS)"
fi

# Open dashboard in browser if it's responding
if [ "$VIS_HEALTH_STATUS" = "200" ]; then
    echo ""
    echo "🌐 Opening dashboard in browser..."
    open "$VIS_URL" || xdg-open "$VIS_URL" || echo "Could not open browser automatically. Please visit $VIS_URL"
fi

# Show example API calls
echo ""
echo "📝 Example API calls:"
echo "# Send test data"
echo "curl -X POST $DATA_URL/api/data \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -d '{\"device_id\": \"test-001\", \"temperature\": 25.5, \"humidity\": 60}'"
echo ""
echo "# Check service health"
echo "curl $DATA_URL/health"
echo "curl $VIS_URL/health"

# Remind about port-forwarding if that's what we're using
if [[ "$VIS_URL" == *"localhost"* ]]; then
    echo ""
    echo "⚠️  Using port forwarding. Keep this terminal open to maintain the connection."
    echo "   To stop, press Ctrl+C"
    echo ""
    # Keep the script running so the port-forwarding stays active
    echo "Press Ctrl+C to quit and close port forwarding"
    tail -f /dev/null
fi
