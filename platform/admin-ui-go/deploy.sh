#!/bin/bash

# Script to deploy the Admin UI Go service to Kubernetes

# Set variables
IMAGE_NAME="admin-ui-go"
IMAGE_TAG="latest"
NAMESPACE="analytics-platform"

echo "Building Docker image for $IMAGE_NAME..."
docker build -t $IMAGE_NAME:$IMAGE_TAG .

echo "Creating Kubernetes YAML files..."

# Create deployment YAML
cat > admin-ui-go-deployment.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: admin-ui-go
  namespace: $NAMESPACE
  labels:
    app: admin-ui-go
spec:
  replicas: 1
  selector:
    matchLabels:
      app: admin-ui-go
  template:
    metadata:
      labels:
        app: admin-ui-go
    spec:
      containers:
      - name: admin-ui-go
        image: $IMAGE_NAME:$IMAGE_TAG
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 5050
        env:
        - name: PORT
          value: "5050"
        - name: TENANT_SERVICE_URL
          value: "http://tenant-management-service:5010"
        - name: DEBUG_MODE
          value: "false"
        resources:
          limits:
            cpu: "0.5"
            memory: "256Mi"
          requests:
            cpu: "0.2"
            memory: "128Mi"
        securityContext:
          runAsUser: 1000
          runAsGroup: 1000
          allowPrivilegeEscalation: false
        readinessProbe:
          httpGet:
            path: /health
            port: 5050
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: 5050
          initialDelaySeconds: 15
          periodSeconds: 20
EOF

# Create service YAML
cat > admin-ui-go-service.yaml << EOF
apiVersion: v1
kind: Service
metadata:
  name: admin-ui-go-service
  namespace: $NAMESPACE
spec:
  selector:
    app: admin-ui-go
  ports:
  - port: 80
    targetPort: 5050
    protocol: TCP
  type: ClusterIP
EOF

echo "Deploying to Kubernetes..."
kubectl apply -f admin-ui-go-deployment.yaml
kubectl apply -f admin-ui-go-service.yaml

echo "Moving YAML files to k8s directory..."
mkdir -p ../../k8s
mv admin-ui-go-deployment.yaml ../../k8s/
mv admin-ui-go-service.yaml ../../k8s/

echo "Deployment complete!"
echo "Service should be available shortly."

echo "You can check the status with:"
echo "kubectl get pods -n $NAMESPACE -l app=admin-ui-go"
echo "kubectl get services -n $NAMESPACE | grep admin-ui-go" 