# Visualization Service (Go)

This is the Go implementation of the Visualization Service for the Real-Time Analytics Platform. The service provides real-time visualization of sensor data using WebSockets for live updates.

## Features

- Real-time sensor data visualization using WebSockets
- Chart.js integration for beautiful charts and graphs
- RESTful API endpoints for data access
- Prometheus metrics integration
- Health check endpoints for monitoring
- Multi-tenant support

## Architecture

The service is built using:

- **Gin** - High-performance HTTP web framework
- **Gorilla WebSockets** - WebSocket implementation for Go
- **Prometheus** - Metrics and monitoring
- **Chart.js** - JavaScript charting library for data visualization

## Directory Structure

```
visualization-go/
├── config/        - Configuration management
├── handlers/      - HTTP request handlers
├── metrics/       - Prometheus metrics definitions
├── middleware/    - HTTP middleware (metrics, logging)
├── models/        - Data models
├── static/        - Static assets (CSS, JS)
├── templates/     - HTML templates
├── websocket/     - WebSocket implementation
├── main.go        - Main application entry point
├── Dockerfile     - Container definition
└── README.md      - This file
```

## API Endpoints

- `GET /` - Main dashboard UI
- `GET /ws` - WebSocket connection for real-time updates
- `GET /health` - Health check endpoint
- `GET /metrics` - Prometheus metrics
- `GET /api/status` - System status
- `POST /api/data` - Send sensor data
- `GET /api/data/recent` - Get recent sensor data

## Dependencies

- Go 1.21+
- Gin Web Framework
- Gorilla WebSockets
- Prometheus Client

## Local Development

1. Build the service:
```
./build.sh
```

2. Run the service:
```
./visualization-go
```

3. Access the UI at: http://localhost:5003

## Deployment

The service can be deployed to Kubernetes using the provided deployment script:

```
../scripts/deploy-visualization-go.sh
```

This script will:
- Build the Docker image
- Create Kubernetes deployment manifests if needed
- Deploy the service to the `analytics-platform` namespace
- Configure service with NodePort for external access

## Environment Variables

