# Storage Layer Service

The storage layer service provides persistent data storage using SQLite with high-performance data access patterns optimized for time-series sensor data.

## Features

- **SQLite Database** - Lightweight, serverless SQL database
- **Time-series Optimization** - Optimized for sensor data storage and retrieval
- **RESTful API** - HTTP endpoints for data operations
- **Health Monitoring** - Built-in health checks and metrics
- **Data Validation** - Input validation and error handling
- **Concurrent Access** - Thread-safe database operations

## Technology Stack

- **Go + Gin** - HTTP web framework
- **SQLite3** - Embedded SQL database
- **CGO Enabled** - For SQLite C library integration
- **Prometheus** - Metrics collection

## API Endpoints

### Data Operations
- `POST /api/data` - Store new sensor data
- `GET /api/data/recent` - Get recent sensor readings
- `GET /api/data/range` - Get data within time range
- `GET /api/data/sensor/{id}` - Get data for specific sensor

### System Endpoints
- `GET /health` - Health check endpoint
- `GET /metrics` - Prometheus metrics
- `GET /api/status` - Database status and statistics

## Database Schema

```sql
CREATE TABLE sensor_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sensor_id TEXT NOT NULL,
    value REAL NOT NULL,
    timestamp DATETIME NOT NULL,
    tenant_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sensor_timestamp ON sensor_data(sensor_id, timestamp);
CREATE INDEX idx_tenant_timestamp ON sensor_data(tenant_id, timestamp);
```

## Configuration

Environment variables:

- `PORT` - HTTP server port (default: 5004)
- `DB_PATH` - SQLite database file path (default: /data/analytics.db)
- `MAX_CONNECTIONS` - Maximum database connections

## Data Format

### Input (POST /api/data)
```json
{
    "sensorId": "sensor-001",
    "value": 23.5,
    "timestamp": "2025-05-25T10:00:00Z",
    "tenantId": "tenant-1"
}
```

### Output (GET endpoints)
```json
{
    "data": [
        {
            "id": 1,
            "sensorId": "sensor-001",
            "value": 23.5,
            "timestamp": "2025-05-25T10:00:00Z",
            "tenantId": "tenant-1"
        }
    ],
    "count": 1
}
```

## Docker Deployment

```bash
# Build image
docker build -t storage-layer-go:latest .

# Run with persistent volume
docker run -p 5004:5004 -v $(pwd)/data:/data storage-layer-go:latest
```

## Local Development

```bash
# Install SQLite development headers (if needed)
# macOS: brew install sqlite3
# Ubuntu: apt-get install libsqlite3-dev

# Build with CGO
CGO_ENABLED=1 go build -o storage-layer

# Run service
./storage-layer
```

## Performance

- **Write Performance** - Optimized for high-frequency sensor data insertion
- **Read Performance** - Indexed queries for fast time-range retrievals
- **Storage Efficiency** - Compact SQLite storage format
- **Concurrent Safety** - Multiple goroutines can safely access the database

## Backup & Recovery

The SQLite database file can be backed up using:

```bash
# Backup
sqlite3 /data/analytics.db ".backup backup.db"

# Restore
cp backup.db /data/analytics.db
```
