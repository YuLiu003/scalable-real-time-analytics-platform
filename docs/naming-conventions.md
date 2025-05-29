# Naming Conventions for Real-Time Analytics Platform

## Environment Variables

**Standard: UPPERCASE_WITH_UNDERSCORES**

Example:
- `KAFKA_BROKER`
- `KAFKA_TOPIC`
- `API_KEY_1`

## Kubernetes Resources

**Standard: lowercase-with-hyphens**

Example:
- Deployments: `flask-api`, `data-ingestion`
- Services: `kafka`, `flask-api-service`

## Docker Images

**Standard: lowercase-with-hyphens:tag**

Example:
- `flask-api:latest`
- `data-ingestion:latest`

## Ports

**Standard: Port numbers should be consistent between services**

Example:
- Flask API: 5000
- Data Ingestion: 5001

## Documentation

All new components should follow these naming conventions to maintain consistency across the platform.
