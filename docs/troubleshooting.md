# Troubleshooting Guide

This guide covers common issues you might encounter with the Real-Time Analytics Platform in production and how to resolve them.

## Table of Contents

1. [General Troubleshooting Strategy](#general-troubleshooting-strategy)
2. [Data Ingestion Issues](#data-ingestion-issues)
3. [Kafka and ZooKeeper Problems](#kafka-and-zookeeper-problems)
4. [Processing Engine Failures](#processing-engine-failures)
5. [Storage Layer Issues](#storage-layer-issues)
6. [Visualization Problems](#visualization-problems)
7. [Authentication and Authorization](#authentication-and-authorization)
8. [Kubernetes Resource Issues](#kubernetes-resource-issues)
9. [Monitoring and Alerting](#monitoring-and-alerting)
10. [Common Error Messages](#common-error-messages)

## General Troubleshooting Strategy

When troubleshooting issues in the platform, follow these steps:

1. **Check logs**: Use `kubectl logs` to view container logs.
2. **Check pod status**: Use `kubectl get pods` to check pod status and `kubectl describe pod <pod-name>` for details.
3. **Check metrics**: Look at Prometheus metrics in Grafana.
4. **Check events**: Use `kubectl get events` to see cluster events.
5. **Verify configuration**: Compare against known good configurations.
6. **Check connectivity**: Verify network policies and service communication.

## Data Ingestion Issues

### API Returns 401 Unauthorized

**Symptom**: Data ingestion API returns 401 even with valid API key.

**Possible Causes**:
1. API key not properly configured in secrets
2. Secret not mounted correctly in pod
3. Incorrect API key format in request

**Resolution**:
```bash
# Check the API keys secret
kubectl get secret api-keys -n analytics-platform -o yaml

# Check environment variables in pod
kubectl exec -it <data-ingestion-pod> -n analytics-platform -- env | grep API_KEY

# Verify the request uses correct header format
# Should be: "X-API-Key: <key>"
```

### API Returns 5xx Errors

**Symptom**: Data ingestion API returns 500 or 503 errors.

**Possible Causes**:
1. Kafka connection issues
2. Pod resource constraints
3. Errors in application code

**Resolution**:
```bash
# Check pod logs
kubectl logs <data-ingestion-pod> -n analytics-platform

# Check Kafka connectivity
kubectl exec -it <data-ingestion-pod> -n analytics-platform -- nc -vz kafka 9092

# Check pod resource usage
kubectl top pod <data-ingestion-pod> -n analytics-platform
```

## Kafka and ZooKeeper Problems

### Kafka Pods Crash Looping

**Symptom**: Kafka pods repeatedly crash and restart.

**Possible Causes**:
1. ZooKeeper connection issues
2. Insufficient resources
3. Persistent volume problems

**Resolution**:
```bash
# Check pod logs
kubectl logs <kafka-pod> -n analytics-platform

# Check ZooKeeper connectivity
kubectl exec -it <kafka-pod> -n analytics-platform -- nc -vz zookeeper 2181

# Check PVC status
kubectl get pvc -n analytics-platform
```

### Kafka Consumer Lag Growing

**Symptom**: Metrics show increasing consumer lag.

**Possible Causes**:
1. Insufficient processing resources
2. Consumer errors
3. Message processing bottlenecks

**Resolution**:
```bash
# Check consumer group status
kubectl exec -it <kafka-pod> -n analytics-platform -- \
  kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group <consumer-group>

# Scale processor deployments
kubectl scale deployment processing-engine-go -n analytics-platform --replicas=3

# Check processor logs for errors
kubectl logs <processing-pod> -n analytics-platform
```

## Processing Engine Failures

### Messages Not Being Processed

**Symptom**: Data appears in Kafka but not being processed.

**Possible Causes**:
1. Consumer configuration issues
2. Processing errors
3. Output topic issues

**Resolution**:
```bash
# Check processor logs
kubectl logs <processing-pod> -n analytics-platform

# Check consumer group offset
kubectl exec -it <kafka-pod> -n analytics-platform -- \
  kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group processing-engine

# Restart processing engine
kubectl rollout restart deployment processing-engine-go -n analytics-platform
```

### High Error Rate in Processing

**Symptom**: Metrics show high error rate in message processing.

**Possible Causes**:
1. Data format issues
2. Processing logic errors
3. Resource constraints

**Resolution**:
```bash
# Check error logs
kubectl logs <processing-pod> -n analytics-platform | grep ERROR

# Check resource usage
kubectl top pod <processing-pod> -n analytics-platform

# Verify sample messages format
kubectl exec -it <kafka-pod> -n analytics-platform -- \
  kafka-console-consumer.sh --bootstrap-server localhost:9092 \
  --topic sensor-data --from-beginning --max-messages 5
```

## Storage Layer Issues

### Storage Service Not Saving Data

**Symptom**: Processed data not appearing in storage.

**Possible Causes**:
1. Database connection issues
2. Transaction errors
3. Authorization problems

**Resolution**:
```bash
# Check storage service logs
kubectl logs <storage-pod> -n analytics-platform

# Check database connectivity
kubectl exec -it <storage-pod> -n analytics-platform -- \
  nc -vz <db-host> <db-port>

# Check database permissions
kubectl exec -it <storage-pod> -n analytics-platform -- \
  env | grep DB_
```

### Slow Query Performance

**Symptom**: Visualization dashboards or API queries are slow.

**Possible Causes**:
1. Missing indexes
2. Query optimization issues
3. Insufficient database resources

**Resolution**:
```bash
# Check query logs
kubectl logs <storage-pod> -n analytics-platform | grep "query time"

# Check resource usage
kubectl top pod <storage-pod> -n analytics-platform

# Consider scaling storage resources
kubectl scale deployment storage-layer-go -n analytics-platform --replicas=3
```

## Visualization Problems

### Dashboards Show No Data

**Symptom**: Grafana dashboards show no data.

**Possible Causes**:
1. Prometheus connection issues
2. Query issues
3. Metric collection problems

**Resolution**:
```bash
# Check Grafana logs
kubectl logs <grafana-pod> -n analytics-platform

# Verify Prometheus data source
kubectl port-forward svc/prometheus -n analytics-platform 9090:9090
# Then open http://localhost:9090 in browser and check status

# Check if metrics are being collected
curl -s http://prometheus:9090/api/v1/targets | grep state
```

### Tenant Dashboards Not Loading

**Symptom**: Tenant-specific dashboards fail to load.

**Possible Causes**:
1. Authorization issues
2. Tenant data isolation problems
3. Dashboard configuration errors

**Resolution**:
```bash
# Check for errors in Grafana logs
kubectl logs <grafana-pod> -n analytics-platform | grep ERROR

# Verify tenant data exists
kubectl exec -it <storage-pod> -n analytics-platform -- \
  curl -s "localhost:5002/api/tenant/<tenant-id>/status"

# Check dashboard provisioning
kubectl describe configmap grafana-tenant-dashboards -n analytics-platform
```

## Authentication and Authorization

### API Key Authentication Failures

**Symptom**: Valid API keys being rejected.

**Possible Causes**:
1. Secret rotation issues
2. Cache inconsistencies
3. Configuration errors

**Resolution**:
```bash
# Check auth logs
kubectl logs <data-ingestion-pod> -n analytics-platform | grep "auth"

# Verify secrets are correctly mounted
kubectl exec -it <data-ingestion-pod> -n analytics-platform -- \
  ls -la /path/to/secrets

# Restart authentication cache if applicable
kubectl rollout restart deployment data-ingestion-go -n analytics-platform
```

### RBAC Issues

**Symptom**: Services cannot access required resources.

**Possible Causes**:
1. Missing role bindings
2. Incorrect service accounts
3. Network policy restrictions

**Resolution**:
```bash
# Check service account permissions
kubectl auth can-i get pods \
  --as=system:serviceaccount:analytics-platform:data-ingestion-sa \
  -n analytics-platform

# Verify role bindings
kubectl get rolebindings -n analytics-platform

# Check for network policy restrictions
kubectl describe networkpolicy -n analytics-platform
```

## Kubernetes Resource Issues

### Pod Evictions

**Symptom**: Pods being evicted.

**Possible Causes**:
1. Node resource pressure
2. Pod resource limits too low
3. Node failures

**Resolution**:
```bash
# Check node status
kubectl describe node <node-name>

# Check pod resource requests/limits
kubectl describe pod <pod-name> -n analytics-platform | grep -A 5 Limits

# Adjust resources if needed
kubectl edit deployment <deployment-name> -n analytics-platform
```

### PersistentVolumeClaim Issues

**Symptom**: PVCs stuck in pending state.

**Possible Causes**:
1. Storage class issues
2. Storage provider problems
3. Resource quota exceeded

**Resolution**:
```bash
# Check PVC status
kubectl describe pvc <pvc-name> -n analytics-platform

# Check storage class
kubectl get storageclass

# Check events for storage issues
kubectl get events -n analytics-platform | grep PVC
```

## Monitoring and Alerting

### Missing Prometheus Metrics

**Symptom**: Expected metrics not showing in Prometheus.

**Possible Causes**:
1. Service annotation issues
2. Network policy restrictions
3. Target configuration problems

**Resolution**:
```bash
# Check Prometheus targets
kubectl port-forward svc/prometheus -n analytics-platform 9090:9090
# Then visit http://localhost:9090/targets

# Verify pod annotations
kubectl get pod <pod-name> -n analytics-platform -o yaml | grep prometheus

# Check if metrics endpoint is accessible
kubectl exec -it <prometheus-pod> -n analytics-platform -- \
  curl http://<target-service>:<port>/metrics
```

### Alert Manager Not Sending Alerts

**Symptom**: Alerts triggered but notifications not sent.

**Possible Causes**:
1. Alertmanager configuration issues
2. SMTP/webhook connectivity problems
3. Incorrect routing rules

**Resolution**:
```bash
# Check Alertmanager configuration
kubectl get configmap alertmanager-config -n analytics-platform -o yaml

# Verify Alertmanager is running
kubectl get pods -l app=alertmanager -n analytics-platform

# Check Alertmanager logs
kubectl logs <alertmanager-pod> -n analytics-platform
```

## Common Error Messages

Here are some common error messages and their resolutions:

### "dial tcp: lookup kafka on 10.96.0.10:53: no such host"

**Resolution**:
```bash
# Check if Kafka service exists
kubectl get svc kafka -n analytics-platform

# Check DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kafka.analytics-platform.svc.cluster.local

# Create or fix Kafka service if needed
kubectl apply -f k8s/kafka-service.yaml
```

### "could not find any JVMs matching version"

**Resolution**:
```bash
# Check Java version in Kafka/ZooKeeper containers
kubectl exec -it <kafka-pod> -n analytics-platform -- java -version

# Ensure correct container image is used
kubectl set image statefulset/kafka kafka=bitnami/kafka:3.3.1 -n analytics-platform
```

### "error connecting to the metrics service"

**Resolution**:
```bash
# Check Prometheus configuration
kubectl get configmap prometheus-config -n analytics-platform -o yaml

# Verify Prometheus can reach services
kubectl exec -it <prometheus-pod> -n analytics-platform -- \
  curl -s http://<service-name>:<port>/metrics | head

# Update network policies if needed
kubectl apply -f k8s/network-policy.yaml
``` 