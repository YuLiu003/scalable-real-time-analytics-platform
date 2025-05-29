#!/bin/bash

# Test the Kafka KRaft mode with Docker Compose
set -e

echo "Testing Kafka KRaft mode with Docker Compose"

# Check if docker-compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo "Error: docker-compose is not installed"
    exit 1
fi

# Stop any running containers
echo "Stopping any running Docker containers..."
docker-compose down 2>/dev/null || true

# Clean up volumes for a fresh start
echo "Cleaning up Kafka and Zookeeper volumes..."
docker volume rm $(docker volume ls -q | grep -E 'kafka|zookeeper') 2>/dev/null || true

# Start the new KRaft configuration
echo "Starting services with KRaft mode..."
docker-compose -f docker-compose-kraft.yml up -d

# Wait for Kafka to be ready
echo "Waiting for Kafka to be ready..."
for i in {1..30}; do
    if docker-compose -f docker-compose-kraft.yml exec kafka kafka-topics --bootstrap-server localhost:9092 --list &>/dev/null; then
        echo "Kafka is ready!"
        break
    fi
    
    if [ $i -eq 30 ]; then
        echo "Error: Kafka did not start within the expected time."
        echo "Checking Kafka logs..."
        docker-compose -f docker-compose-kraft.yml logs kafka
        exit 1
    fi
    
    echo "Waiting for Kafka to start... ($i/30)"
    sleep 5
done

# Create test topics
echo "Creating test topics..."
docker-compose -f docker-compose-kraft.yml exec kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic test-topic --partitions 3 --replication-factor 1

# List topics
echo "Listing topics:"
docker-compose -f docker-compose-kraft.yml exec kafka kafka-topics --bootstrap-server localhost:9092 --list

# Send test message
echo "Sending test message..."
echo "This is a test message from KRaft mode Kafka" | docker-compose -f docker-compose-kraft.yml exec -T kafka kafka-console-producer --bootstrap-server localhost:9092 --topic test-topic

# Read test message
echo "Reading test message:"
docker-compose -f docker-compose-kraft.yml exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic test-topic --from-beginning --max-messages 1

echo ""
echo "KRaft mode test completed successfully!"
echo ""
echo "To stop the services, run:"
echo "docker-compose -f docker-compose-kraft.yml down"
echo ""
echo "To view Kafka logs:"
echo "docker-compose -f docker-compose-kraft.yml logs -f kafka" 