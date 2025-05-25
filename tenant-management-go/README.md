# Tenant Management Service

The tenant management service provides multi-tenant capabilities for the analytics platform, including tenant isolation, resource management, and administrative controls.

## Features

- **Multi-tenant Architecture** - Isolated data and resources per tenant
- **Tenant Lifecycle Management** - Create, update, delete tenant configurations
- **Resource Quotas** - Per-tenant resource limits and monitoring
- **Administrative API** - Tenant management operations
- **Security Isolation** - Tenant data separation and access controls
- **Metrics & Monitoring** - Per-tenant usage metrics

## Technology Stack

- **Go + Gin** - HTTP web framework
- **SQLite** - Tenant metadata storage
- **Prometheus** - Metrics collection
- **JWT** - Tenant authentication tokens

## API Endpoints

### Tenant Management
- `POST /api/tenants` - Create new tenant
- `GET /api/tenants` - List all tenants
- `GET /api/tenants/{id}` - Get tenant details
- `PUT /api/tenants/{id}` - Update tenant configuration
- `DELETE /api/tenants/{id}` - Delete tenant

### Resource Management
- `GET /api/tenants/{id}/usage` - Get tenant resource usage
- `PUT /api/tenants/{id}/quota` - Update tenant quotas
- `GET /api/tenants/{id}/metrics` - Get tenant-specific metrics

### System Endpoints
- `GET /health` - Health check endpoint
- `GET /metrics` - Prometheus metrics
- `GET /api/status` - Service status

## Tenant Model

```json
{
    "id": "tenant-001",
    "name": "Acme Corporation",
    "status": "active",
    "config": {
        "dataRetentionDays": 90,
        "maxSensors": 1000,
        "maxApiRequestsPerHour": 10000
    },
    "quotas": {
        "storage": "10GB",
        "bandwidth": "1GB/day",
        "apiCalls": 100000
    },
    "createdAt": "2025-05-25T10:00:00Z",
    "updatedAt": "2025-05-25T10:00:00Z"
}
```

## Multi-tenant Data Flow

1. **Request Routing** - Tenant ID extracted from headers or JWT token
2. **Authorization** - Verify tenant access permissions
3. **Data Isolation** - Route data to tenant-specific storage
4. **Resource Monitoring** - Track per-tenant usage against quotas
5. **Metrics Collection** - Collect tenant-specific metrics

## Configuration

Environment variables:

- `PORT` - HTTP server port (default: 5005)
- `DB_PATH` - SQLite database for tenant metadata
- `JWT_SECRET` - Secret for JWT token verification
- `ADMIN_API_KEY` - Administrative API access key

## Database Schema

```sql
CREATE TABLE tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    config TEXT,  -- JSON configuration
    quotas TEXT,  -- JSON quotas
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tenant_usage (
    tenant_id TEXT,
    metric_name TEXT,
    metric_value REAL,
    recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
```

## Usage Examples

### Create Tenant
```bash
curl -X POST http://localhost:5005/api/tenants \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: admin-key" \
  -d '{
    "id": "tenant-001",
    "name": "Acme Corp",
    "config": {
      "dataRetentionDays": 90,
      "maxSensors": 1000
    }
  }'
```

### Check Tenant Usage
```bash
curl http://localhost:5005/api/tenants/tenant-001/usage \
  -H "X-Admin-Key: admin-key"
```

## Security

- **API Key Authentication** - Administrative operations require admin key
- **Tenant Isolation** - Data never crosses tenant boundaries
- **Resource Limits** - Automatic quota enforcement
- **Audit Logging** - All tenant operations logged

## Docker Deployment

```bash
# Build image
docker build -t tenant-management-go:latest .

# Run service
docker run -p 5005:5005 \
  -e JWT_SECRET=your-secret \
  -e ADMIN_API_KEY=admin-key \
  tenant-management-go:latest
```

## Local Development

```bash
# Install dependencies
go mod download

# Run service
JWT_SECRET=dev-secret ADMIN_API_KEY=dev-admin go run main.go

# Test endpoints
curl http://localhost:5005/health
```

## Integration

The service integrates with other platform components:

- **Data Ingestion** - Validates tenant IDs for incoming data
- **Storage Layer** - Provides tenant-specific data routing
- **Visualization** - Serves tenant-specific dashboards
- **Monitoring** - Aggregates per-tenant metrics
