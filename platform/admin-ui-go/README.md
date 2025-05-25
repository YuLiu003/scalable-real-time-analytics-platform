# Admin UI Go

A Go-based implementation of the Analytics Platform Admin UI, part of a hybrid approach to platform management.

## Overview

This project provides a modernized Admin UI for the Analytics Platform, built with Go using the Gin web framework. It works in conjunction with the existing Python-based Tenant Management service, acting as a frontend and proxy for tenant management operations.

### Hybrid Architecture

This project represents a step in migrating the platform to Go while maintaining compatibility with existing Python services:

- **Admin UI (Go)**: This service, built with Go and Gin, provides an improved frontend for platform administration.
- **Tenant Management (Go)**: The Go-based tenant management service handles backend functionality, business logic, and data persistence.

## Features

- Modern responsive web UI for platform administration
- Real-time system status monitoring
- Tenant management operations (create, list, delete)
- API key management for tenants
- Proxy requests to the Go tenant management service

## Technologies

- Go 1.22+
- Gin web framework
- Bootstrap 5 for UI
- Prometheus metrics integration
- Docker and Kubernetes deployment

## Getting Started

### Prerequisites

- Go 1.22 or higher
- Docker (for containerization)
- Kubernetes (for deployment)

### Building

```bash
# Build the service
./build.sh

# Run locally
./admin-ui-go
```

### Deployment

```bash
# Deploy to Kubernetes
./deploy.sh
```

## Configuration

Configuration is handled through environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| PORT | Port to run the service on | 5050 |
| TENANT_SERVICE_URL | URL of the Go tenant management service | http://tenant-management-go-service:80 |
| DEBUG_MODE | Enable debug mode | false |

## Directory Structure

```
.
├── config/          # Configuration management
├── handlers/        # HTTP request handlers
├── middleware/      # Middleware components
├── models/          # Data models
├── static/          # Static assets (CSS, JS)
│   └── js/          # JavaScript files
├── templates/       # HTML templates
├── Dockerfile       # Docker configuration
├── build.sh         # Build script
├── deploy.sh        # Deployment script
├── go.mod           # Go module definition
├── go.sum           # Go dependencies checksums
├── main.go          # Application entry point
└── README.md        # This file
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | / | Main admin interface |
| GET | /api/tenants | List all tenants |
| POST | /api/tenants | Create a new tenant |
| GET | /api/tenants/:id | Get a specific tenant |
| DELETE | /api/tenants/:id | Delete a tenant |
| GET | /api/status | Get system status |
| GET | /health | Health check endpoint |
| GET | /metrics | Prometheus metrics |

## Future Improvements

- Add authentication/authorization
- Implement more detailed system monitoring
- Add tenant configuration management
- Create detailed analytics dashboards 