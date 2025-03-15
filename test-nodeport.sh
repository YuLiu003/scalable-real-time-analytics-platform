#!/bin/bash
set -e

MINIKUBE_IP=$(minikube ip)

echo "🔍 Testing NodePort services..."
echo "1. Checking if NodePorts are accessible directly from within the cluster..."

# Create a debug pod
kubectl run -i --tty --rm debug --image=nicolaka/netshoot --restart=Never -n analytics-platform -- bash -c "
echo 'Testing visualization service...'
curl -v visualization-service/health
echo
echo 'Testing visualization service on NodePort...'
curl -v $MINIKUBE_IP:30081/health
echo
echo 'Testing data-ingestion service...'
curl -v data-ingestion-service/health
echo
echo 'Testing data-ingestion service on NodePort...'
curl -v $MINIKUBE_IP:30080/health
echo
"

echo "🔄 Checking if the minikube VM is properly exposing the NodePorts..."
minikube ssh "netstat -tuln | grep -E '30080|30081'"

echo "🛠️ Recreating NodePort services to ensure proper configuration..."

# Delete and recreate the services as NodePort
kubectl delete service visualization-service data-ingestion-service -n analytics-platform

# Create the services as NodePort
cat > k8s/services-nodeport.yaml << SERVICE
apiVersion: v1
kind: Service
metadata:
  name: visualization-service
  namespace: analytics-platform
spec:
  selector:
    app: visualization
  ports:
  - port: 80
    targetPort: 5003
    nodePort: 30081
  type: NodePort
---
apiVersion: v1
kind: Service
metadata:
  name: data-ingestion-service
  namespace: analytics-platform
spec:
  selector:
    app: data-ingestion
  ports:
  - port: 80
    targetPort: 5000
    nodePort: 30080
  type: NodePort
SERVICE

kubectl apply -f k8s/services-nodeport.yaml

echo "✅ NodePort services recreated"
echo "📝 Verify NodePort services:"
kubectl get services -n analytics-platform

echo "🔄 Restarting minikube tunnel to ensure connectivity..."
minikube tunnel --cleanup || true
minikube tunnel &

echo "⏳ Waiting for services to be fully ready..."
sleep 5

echo "🌐 Testing NodePort services from host machine..."
curl -s http://$MINIKUBE_IP:30081/health || echo "Failed to connect to visualization"
curl -s http://$MINIKUBE_IP:30080/health || echo "Failed to connect to data-ingestion"

echo "🔄 If tests failed, verify that no firewall is blocking ports 30080 and 30081"
