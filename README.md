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
   ./manage.sh reset-minikube
   ```

3. **Build Container Images**:
   ```bash
   ./manage.sh build
   ```
   This script builds all necessary container images with the correct names for Kubernetes.

4. **Deploy the Platform**:
   ```bash
   ./manage.sh deploy
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
   - Maps to specific tenants for multi-tenant isolation

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

## Multi-Tenant Architecture

The platform supports secure multi-tenant isolation, allowing data from multiple customers to be processed in the same infrastructure while maintaining strict boundaries between tenant data.

### Tenant Isolation Features

- **API Key to Tenant Mapping**: Each API key is mapped to a specific tenant ID
- **Data Isolation**: Tenants can only access their own device data
- **Authorization Barriers**: Prevents cross-tenant data access
- **Tenant Context Propagation**: Tenant information flows through the entire processing pipeline

### Configuring Multi-Tenant Mode

Multi-tenant isolation is enabled by default. Configure it through these environment variables:

```yaml
# In ConfigMap or directly in deployment
ENABLE_TENANT_ISOLATION: "true"
TENANT_API_KEY_MAP: '{"test-key-1":"tenant1","test-key-2":"tenant2"}'
```

### Testing Tenant Isolation

The platform includes a test script to verify tenant isolation:

```bash
./test_multi_tenant_local.sh
```

This script tests:
1. Data ingestion for multiple tenants
2. Positive test cases (tenant accessing their own data)
3. Negative test cases (tenant attempting to access another tenant's data)

## Usage

### Sending Test Data

```bash
# Using curl - include the tenant-specific API key
curl -X POST http://$(minikube ip):30085/api/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-key-1" \
  -d '{"device_id": "test-001", "temperature": 25.5, "humidity": 60}'
```

### Querying Data (Tenant-Specific)

```bash
# Query data for a specific device (using tenant API key)
curl http://$(minikube ip):30085/api/data?device_id=test-001 \
  -H "X-API-Key: test-key-1"
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

4. **Tenant Isolation Issues**:
   - Verify tenant mapping environment variable: `kubectl describe configmap platform-config -n analytics-platform`
   - Check API logs: `kubectl logs -n analytics-platform deployment/flask-api | grep "tenant"`
   - Test tenant isolation directly: test_multi_tenant_local.sh

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
- **Tenant Isolation**: Strong multi-tenant boundaries for data security
- **Container Hardening**: All containers run as non-root with minimal capabilities
- **Network Policies**: Default deny policy with explicit allow rules
- **Secrets Management**: Sensitive data stored in Kubernetes secrets
- **Security Scanning**: Automated checking with security-check.sh

## Next Steps

- **Grafana Integration**: Add Grafana dashboards connected to Prometheus
- **Tenant-Specific Dashboards**: Create per-tenant visualization dashboards
- **Processing Pipeline Enhancement**: Ensure tenant context propagation through all stages
- **Tenant Rate Limiting**: Implement tenant-specific API usage quotas
- **Audit Logging**: Add comprehensive audit logs for tenant actions
- **Scaling**: Test horizontal scaling of services under load
- **Persistence**: Implement persistent volumes for Kafka and storage layer
- **CI/CD**: Add GitHub Actions for automated testing and deployment

## License

This project is licensed under the MIT License - see the LICENSE file for details.
```

## Next Steps After Implementing Multi-Tenant Capabilities

Now that you've successfully implemented multi-tenant isolation in your platform, I recommend focusing on these next steps:

1. **Update The Processing Engine**: 
   - Ensure it respects tenant boundaries when processing Kafka messages
   - Store tenant_id with processed data

2. **Update The Storage Layer**:
   - Partition storage by tenant_id
   - Implement proper authorization checks on queries

3. **Metrics Collection**:
   - Implement the tenant metrics collection that's already defined
   - Add monitoring dashboards to track usage by tenant

4. **Rate Limiting**:
   - Implement the rate limiting that's defined but not used
   - Add tenant-specific quotas and enforcement

5. **Documentation**:
   - Create a tenant onboarding guide
   - Document API usage for tenant applications

6. **Stress Testing**:
   - Test the platform with multiple tenants
   - Verify isolation under high load conditions

The most critical next step is ensuring that the tenant context continues to flow through your entire pipeline, from API to processing engine to storage layer, maintaining the strict tenant boundaries you've established at the API level.## Next Steps After Implementing Multi-Tenant Capabilities