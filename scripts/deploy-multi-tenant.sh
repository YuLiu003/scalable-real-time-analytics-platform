#!/bin/bash

echo "🔒 Deploying Multi-Tenant Enhancements..."

# 1. Update ConfigMap with tenant configuration
echo "Updating platform ConfigMap with tenant configuration..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: platform-config
  namespace: analytics-platform
data:
  KAFKA_BROKER: "kafka:9092"
  KAFKA_TOPIC: "sensor-data"
  DATA_INGESTION_PORT: "5000"
  PROCESSING_ENGINE_PORT: "5001"
  STORAGE_LAYER_PORT: "5002"
  VISUALIZATION_PORT: "5003"
  DB_HOST: "localhost"
  DB_PORT: "5432"
  DB_NAME: "analytics"
  DB_USER: "user"
  ENABLE_TENANT_ISOLATION: "true"
  TENANT_API_KEY_MAP: '{"api-key-tenant1":"tenant1","api-key-tenant2":"tenant2"}'
  TENANT_RATE_LIMITS: '{"tenant1":100,"tenant2":200}'
  TENANT_RETENTION_DAYS: '{"tenant1":90,"tenant2":30}'
EOF

# 2. Apply the Grafana tenant dashboards ConfigMap
echo "Creating Grafana tenant dashboards..."
kubectl apply -f k8s/grafana-tenant-dashboards.yaml

# 3. Deploy the tenant admin dashboard
echo "Deploying tenant administration dashboard..."
kubectl apply -f k8s/tenant-admin-dashboard.yaml

# 4. Update deployments to use tenant configuration
echo "Updating deployments with tenant configuration..."

# 4.1 Update Flask API deployment
kubectl patch deployment flask-api -n analytics-platform --type=strategic -p '
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "flask-api",
          "env": [
            {
              "name": "ENABLE_TENANT_ISOLATION",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "ENABLE_TENANT_ISOLATION"
                }
              }
            },
            {
              "name": "TENANT_API_KEY_MAP",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "TENANT_API_KEY_MAP"
                }
              }
            },
            {
              "name": "TENANT_RATE_LIMITS",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "TENANT_RATE_LIMITS"
                }
              }
            }
          ]
        }]
      }
    }
  }
}'

# 4.2 Update Processing Engine deployment
kubectl patch deployment processing-engine -n analytics-platform --type=strategic -p '
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "processing-engine",
          "env": [
            {
              "name": "ENABLE_TENANT_ISOLATION",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "ENABLE_TENANT_ISOLATION"
                }
              }
            },
            {
              "name": "TENANT_API_KEY_MAP",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "TENANT_API_KEY_MAP"
                }
              }
            }
          ]
        }]
      }
    }
  }
}'

# 4.3 Update Storage Layer deployment
kubectl patch deployment storage-layer -n analytics-platform --type=strategic -p '
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "storage-layer",
          "env": [
            {
              "name": "ENABLE_TENANT_ISOLATION",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "ENABLE_TENANT_ISOLATION"
                }
              }
            },
            {
              "name": "TENANT_API_KEY_MAP",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "TENANT_API_KEY_MAP"
                }
              }
            },
            {
              "name": "TENANT_RETENTION_DAYS",
              "valueFrom": {
                "configMapKeyRef": {
                  "name": "platform-config",
                  "key": "TENANT_RETENTION_DAYS"
                }
              }
            }
          ]
        }]
      }
    }
  }
}'

# 5. Restart deployments to apply changes
echo "Restarting deployments to apply changes..."
kubectl rollout restart deployment/flask-api -n analytics-platform
kubectl rollout restart deployment/processing-engine -n analytics-platform
kubectl rollout restart deployment/storage-layer -n analytics-platform
kubectl rollout restart deployment/visualization -n analytics-platform
kubectl rollout restart deployment/grafana -n analytics-platform

# 6. Wait for deployments to be ready
echo "Waiting for deployments to be ready..."
kubectl rollout status deployment/flask-api -n analytics-platform
kubectl rollout status deployment/processing-engine -n analytics-platform
kubectl rollout status deployment/storage-layer -n analytics-platform
kubectl rollout status deployment/visualization -n analytics-platform
kubectl rollout status deployment/grafana -n analytics-platform

echo "✅ Multi-tenant enhancements successfully deployed!"
echo "Use './manage.sh test-api' to verify the multi-tenant implementation."
echo "Or run 'python3 scripts/test-platform.py' for comprehensive testing."