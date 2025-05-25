#!/bin/bash

# This script performs benchmarking tests comparing different Go implementations
# Usage: ./benchmark-go.sh [requests] [concurrency]

set -e

# Get API keys from environment or use defaults for testing
API_KEY_1=${API_KEY_1:-"demo-key-for-testing-only"}
API_KEY_2=${API_KEY_2:-"demo-key-for-testing-only"}

# SECURITY WARNING: Default API keys are for testing only!
# For production benchmarks, set API_KEY_1 and API_KEY_2 environment variables
if [[ "$API_KEY_1" == "demo-key-for-testing-only" ]]; then
    echo "⚠️  WARNING: Using demo API keys. Set API_KEY_1 and API_KEY_2 for production tests."
fi

REQUESTS=${1:-1000}    # Number of requests to send, default 1000
CONCURRENCY=${2:-10}   # Number of concurrent requests, default 10
TARGET_ENDPOINT="/api/data"
DATA_FILE="/tmp/test-data.json"
TEST_REPORT="/tmp/benchmark-report.txt"
GO_SVC1_HOST="http://localhost:5001"  # First service endpoint
GO_SVC2_HOST="http://localhost:5002"  # Second service endpoint

cat > $DATA_FILE << EOF
{
  "device_id": "benchmark-device-001",
  "temperature": 25.5,
  "humidity": 60,
  "tenant_id": "benchmark"
}
EOF

echo "📊 Starting benchmark tests comparing Go service implementations"
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

echo "🔄 Deploying data-ingestion-go service..."
echo "Running docker-compose up -d data-ingestion-go"
docker-compose up -d data-ingestion-go

# Wait for service to become available
echo "⏱️ Waiting for data-ingestion-go service to start..."
sleep 10
while ! curl -s "$GO_SVC1_HOST/health" > /dev/null; do
  sleep 2
  echo "Still waiting for data-ingestion-go service..."
done

# Run benchmark for first Go implementation
run_benchmark "data-ingestion-go" "$GO_SVC1_HOST" "$API_KEY_1"

echo "🔄 Deploying clean-ingestion-go service..."
echo "Running docker-compose up -d clean-ingestion-go"
docker-compose up -d clean-ingestion-go

# Wait for service to become available
echo "⏱️ Waiting for clean-ingestion-go service to start..."
sleep 10
while ! curl -s "$GO_SVC2_HOST/health" > /dev/null; do
  sleep 2
  echo "Still waiting for clean-ingestion-go service..."
done

# Run benchmark for second Go implementation
run_benchmark "clean-ingestion-go" "$GO_SVC2_HOST" "$API_KEY_2"

echo "✅ Benchmark tests completed!"
echo "Test report saved to $TEST_REPORT"
echo ""
echo "Summary:"
echo "-------------------------------------------------"
grep -A 3 "Requests per second" $TEST_REPORT
echo "-------------------------------------------------"
grep -A 3 "Time per request" $TEST_REPORT

# Calculate performance difference
SVC1_RPS=$(grep "Requests per second" $TEST_REPORT | head -1 | awk '{print $4}')
SVC2_RPS=$(grep "Requests per second" $TEST_REPORT | tail -1 | awk '{print $4}')
DIFFERENCE=$(echo "scale=2; ($SVC2_RPS - $SVC1_RPS) * 100 / $SVC1_RPS" | bc)

echo "-------------------------------------------------"
echo "Performance difference: $DIFFERENCE%"
echo "-------------------------------------------------"