- `PORT` - HTTP server port (default: 5003)
- `DATA_SERVICE_URL` - URL of the data ingestion service (default: http://data-ingestion-service)
- `MAX_DATA_POINTS` - Maximum data points to store in memory (default: 100)
- `DEBUG_MODE` - Enable debug mode (default: false)
- `GIN_MODE` - Gin framework mode (default: release)

## Comparison with Python Version

This Go implementation provides the same functionality as the Python/Flask implementation with the following advantages:

- Significantly lower memory usage
- Improved performance for concurrent connections
- Native WebSocket support without additional dependencies
- Strong typing and compile-time checks
- Smaller container footprint

# Go Migration

The platform has been completely migrated from Python to Go for improved performance, decreased memory usage, and better concurrency support. All Python components have been removed except for the tenant management service. The migration provides the following benefits:

- Reduced memory usage (~30%)
- Faster startup times (~50% improvement)
- Static typing for better code quality
- Improved error handling
- Better concurrency with goroutines
- Simplified deployment
- Multi-tenant isolation at process level

## Components

The following components are now implemented in Go:

- **data-ingestion-go**: Handles inbound data from sensors via REST API
- **clean-ingestion-go**: Validates and sanitizes data
- **processing-engine-go-new**: Processes data and detects anomalies using Sarama Kafka client
- **visualization-go**: Provides real-time visualization with WebSocket support
- **storage-layer-go**: Persists data with improved SQL transaction support

The Python tenant management component remains unchanged for now.

## Deployment

To deploy the platform with Go components:

```bash
./deploy-platform.sh
```

To test the platform:

```bash
./test-go-platform.sh
```

## File Structure

All Python components and redundant Go files have been permanently removed. The codebase now consists only of Go implementations.

# Real-Time Analytics Platform

A scalable, secure, multi-tenant analytics platform built with microservices architecture on Kubernetes.

## Quick Start

### One-Command Setup (Recommended)

```bash
# Complete setup: starts minikube, builds images, and deploys everything
./manage.sh setup-all
```

### Manual Setup Process

```bash
# Start Minikube (if not already running)
./manage.sh start

# Build all Docker images
./manage.sh build

# Deploy the entire platform with security features
./manage.sh deploy

# Check status of all components
./manage.sh status
```

## Accessing Services

After deploying the platform, you can access the services using these commands:

### Visualization Dashboard
```bash
# Open the visualization dashboard in your browser
minikube service visualization-go-service -n analytics-platform
```

### Monitoring Dashboards
```bash
# Deploy both Prometheus and Grafana if not already done
./manage.sh monitoring

# Access Prometheus metrics dashboard
./manage.sh prometheus
# Access via: http://localhost:9090

# Access Grafana dashboard
./manage.sh grafana
# Access via: http://localhost:3000
# Default credentials: admin / admin-secure-password
```

### Sending Test Data
```bash
# Send test data to the clean-ingestion service
curl -X POST http://$(minikube ip):30087/api/data \
  -H "Content-Type: application/json" \
  -d '{"sensor_id": "test1", "value": 25.5, "timestamp": "2023-06-01T12:00:00Z"}'
```

## Architecture Overview

This platform consists of interconnected microservices:

1. **Data Ingestion Service** - Receives and validates incoming data
2. **Clean Ingestion Service** - Sanitizes and validates data
3. **Processing Engine** - Processes data streams and detects anomalies
4. **Storage Layer** - Manages persistent data storage with SQLite
5. **Visualization Service** - Provides real-time visualization with WebSockets
6. **Tenant Management** - Handles multi-tenant configuration and isolation

## Components

All components are now implemented in Go for improved performance:

- **data-ingestion-go**: Handles inbound data from sensors via REST API
- **clean-ingestion-go**: Validates and sanitizes data
- **processing-engine-go**: Processes data and detects anomalies using Sarama Kafka client
- **visualization-go**: Provides real-time visualization with WebSocket support
- **storage-layer-go**: Persists data with improved SQL transaction support
- **tenant-management-go**: Manages tenant configuration and API keys

## Troubleshooting

If you encounter issues with the deployment, try the following:

### Complete Reset

If you need to start from scratch:

```bash
# Complete cleanup and reset
./manage.sh reset-all

# Then run setup again
./manage.sh setup-all
```

### Storage Layer Issues

If you encounter issues with the storage layer component:

```bash
# Fix the storage layer specifically
./manage.sh fix-storage
```

### Kafka Timing Out

If Kafka is taking too long to deploy or fails:

```bash
# Reset the full environment
./manage.sh reset-all

# Deploy with the updated configuration
./manage.sh setup-all
```

## Monitoring Setup

The platform includes a comprehensive monitoring stack:

### Prometheus
- Collects metrics from all services
- Exposes metrics via HTTP API
- Used for alerting and monitoring

### Grafana
- Provides visualization of metrics
- Pre-configured dashboards
- Connects to Prometheus data source
- Custom alert thresholds

To setup a Grafana dashboard:
1. Access Grafana at http://localhost:3000
2. Login with admin/admin-secure-password
3. Go to Dashboards → New Dashboard
4. Add a new panel
5. Select Prometheus as the data source
6. Query examples:
   - `rate(http_requests_total[5m])` - Request rate
   - `process_resident_memory_bytes` - Memory usage
   - `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))` - 95th percentile latency

## Management Commands

```bash
# View pod statuses
./manage.sh pods

# View logs for a specific pod
./manage.sh logs <pod-name>

# Get detailed information about a pod
./manage.sh describe <pod-name>

# Restart a specific deployment
./manage.sh restart <deployment-name>

# Restart all deployments
./manage.sh restart-all

# Start minikube tunnel for LoadBalancer services
./manage.sh tunnel-start

# Stop minikube tunnel
./manage.sh tunnel-stop
```

## Platform Security Features

- Network policies to control pod-to-pod communication
- RBAC for service accounts with minimal permissions
- API key authentication for tenant management
- Secure communication between microservices
- Non-root containers with restricted capabilities
- Resource limits and requests for all components
- Health and readiness probes for resilience

## Prerequisites

- Docker Desktop with Kubernetes enabled, or Minikube
- kubectl CLI tool
- Git
- Bash shell (Linux/Mac) or WSL/Git Bash (Windows)

## API Documentation

The platform exposes several REST APIs for data ingestion and retrieval:

### Data Ingestion API

`POST /api/data`

Request Headers:
- `Content-Type: application/json`
- `X-API-Key: [api-key]` (required)

Request Body:
```json
{
  "sensor_id": "sensor123",
  "value": 42.5,
  "timestamp": "2023-06-01T12:00:00Z"
}
```

### Visualization API

`GET /api/data/recent`

Query Parameters:
- `limit` - Number of recent data points (default: 100)
- `sensor_id` - Filter by specific sensor (optional)

WebSocket Connection:
- Connect to `/ws` for real-time data updates

## Known Issues and Troubleshooting

- If Kafka and Zookeeper fail to start, check the logs with `kubectl logs -n analytics-platform -l app=kafka` and `kubectl logs -n analytics-platform -l app=zookeeper`
- For local development, set `imagePullPolicy: Never` in the deployment files
- Some services require persistent storage - ensure PVCs are properly created
- Grafana may take up to 60 seconds to become ready due to strict readiness probe settings

## Security Recommendations for Production

This repository contains a demo/development setup. Before deploying to production, consider these security enhancements:

1. **Secret Management**:
   - Replace base64-encoded secrets with a proper secrets management solution (HashiCorp Vault, AWS Secrets Manager)
   - Use Kubernetes External Secrets or Sealed Secrets for GitOps workflows
   - Rotate secrets regularly using automated processes

2. **Container Security**:
   - Scan container images for vulnerabilities using tools like Trivy, Clair, or Snyk
   - Use minimal base images like distroless or alpine
   - Implement Pod Security Standards at the Restricted level
   - Consider using OPA/Gatekeeper or Kyverno for policy enforcement

3. **Network Security**:
   - Implement service mesh (Istio or Linkerd) for mTLS between services
   - Use NetworkPolicies to restrict communication between components
   - Add proper Ingress with TLS termination

4. **Authentication & Access Control**:
   - Implement proper OIDC authentication for external access
   - Use cert-manager for certificate management
   - Define proper RBAC roles with least privilege principle

5. **Monitoring & Compliance**:
   - Set up alerts for security events
   - Implement audit logging
   - Deploy a SIEM solution for security monitoring

Run `./scripts/security-check.sh` regularly to validate your security posture.