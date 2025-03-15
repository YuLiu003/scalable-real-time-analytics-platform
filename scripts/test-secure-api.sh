#!/bin/bash
set -e

echo "🔒 Testing Secure API Authentication"

# Test variables
PORT=8090
KEY="test-key-1"
INVALID_KEY="invalid-key-123"

# Setup port forwarding
setup_port_forwarding() {
  echo "Setting up port forwarding to secure-api service..."
  kubectl port-forward -n analytics-platform svc/secure-api-service $PORT:80 &
  PORT_FORWARD_PID=$!
  sleep 2

  # Ensure port forwarding is killed when script exits
  trap "kill $PORT_FORWARD_PID 2>/dev/null || true" EXIT
}

# Clean any existing port forwarding
pkill -f "port-forward.*$PORT" || true
setup_port_forwarding

# Run a series of authentication tests
echo -e "\n1. Testing without API key (should fail):"
curl -s -o /dev/null -w "  Status code: %{http_code}\n" \
  -X POST http://localhost:$PORT/api/data \
  -H "Content-Type: application/json" \
  -d '{"device_id": "test-001", "temperature": 25.5, "humidity": 60}'

echo -e "\n2. Testing with invalid API key (should fail):"
curl -s -o /dev/null -w "  Status code: %{http_code}\n" \
  -X POST http://localhost:$PORT/api/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $INVALID_KEY" \
  -d '{"device_id": "test-001", "temperature": 25.5, "humidity": 60}'

echo -e "\n3. Testing with valid API key (should succeed):"
curl -s -o /dev/null -w "  Status code: %{http_code}\n" \
  -X POST http://localhost:$PORT/api/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $KEY" \
  -d '{"device_id": "test-001", "temperature": 25.5, "humidity": 60}'

echo -e "\n4. Testing auth endpoint without API key (should fail):"
curl -s -o /dev/null -w "  Status code: %{http_code}\n" http://localhost:$PORT/auth-test

echo -e "\n5. Testing auth endpoint with valid API key (should succeed):"
curl -s -o /dev/null -w "  Status code: %{http_code}\n" \
  http://localhost:$PORT/auth-test -H "X-API-Key: $KEY"

echo -e "\n✅ Security tests complete!"
