# Data Ingestion Service (Go Implementation)

This service is responsible for ingesting sensor data via a REST API and publishing it to Kafka for further processing. It's written in Go using the Gin web framework.

## Features

- RESTful API endpoint for data ingestion
- API key authentication
- Kafka integration for message publishing
- Input validation
- Health check endpoint
- Unit tests

## API Endpoints

### POST /api/data

Ingests sensor data and publishes it to Kafka.

**Request Headers**:
- `Content-Type: application/json`
- `X-API-Key: <api-key>` (required unless BYPASS_AUTH=true)

**Request Body**:
```json
{
  "device_id": "device-001",  // required
  "temperature": 25.5,
  "humidity": 60,
  // any other sensor data
}
```

**Response**:
```json
{
  "status": "success",
  "message": "Data received successfully",
  "data": {...},  // Echo of the data received with added timestamp if missing
  "kafka_status": "sent"  // or error message if Kafka publishing failed
}
```

### GET /health

Simple health check endpoint.

**Response**:
```json
{
  "status": "healthy"
}
```

## Configuration

The service is configured via environment variables:

- `KAFKA_BROKER`: Kafka broker address (default: "localhost:9092")
- `KAFKA_TOPIC`: Kafka topic to publish to (default: "sensor-data")
- `BYPASS_AUTH`: If "true", bypasses API key authentication (default: "false")
- `PORT`: Port to listen on (default: "5000")

## Development

### Prerequisites

- Go 1.21+

### Running Locally

```bash
go run main.go
```

### Running Tests

```bash
go test -v
```

### Building

```bash
go build -o data-ingestion .
```

### Docker

Build the Docker image:

```bash
docker build -t data-ingestion-go:latest .
```

Run the Docker container:

```bash
docker run -p 5000:5000 \
  -e KAFKA_BROKER=kafka:9092 \
  -e KAFKA_TOPIC=sensor-data \
  data-ingestion-go:latest
```

## Kubernetes Deployment

Apply the Kubernetes manifests:

```bash
kubectl apply -f k8s/data-ingestion-go-deployment.yaml
kubectl apply -f k8s/data-ingestion-go-service.yaml
```

Or use the deployment script:

```bash
./scripts/deploy-data-ingestion-go.sh
``` 