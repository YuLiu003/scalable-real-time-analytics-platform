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

2. **Initial Setup**:
   ```bash
   ./setup.sh
   ```

3. **Deploy to Kubernetes**:
   ```bash
   ./build-images.sh
   ./deploy-platform.sh
   ```
   This script builds the container images and deploys the platform components in the correct order:
   - Creates namespace and applies configurations
   - Deploys Kafka messaging infrastructure
   - Deploys the data ingestion, processing, storage, and visualization services

4. **Access the Dashboard**:
   ```bash
   minikube service visualization-service -n analytics-platform
   
   # Method 1: Port forwarding
   ./open-dashboard.sh
   
   # Method 2: Ingress (requires additional setup)
   minikube addons enable ingress
   minikube tunnel
   # Then visit http://192.168.49.2/
   ```

## Deployment with Secrets

Before deploying, you need to set up your secrets:

1. Create a copy of the secrets template:
   ```bash
   cp k8s/secrets.yaml.template k8s/secrets.yaml
   ```

2. Edit the secrets file with your actual credentials.

## Usage

### Sending Test Data

```bash
# Using curl
curl -X POST http://localhost:8080/api/data \
  -H "Content-Type: application/json" \
  -d '{"device_id": "test-001", "temperature": 25.5, "humidity": 60}'
```

### Monitoring

```bash
# Check pod status
kubectl get pods -n analytics-platform

# View service logs
kubectl logs -n analytics-platform deployment/visualization
kubectl logs -n analytics-platform deployment/data-ingestion

# Test Kafka connectivity
kubectl run kafka-test -n analytics-platform --image=ubuntu:20.04 -- sleep infinity
kubectl exec -it -n analytics-platform kafka-test -- bash -c "apt-get update && apt-get install -y kafkacat"
kubectl exec -it -n analytics-platform kafka-test -- kafkacat -b kafka-service:9092 -L
```

## Troubleshooting

### Common Issues and Solutions

1. **Kafka Connection Issues**:
   - Check that Kafka is running: `kubectl get pods -n analytics-platform -l app=kafka`
   - Verify Kafka service: `kubectl get svc -n analytics-platform | grep kafka`
   - Check Kafka logs: `kubectl logs -n analytics-platform -l app=kafka`

2. **Data Flow Problems**:
   - Examine Kafka topics: `kubectl exec -it -n analytics-platform kafka-test -- kafkacat -b kafka-service:9092 -C -t sensor-data -o beginning -c 5`
   - Check service logs for connection errors

## Development

### Updating the Application

After making changes to the code:

1. Rebuild images:
   ```bash
   docker-compose build
   ```

2. Restart specific deployments:
   ```bash
   kubectl rollout restart deployment <deployment-name> -n analytics-platform
   ```

3. For major changes, redeploy everything:
   ```bash
   ./deploy-platform.sh
   ```

## Security Features

This project implements several security measures:

- **API Authentication**: Protected endpoints with API key authentication
- **Container Hardening**: All containers run as non-root with minimal capabilities
- **Network Policies**: Default deny policy with explicit allow rules
- **Security Scanning**: Run `./scripts/security-check.sh` to verify security settings

For more details, see [SECURITY.md](./SECURITY.md).

## Next Steps

- **Monitoring**: Add Prometheus and Grafana for platform monitoring
- **Scaling**: Test horizontal scaling of services under load
- **Persistence**: Implement persistent volumes for Kafka
- **Security**: Add authentication and encryption for Kafka connections

## License

This project is licensed under the MIT License - see the LICENSE file for details.
