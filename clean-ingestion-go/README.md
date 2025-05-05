# Clean Ingestion Service (Go Implementation)

This service is responsible for receiving and validating sensor data. It's a basic implementation that logs the data received for further processing. It's written in Go using the Gin web framework.

## Features

- RESTful API endpoint for data ingestion
- Data validation
- Health check endpoint
- Unit tests

## API Endpoints

### POST /api/data

Receives and logs sensor data.

**Request Headers**:
- `Content-Type: application/json`

**Request Body**:
```json
{
  "device_id": "device-001",
  "temperature": 25.5,
  "humidity": 60,
  // any other sensor data
}
```

**Response**:
```json
{
  "status": "success",
  "message": "Data logged successfully",
  "data": {...}  // Echo of the data received
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
go build -o clean-ingestion .
```

### Docker

Build the Docker image:

```bash
docker build -t clean-ingestion-go:latest .
```

Run the Docker container:

```bash
docker run -p 5000:5000 clean-ingestion-go:latest
```

## Kubernetes Deployment

Apply the Kubernetes manifests:

```bash
kubectl apply -f k8s/clean-ingestion-go-deployment.yaml
kubectl apply -f k8s/clean-ingestion-go-service.yaml
```

Or use the deployment script:

```bash
./scripts/deploy-clean-ingestion-go.sh
``` 