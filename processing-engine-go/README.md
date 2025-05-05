# Processing Engine (Go Implementation)

This service is responsible for processing sensor data from Kafka, performing analytics, and publishing results back to Kafka. It's written in Go with full multi-tenant support.

## Features

- Kafka consumer for streaming data processing
- Prometheus metrics integration
- Real-time data analytics and anomaly detection
- Multi-tenant isolation for data and processing
- REST API for monitoring and statistics
- Configurable tenant-specific processing parameters
- Integration with storage service

## API Endpoints

### GET /health

Health check endpoint to verify service status.

**Response**:
```json
{
  "status": "ok",
  "service": "processing-engine-go",
  "running": true,
  "timestamp": "2023-06-01T12:34:56Z"
}
```

### GET /api/stats

Get current processing statistics.

**Response**:
```json
{
  "status": "ok",
  "data": {
    "temperature": {
      "count": 120,
      "sum": 3000.5,
      "min": 18.5,
      "max": 32.7,
      "avg": 25.0
    },
    "humidity": {
      "count": 120,
      "sum": 7200.0,
      "min": 40.0,
      "max": 85.0,
      "avg": 60.0
    },
    "devices": {
      "device-001": {
        "temperature": {
          "readings": [24.5, 25.0, 24.8],
          "avg": 24.8,
          "min": 24.5,
          "max": 25.0
        },
        "humidity": {
          "readings": [60.0, 61.2, 59.8],
          "avg": 60.3,
          "min": 59.8,
          "max": 61.2
        },
        "last_seen": "2023-06-01T12:34:56Z"
      }
    }
  },
  "processing": {
    "running": true,
    "messages_processed": 120,
    "last_processed": "2023-06-01T12:34:56Z",
    "errors": 0
  }
}
```

### GET /api/stats/device/:id

Get statistics for a specific device.

**Response**:
```json
{
  "status": "ok",
  "device_id": "device-001",
  "data": {
    "temperature": {
      "readings": [24.5, 25.0, 24.8],
      "avg": 24.8,
      "min": 24.5,
      "max": 25.0
    },
    "humidity": {
      "readings": [60.0, 61.2, 59.8],
      "avg": 60.3,
      "min": 59.8,
      "max": 61.2
    },
    "last_seen": "2023-06-01T12:34:56Z"
  }
}
```

### GET /metrics

Prometheus metrics endpoint.

## Configuration

The service is configured via environment variables:

- `KAFKA_BROKER`: Kafka broker address (default: "localhost:9092")
- `INPUT_TOPIC`: Kafka topic to consume from (default: "sensor-data")
- `OUTPUT_TOPIC`: Kafka topic to produce to (default: "processed-data")
- `CONSUMER_GROUP`: Kafka consumer group (default: "processing-engine-go-new")
- `METRICS_PORT`: Port for HTTP server (default: 8000)
- `STORAGE_SERVICE_URL`: URL of the storage service (default: "http://storage-layer-service")
- `MAX_READINGS`: Maximum number of readings to keep per device (default: 100)
- `KAFKA_ENABLED`: Enable/disable Kafka integration (default: true)
- `DEBUG`: Enable/disable debug mode (default: false)

## Tenant-specific Processing

The service supports tenant-specific processing parameters:

- `AnomalyThreshold`: Threshold for anomaly detection (higher for premium tenants)
- `SamplingRate`: Rate at which to sample data (1.0 = process all data, 0.5 = process half)

## Deployment

```bash
# Using script
./scripts/deploy-processing-engine-go-new.sh

# Manual deployment
cd processing-engine-go-new
docker build -t processing-engine-go-new:latest .
kubectl apply -f ../k8s/processing-engine-go-new-deployment.yaml
kubectl apply -f ../k8s/processing-engine-go-new-service.yaml
``` 