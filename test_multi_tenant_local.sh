#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Start port forwarding in the background
echo -e "${BLUE}Setting up port forwarding...${NC}"
kubectl port-forward service/flask-api-service -n analytics-platform 8085:80 &
PF_PID=$!

# Give it a moment to establish
sleep 3
echo -e "${GREEN}Port forwarding established on port 8085${NC}"

# Test device IDs
DEVICE_T1="device-t1-$(date +%s)"
DEVICE_T2="device-t2-$(date +%s)"
API_ENDPOINT="http://localhost:8085"  # Port forwarded endpoint

echo -e "${BLUE}Testing with device IDs: $DEVICE_T1 (tenant1) and $DEVICE_T2 (tenant2)${NC}"

# Send data as tenant1
echo -e "\n${YELLOW}TEST 1: Sending data as tenant1...${NC}"
RESPONSE=$(curl -s -X POST \
  "$API_ENDPOINT/api/data" \
  -H "X-API-Key: test-key-1" \
  -H "Content-Type: application/json" \
  -d "{\"device_id\": \"$DEVICE_T1\", \"temperature\": 25.5, \"humidity\": 60}" \
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

# Send data as tenant2
echo -e "\n${YELLOW}TEST 2: Sending data as tenant2...${NC}"
RESPONSE=$(curl -s -X POST \
  "$API_ENDPOINT/api/data" \
  -H "X-API-Key: test-key-2" \
  -H "Content-Type: application/json" \
  -d "{\"device_id\": \"$DEVICE_T2\", \"temperature\": 27.0, \"humidity\": 55}" \
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

# Wait for data processing
echo -e "\n${BLUE}Waiting for data processing...${NC}"
sleep 5

# Query tenant1 data as tenant1 (should work)
echo -e "\n${YELLOW}TEST 3: Querying tenant1 data as tenant1 (should succeed)...${NC}"
RESPONSE=$(curl -s "$API_ENDPOINT/api/data?device_id=$DEVICE_T1" \
  -H "X-API-Key: test-key-1" \
  -w "\n%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_STATUS" -eq 200 ] && [[ "$BODY" == *"$DEVICE_T1"* ]]; then
  echo -e "${GREEN}SUCCESS: Retrieved tenant1 data correctly (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
else
  echo -e "${RED}FAILED: Could not retrieve tenant1 data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
fi

# Query tenant1 data as tenant2 (should fail)
echo -e "\n${YELLOW}TEST 4: Querying tenant1 data as tenant2 (should fail)...${NC}"
RESPONSE=$(curl -s "$API_ENDPOINT/api/data?device_id=$DEVICE_T1" \
  -H "X-API-Key: test-key-2" \
  -w "\n%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_STATUS" -eq 403 ] || [[ "$BODY" == *"not authorized"* ]] || [[ "$BODY" == *"No data found"* ]]; then
  echo -e "${GREEN}SUCCESS: Correctly denied access to tenant1 data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
else
  echo -e "${RED}FAILED: Incorrectly granted access to tenant1 data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
fi

# Query tenant2 data as tenant2 (should work)
echo -e "\n${YELLOW}TEST 5: Querying tenant2 data as tenant2 (should succeed)...${NC}"
RESPONSE=$(curl -s "$API_ENDPOINT/api/data?device_id=$DEVICE_T2" \
  -H "X-API-Key: test-key-2" \
  -w "\n%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_STATUS" -eq 200 ] && [[ "$BODY" == *"$DEVICE_T2"* ]]; then
  echo -e "${GREEN}SUCCESS: Retrieved tenant2 data correctly (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
else
  echo -e "${RED}FAILED: Could not retrieve tenant2 data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
fi

# Query tenant2 data as tenant1 (should fail)
echo -e "\n${YELLOW}TEST 6: Querying tenant2 data as tenant1 (should fail)...${NC}"
RESPONSE=$(curl -s "$API_ENDPOINT/api/data?device_id=$DEVICE_T2" \
  -H "X-API-Key: test-key-1" \
  -w "\n%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_STATUS" -eq 403 ] || [[ "$BODY" == *"not authorized"* ]] || [[ "$BODY" == *"No data found"* ]]; then
  echo -e "${GREEN}SUCCESS: Correctly denied access to tenant2 data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
else
  echo -e "${RED}FAILED: Incorrectly granted access to tenant2 data (HTTP $HTTP_STATUS)${NC}"
  echo -e "Response: $BODY"
fi

echo -e "\n${BLUE}==== Multi-tenant test complete ====${NC}"

# Clean up port forwarding
echo -e "${BLUE}Cleaning up port forwarding...${NC}"
kill $PF_PID