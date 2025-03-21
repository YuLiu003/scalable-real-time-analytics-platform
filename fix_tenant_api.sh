#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Building updated Flask API...${NC}"

# Make sure minikube is running and connected
eval $(minikube docker-env)

# Build the updated image
docker build -t flask-api:latest -f flask-api/Dockerfile ./flask-api

# Restart the deployment
echo -e "${BLUE}Restarting flask-api deployment...${NC}"
kubectl rollout restart deployment/flask-api -n analytics-platform

# Wait for deployment to complete
echo -e "${BLUE}Waiting for deployment to complete...${NC}"
kubectl rollout status deployment/flask-api -n analytics-platform

echo -e "${GREEN}Deployment completed! Testing the updated API...${NC}"

# Test the API with port forwarding
echo -e "${BLUE}Testing API via port forwarding...${NC}"
kubectl port-forward service/flask-api-service -n analytics-platform 8085:80 &
PF_PID=$!
sleep 3

# Try sending a test request
DEVICE_ID="test-device-$(date +%s)"
RESPONSE=$(curl -s -X POST \
  "http://localhost:8085/api/data" \
  -H "X-API-Key: test-key-1" \
  -H "Content-Type: application/json" \
  -d "{\"device_id\": \"$DEVICE_ID\", \"temperature\": 25.5, \"humidity\": 60}" \
  -w "\n%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_STATUS" -eq 200 ]; then
  echo -e "${GREEN}SUCCESS: Data sent successfully (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
else
  echo -e "${RED}FAILED: Could not send data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
fi

# Now query the data back
sleep 2
RESPONSE=$(curl -s "http://localhost:8085/api/data?device_id=$DEVICE_ID" \
  -H "X-API-Key: test-key-1" \
  -w "\n%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_STATUS" -eq 200 ] && [[ "$BODY" == *"$DEVICE_ID"* ]]; then
  echo -e "${GREEN}SUCCESS: Retrieved data correctly (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
else
  echo -e "${RED}FAILED: Could not retrieve data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
fi

# Clean up port forwarding
kill $PF_PID

echo -e "${BLUE}Ready to run the full multi-tenant test!${NC}"
echo -e "${YELLOW}Run: ./test_multi_tenant_local.sh${NC}"