# Kafka KRaft Mode Deployment Guide

This guide explains how to deploy Apache Kafka in KRaft mode (without Zookeeper) on Kubernetes for the Real-Time Analytics Platform.

## Overview

Kafka KRaft mode (Kafka Raft) is a new consensus mechanism introduced in Kafka 3.x that replaces the dependency on Zookeeper. KRaft simplifies the architecture by having Kafka manage its own metadata.

## Single-Node Deployment

For development and testing environments, a single-node Kafka KRaft deployment is recommended:

```bash
# Run the fix script with default settings
./scripts/fix-kafka-kraft.sh 
```

This script will:
1. Delete any existing Kafka StatefulSet
2. Create a ConfigMap with the proper configuration
3. Create a single-node StatefulSet running in KRaft mode
4. Verify the installation

## Lessons Learned and Common Issues

During the implementation, we encountered several issues that were resolved:

1. **DNS Resolution Issues**: In KRaft mode, Kafka nodes need to resolve each other's hostnames. To avoid DNS resolution problems, we:
   - Use `localhost` for controller communication in single-node deployments
   - For multi-node deployments, use IP addresses rather than hostnames

2. **Controller Listener Configuration**: For KRaft mode, the controller listener configuration must be properly set:
   - For controllers: `controller.listener.names` must be set to "CONTROLLER"
   - For broker-only nodes: Special setup is needed (see multi-node deployment section)

3. **Process Roles**: In KRaft mode, each node must have its role clearly defined:
   - For nodes that act as both broker and controller: `process.roles=broker,controller`
   - For nodes that act only as brokers: `process.roles=broker`

4. **Quorum Voters**: The controller quorum voters must be specified correctly:
   - Format: `<node-id>@<hostname>:<port>`
   - For single-node: Use `0@localhost:9093`
   - For multi-node: Include all controller nodes in the quorum

## Multi-Node Deployment (Advanced)

Multi-node deployments require careful configuration to handle the controller election process. Currently, our platform is configured for a single-node deployment for simplicity.

If you need to scale to multiple nodes, consider these approaches:

1. **Controller-Only + Broker-Only Separation**:
   - Dedicate a set of nodes (typically 3) to be controllers
   - Configure the rest as broker-only nodes
   - Ensure proper listener configuration

2. **Combined Controller+Broker Nodes**:
   - Configure multiple nodes to serve both roles
   - Ensure proper controller quorum voters configuration

## Monitoring

Kafka KRaft exposes JMX metrics that can be scraped by Prometheus. The platform's Prometheus configuration is already set up to collect these metrics.

To view Kafka metrics:
1. Access the Grafana dashboard
2. Navigate to the "Kafka Overview" dashboard

## Troubleshooting

If you encounter issues with your Kafka KRaft deployment:

1. **Check pod logs**:
   ```bash
   kubectl logs -n analytics-platform kafka-0
   ```

2. **Verify listeners**:
   ```bash
   kubectl exec -it -n analytics-platform kafka-0 -- netstat -tulpn | grep 9092
   ```

3. **Test topic creation**:
   ```bash
   kubectl exec -it -n analytics-platform kafka-0 -- kafka-topics.sh --bootstrap-server localhost:9092 --create --topic test-topic --partitions 1 --replication-factor 1
   ```

4. **Run the fix script again**:
   ```bash
   ./scripts/fix-kafka-kraft.sh
   ```

## Production Recommendations

For production environments:

1. Use at least 3 Kafka nodes for high availability
2. Configure proper resource limits (at least 2Gi RAM, 1000m CPU)
3. Use a larger persistent storage size (50Gi+)
4. Enable authentication and encryption for Kafka
5. Implement proper monitoring and alerting

## References

- [Kafka KRaft Documentation](https://kafka.apache.org/documentation/#kraft)
- [Bitnami Kafka Docker Image](https://github.com/bitnami/containers/tree/main/bitnami/kafka)
- [Kafka on Kubernetes Best Practices](https://www.confluent.io/blog/kafka-on-kubernetes-best-practices/) 