# Scalable Real-Time Data Analytics Platform

## Overview

The Scalable Real-Time Data Analytics Platform is a cloud-native application designed to process and analyze data in real-time. This project showcases the integration of various modern technologies to build a high-performance data processing system. It is structured as a microservices architecture with Kafka as the messaging backbone, ensuring scalability, maintainability, and efficiency.

## Architecture

![Architecture Diagram](https://github.com/user-attachments/assets/0b96f0c6-d791-4253-9e6e-45a7f63c6c4a)

The architecture consists of four main microservices:

1. **Data Ingestion Layer**: 
   - REST API endpoints for receiving sensor data
   - Publishes events to Kafka topics
   - Handles data validation and error handling

2. **Processing Engine**:
   - Consumes data from Kafka
   - Performs real-time analytics on incoming data
   - Detects anomalies and processes time-series metrics

3. **Storage Layer**:
   - Persists processed data
   - Manages retention policies
   - Provides query interfaces for historical data

4. **Visualization Dashboard**:
   - Real-time metrics display using WebSockets
   - Interactive charts and graphs
   - System health monitoring

## Technology Stack

- **Backend**: Python, Flask
- **Messaging**: Apache Kafka (KRaft mode, no ZooKeeper)
- **Data Processing**: NumPy
- **Frontend**: HTML, CSS, JavaScript
- **Containerization**: Docker
- **Orchestration**: Kubernetes (Minikube for local deployment)
- **Monitoring**: Prometheus
- **Real-time Communication**: Flask-SocketIO

## Project Structure

The project is organized as follows:

- `data-ingestion/`: Service for ingesting data from sensors/devices
- `processing-engine/`: Service for real-time data processing
- `storage-layer/`: Service for data persistence
- `visualization/`: Web dashboard for data visualization
- `flask-api/`: Secure API for external access
- `k8s/`: Kubernetes manifests for deployment
- `test/`: Unit and integration tests
- `scripts/`: Utility scripts for deployment and security checks

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- Python 3.9+

### Setup and Deployment

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/YuLiu003/real-time-analytics-platform.git
   cd real-time-analytics-platform
   ```

2. **Start Minikube**:
   ```bash
   minikube start
   ```

3. **Build Container Images**:
   ```bash
   ./build-images.sh
   ```
   This script builds all necessary container images with the correct names for Kubernetes.

4. **Deploy the Platform**:
   ```bash
   ./deploy-platform.sh
   ```
   This script deploys all platform components:
   - Creates the namespace and applies configurations
   - Deploys Kafka in KRaft mode (no ZooKeeper needed)
   - Deploys microservices (data ingestion, processing, storage, visualization)
   - Sets up Prometheus for monitoring

5. **Access the Dashboard**:
   ```bash
   minikube service visualization-service -n analytics-platform
   ```

6. **Check Platform Status**:
   ```bash
   ./manage.sh status
   ```

## Security

### Secret Management

The platform uses several secrets for secure operation:

1. **Kafka Secrets**: 
   - Generated automatically on first deployment
   - Stored in `k8s/kafka-secrets.yaml` (excluded from git)
   - Contains the KRaft Cluster ID for Kafka

2. **API Keys**:
   - Stored in `k8s/secrets.yaml` (excluded from git)
   - Contains authentication keys for external API access

3. **Grafana Credentials**:
   - Admin password stored in `k8s/secrets.yaml`
   - Generated during deployment

### Security Checks

Run the security check script before deployment to identify potential issues:

```bash
security-check.sh
```

This script checks for:
- Hardcoded secrets
- Proper security contexts in deployments
- Network policies
- Resource limits
- Health probes
- API authentication
- Non-root user configuration
- Secret handling

### Container Security

All containers follow security best practices:
- Run as non-root users
- Use minimal capabilities
- Have resource limits defined
- Include health probes for resilience

## Usage

### Sending Test Data

```bash
# Using curl
curl -X POST http://$(minikube ip):30085/api/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"device_id": "test-001", "temperature": 25.5, "humidity": 60}'
```

### Monitoring

```bash
# Check pod status
kubectl get pods -n analytics-platform

# View service logs
kubectl logs -n analytics-platform deployment/visualization
kubectl logs -n analytics-platform deployment/data-ingestion

# Access Prometheus
minikube service prometheus-service -n analytics-platform
```

## Troubleshooting

### Common Issues and Solutions

1. **Image Pull Errors**:
   - Ensure images are built correctly: build-images.sh
   - Verify images exist: `docker images | grep -E '(flask-api|data-ingestion|processing-engine|storage-layer|visualization)'`

2. **Kafka Connection Issues**:
   - Check that Kafka is running: `kubectl get pods -n analytics-platform -l app=kafka`
   - Verify Kafka service: `kubectl get svc -n analytics-platform | grep kafka`
   - Check Kafka logs: `kubectl logs -n analytics-platform deployment/kafka`

3. **CreateContainerConfigError**:
   - Verify secrets exist: `kubectl get secrets -n analytics-platform`
   - Check deployment configuration: `kubectl describe deployment/kafka -n analytics-platform`

## Development

### Updating the Application

After making code changes:

1. Rebuild images:
   ```bash
   ./build-images.sh
   ```

2. Restart specific deployments:
   ```bash
   kubectl rollout restart deployment <deployment-name> -n analytics-platform
   ```

3. For major changes, redeploy everything:
   ```bash
   ./deploy-platform.sh
   ```

### Local Development with Docker Compose

For rapid development without Kubernetes:

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose down
```

Note: The docker-compose.yml uses environment variables for secrets. Copy `.env.sample` to .env and update values before running.

## Security Features

This project implements several security measures:

- **API Authentication**: Protected endpoints with API key authentication
- **Container Hardening**: All containers run as non-root with minimal capabilities
- **Network Policies**: Default deny policy with explicit allow rules
- **Secrets Management**: Sensitive data stored in Kubernetes secrets
- **Security Scanning**: Automated checking with security-check.sh

## Next Steps

- **Grafana Integration**: Add Grafana dashboards connected to Prometheus
- **Scaling**: Test horizontal scaling of services under load
- **Persistence**: Implement persistent volumes for Kafka and storage layer
- **CI/CD**: Add GitHub Actions for automated testing and deployment

## License

This project is licensed under the MIT License - see the LICENSE file for details.
