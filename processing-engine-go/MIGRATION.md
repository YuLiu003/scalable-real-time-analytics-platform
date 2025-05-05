# Migration Guide: Python to Go Processing Engine

This document outlines the process for migrating from the Python-based processing engine to the Go implementation.

## Overview

The migration from Python to Go for the processing engine offers several advantages:

- **Performance**: Significantly improved throughput and reduced latency
- **Resource Efficiency**: Lower CPU and memory usage
- **Multi-Tenant Isolation**: Better isolation between tenant workloads
- **Type Safety**: Strong typing to catch errors at compile time
- **Maintainability**: Simplified codebase structure
- **Deployment**: Smaller container images and faster startup times

## Migration Phases

### Phase 1: Side-by-Side Validation (Current)

In this phase, both Python and Go implementations run side-by-side:

1. Deploy both Python (`processing-engine`) and Go (`processing-engine-go-new`) services
2. Run comparison scripts to validate functionality equivalence
3. Monitor performance metrics and resource usage
4. Fix any detected inconsistencies

**Current Status**: Both implementations are operational with feature parity.

### Phase 2: Traffic Shifting (Next)

Gradually shift traffic from Python to Go implementation:

1. Configure Kafka consumer groups to split processing load:
   - Python: `processing-engine` (decreasing percentage)
   - Go: `processing-engine-go-new` (increasing percentage)
2. Monitor error rates and performance during the transition
3. Validate data consistency between implementations

### Phase 3: Complete Cutover (Final)

After sufficient confidence in the Go implementation:

1. Route 100% of traffic to the Go implementation
2. Keep Python implementation running but idle (fallback)
3. After a confirmation period, decommission Python implementation

## Feature Parity Comparison

| Feature | Python Implementation | Go Implementation | Status |
|---------|----------------------|-------------------|--------|
| Kafka Consumer | ✅ | ✅ | Complete |
| Message Processing | ✅ | ✅ | Complete |
| Anomaly Detection | ✅ | ✅ | Complete |
| Multi-tenant Support | ✅ | ✅ | Complete |
| Metrics | ✅ | ✅ | Complete |
| API Endpoints | ✅ | ✅ | Complete |
| Storage Integration | ✅ | ✅ | Complete |
| Device-specific Stats | ✅ | ✅ | Complete |

## Configuration Mapping

| Python Environment Variable | Go Environment Variable | Notes |
|-----------------------------|------------------------|-------|
| `KAFKA_BROKER` | `KAFKA_BROKER` | Same format |
| `KAFKA_TOPIC` | `INPUT_TOPIC` | Renamed for clarity |
| `OUTPUT_TOPIC` | `OUTPUT_TOPIC` | Same format |
| `CONSUMER_GROUP` | `CONSUMER_GROUP` | Same format |
| `PORT` | `METRICS_PORT` | Renamed for clarity |
| `STORAGE_SERVICE_URL` | `STORAGE_SERVICE_URL` | Same format |
| `MAX_READINGS` | `MAX_READINGS` | Same format |
| `DEBUG` | `DEBUG` | Same format |

## Testing

To verify the Go implementation:

1. Run unit tests:
   ```bash
   cd processing-engine-go-new
   go test -v ./...
   ```

2. Run the comparison script:
   ```bash
   python scripts/compare-processing-engines.py
   ```

3. Manually verify API endpoints:
   - `/health` - Health check
   - `/api/stats` - Processing statistics
   - `/api/stats/device/:id` - Device-specific statistics

## Monitoring During Migration

Monitor these key metrics during migration:

1. Processing latency for both implementations
2. Message processing rates
3. Error rates
4. Memory and CPU usage
5. Kafka consumer lag for both implementations

## Rollback Plan

If issues are encountered during migration:

1. Revert Kafka consumer group configuration to route all traffic to Python implementation
2. Update services to use original endpoints
3. Investigate and fix issues in Go implementation
4. Retry migration after fixes are applied

## Timeline

- **Week 1**: Deploy Go implementation alongside Python
- **Week 2**: Validate functionality and performance
- **Week 3**: Shift 20% traffic to Go implementation
- **Week 4**: Shift 50% traffic to Go implementation
- **Week 5**: Shift 80% traffic to Go implementation
- **Week 6**: Complete cutover to Go implementation
- **Week 7**: Decommission Python implementation

## Contacts

For questions or issues during migration:

- Tech Lead: [Your Name] (your.email@company.com)
- Platform Team: platform-team@company.com 