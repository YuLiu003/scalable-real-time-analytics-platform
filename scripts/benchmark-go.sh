#!/bin/bash

# This script performs benchmarking tests comparing Python and Go implementations
# Usage: ./benchmark-go.sh [requests] [concurrency]

set -e

REQUESTS=${1:-1000}    # Number of requests to send, default 1000
CONCURRENCY=${2:-10}   # Number of concurrent requests, default 10
TARGET_ENDPOINT="/api/data"
DATA_FILE="/tmp/test-data.json"
TEST_REPORT="/tmp/benchmark-report.txt"
PYTHON_HOST="http://localhost:5001"
GO_HOST="http://localhost:5001"  # Same port, use Docker Compose to switch between them

cat > $DATA_FILE << EOF
{
  "device_id": "benchmark-device-001",
  "temperature": 25.5,
  "humidity": 60,
  "tenant_id": "benchmark"
}
EOF

echo "📊 Starting benchmark tests comparing Python and Go implementations"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "Target endpoint: $TARGET_ENDPOINT"
echo ""

# Function to run benchmark test
run_benchmark() {
  local name=$1
  local host=$2
  local api_key=$3
  
  echo "🚀 Running benchmark for $name implementation..."
  echo "-------------------------------------------------" | tee -a $TEST_REPORT
  echo "Test: $name @ $host$TARGET_ENDPOINT" | tee -a $TEST_REPORT
  echo "Date: $(date)" | tee -a $TEST_REPORT
  echo "Settings: $REQUESTS requests, $CONCURRENCY concurrent" | tee -a $TEST_REPORT
  echo "" | tee -a $TEST_REPORT
  
  # Run benchmark with Apache Bench
  ab -n $REQUESTS -c $CONCURRENCY -H "Content-Type: application/json" \
     -H "X-API-Key: $api_key" -p $DATA_FILE "$host$TARGET_ENDPOINT" | tee -a $TEST_REPORT
  
  echo "" | tee -a $TEST_REPORT
}

# Make sure the report is empty
> $TEST_REPORT

echo "🔄 Deploying Python implementation..."
echo "Running docker-compose up -d python"
docker-compose stop data-ingestion-go
docker-compose up -d data-ingestion

# Wait for service to become available
echo "⏱️ Waiting for Python service to start..."
sleep 10
while ! curl -s "$PYTHON_HOST/health" > /dev/null; do
  sleep 2
  echo "Still waiting for Python service..."
done

# Run benchmark for Python implementation
run_benchmark "Python" "$PYTHON_HOST" "test-key-1"

echo "🔄 Deploying Go implementation..."
echo "Running docker-compose up -d go"
docker-compose stop data-ingestion
docker-compose up -d data-ingestion-go

# Wait for service to become available
echo "⏱️ Waiting for Go service to start..."
sleep 10
while ! curl -s "$GO_HOST/health" > /dev/null; do
  sleep 2
  echo "Still waiting for Go service..."
done

# Run benchmark for Go implementation
run_benchmark "Go" "$GO_HOST" "test-key-1"

echo "✅ Benchmark tests completed!"
echo "Test report saved to $TEST_REPORT"
echo ""
echo "Summary:"
echo "-------------------------------------------------"
grep -A 3 "Requests per second" $TEST_REPORT
echo "-------------------------------------------------"
grep -A 3 "Time per request" $TEST_REPORT

# Calculate performance difference
PY_RPS=$(grep "Requests per second" $TEST_REPORT | head -1 | awk '{print $4}')
GO_RPS=$(grep "Requests per second" $TEST_REPORT | tail -1 | awk '{print $4}')
IMPROVEMENT=$(echo "scale=2; ($GO_RPS - $PY_RPS) * 100 / $PY_RPS" | bc)

echo "-------------------------------------------------"
echo "Performance improvement: $IMPROVEMENT%"
echo "-------------------------------------------------" 