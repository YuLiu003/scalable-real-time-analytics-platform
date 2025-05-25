# Visualization Service

The visualization service provides a real-time dashboard for the analytics platform with WebSocket-powered live updates and interactive charts.

## Features

- **Real-time Updates** - WebSocket connections for live data streaming
- **Interactive Charts** - Chart.js integration for dynamic visualizations
- **Multi-tenant Views** - Tenant-specific data visualization
- **RESTful API** - Data access endpoints
- **Health Monitoring** - Built-in health checks and metrics

## Technology Stack

- **Go + Gin** - High-performance HTTP web framework
- **Gorilla WebSockets** - Real-time data streaming
- **Chart.js** - Client-side charting library
- **Prometheus** - Metrics and monitoring integration

## API Endpoints

- `GET /` - Main dashboard UI
- `GET /ws` - WebSocket connection for real-time updates
- `GET /health` - Health check endpoint
- `GET /metrics` - Prometheus metrics endpoint
- `GET /api/status` - System status information
- `GET /api/data/recent` - Retrieve recent sensor data

## Configuration

The service reads configuration from environment variables:

- `PORT` - HTTP server port (default: 5003)
- `KAFKA_BROKER` - Kafka broker address
- `STORAGE_SERVICE_URL` - Storage layer service URL

## WebSocket Protocol

The service provides real-time data via WebSocket at `/ws`:

```javascript
const ws = new WebSocket('ws://localhost:5003/ws');
ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    // Handle real-time sensor data
};
```

## Docker Deployment

```bash
# Build image
docker build -t visualization-go:latest .

# Run container
docker run -p 5003:5003 visualization-go:latest
```

## Local Development

```bash
# Install dependencies
go mod download

# Run service
go run main.go

# Access dashboard
open http://localhost:5003
```

## Metrics

The service exposes Prometheus metrics at `/metrics` including:
- HTTP request duration and count
- WebSocket connection count
- Data processing metrics
- Health status indicators
