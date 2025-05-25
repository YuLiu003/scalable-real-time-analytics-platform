#!/bin/bash
# Script to fix Kafka KRaft issues in Minikube/Kubernetes
set -e

NAMESPACE=${1:-analytics-platform}

echo "Kafka KRaft Fix Script (Single Node)"
echo "==================================="
echo "This script will update the Kafka StatefulSet to resolve DNS resolution issues."
echo "Namespace: $NAMESPACE"

# Check if the namespace exists
if ! kubectl get namespace $NAMESPACE &> /dev/null; then
  echo "Error: Namespace $NAMESPACE does not exist."
  exit 1
fi

# Check for existing Kafka pods
if ! kubectl get pods -n $NAMESPACE -l app=kafka &> /dev/null; then
  echo "No Kafka pods found in namespace $NAMESPACE."
else
  # Delete any existing Kafka StatefulSet
  echo "Deleting existing Kafka StatefulSet..."
  kubectl delete statefulset kafka -n $NAMESPACE --cascade=foreground || true
  
  # Wait for pods to be deleted
  echo "Waiting for Kafka pods to terminate..."
  while kubectl get pods -n $NAMESPACE -l app=kafka 2>/dev/null | grep -q "kafka-"; do
    echo -n "."
    sleep 2
  done
  echo " Done!"
fi

# Create ConfigMap with the fixed script
echo "Creating Kafka ConfigMap with fixed configuration..."
cat << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: kafka-scripts
  namespace: analytics-platform
data:
  setup-kafka.sh: |
    #!/bin/bash
    set -e
    
    # For single node setup, use node ID 0
    export KAFKA_CFG_NODE_ID=0
    
    # Set up as both broker and controller
    export KAFKA_CFG_PROCESS_ROLES="broker,controller"
    export KAFKA_CFG_CONTROLLER_LISTENER_NAMES="CONTROLLER"
    export KAFKA_CFG_LISTENERS="PLAINTEXT://:9092,CONTROLLER://:9093"
    
    # Use localhost for controller communication (avoids DNS issues)
    export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS="0@localhost:9093"
    
    # Set the advertised listener using the pod's IP rather than hostname
    export KAFKA_CFG_ADVERTISED_LISTENERS="PLAINTEXT://$POD_IP:9092"
    
    echo "Starting Kafka with configuration:"
    echo "  Node ID: $KAFKA_CFG_NODE_ID"
    echo "  Process Roles: $KAFKA_CFG_PROCESS_ROLES"
    echo "  Listeners: $KAFKA_CFG_LISTENERS"
    echo "  Controller listener names: $KAFKA_CFG_CONTROLLER_LISTENER_NAMES"
    echo "  Controller quorum voters: $KAFKA_CFG_CONTROLLER_QUORUM_VOTERS"
    echo "  Advertised listeners: $KAFKA_CFG_ADVERTISED_LISTENERS"
    
    # Start Kafka
    exec /opt/bitnami/scripts/kafka/entrypoint.sh /opt/bitnami/scripts/kafka/run.sh
EOF

# Create a fixed StatefulSet
echo "Creating fixed Kafka KRaft StatefulSet..."
cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kafka
  namespace: $NAMESPACE
spec:
  serviceName: "kafka-headless"
  replicas: 1
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: kafka
  template:
    metadata:
      labels:
        app: kafka
    spec:
      terminationGracePeriodSeconds: 120
      containers:
      - name: kafka
        image: bitnami/kafka:3.5.1
        imagePullPolicy: IfNotPresent
        command:
          - bash
          - /scripts/setup-kafka.sh
        ports:
        - containerPort: 9092
          name: kafka
        - containerPort: 9093
          name: controller
        env:
        - name: ALLOW_PLAINTEXT_LISTENER
          value: "yes"
        - name: BITNAMI_DEBUG
          value: "true"
        - name: KAFKA_ENABLE_KRAFT
          value: "yes"
        - name: KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP
          value: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
        - name: KAFKA_KRAFT_CLUSTER_ID
          value: "abcdefghijklmnopqrstuv"
        - name: KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE
          value: "true"
        - name: KAFKA_HEAP_OPTS
          value: "-Xms512m -Xmx1g"
        # Set a reasonable default replication factor for minikube
        - name: KAFKA_CFG_DEFAULT_REPLICATION_FACTOR
          value: "1"
        - name: KAFKA_CFG_OFFSETS_TOPIC_REPLICATION_FACTOR
          value: "1"
        - name: KAFKA_CFG_TRANSACTION_STATE_LOG_REPLICATION_FACTOR
          value: "1"
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        resources:
          requests:
            memory: "1Gi"
            cpu: "300m"
          limits:
            memory: "2Gi"
            cpu: "600m"
        volumeMounts:
        - name: data
          mountPath: /bitnami/kafka
        - name: scripts
          mountPath: /scripts
        readinessProbe:
          tcpSocket:
            port: 9092
          initialDelaySeconds: 30
          periodSeconds: 10
          failureThreshold: 6
        livenessProbe:
          tcpSocket:
            port: 9092
          initialDelaySeconds: 60
          periodSeconds: 15
          failureThreshold: 4
      volumes:
      - name: scripts
        configMap:
          name: kafka-scripts
          defaultMode: 0755
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi  # Small size for testing
EOF

echo "Waiting for Kafka pod to start..."
kubectl wait --for=condition=Ready pod/kafka-0 -n $NAMESPACE --timeout=180s || { echo "Error: Kafka failed to start"; exit 1; }

echo "Kafka fixed! Testing the connection..."
kubectl exec -it kafka-0 -n $NAMESPACE -- kafka-topics.sh --bootstrap-server localhost:9092 --list

echo ""
echo "Creating a test topic..."
kubectl exec -it kafka-0 -n $NAMESPACE -- kafka-topics.sh --bootstrap-server localhost:9092 --create --topic test-topic-new --partitions 1 --replication-factor 1

echo ""
echo "Verifying the test topic..."
kubectl exec -it kafka-0 -n $NAMESPACE -- kafka-topics.sh --bootstrap-server localhost:9092 --describe --topic test-topic-new

echo ""
echo "Kafka KRaft has been successfully fixed and is running!"
echo ""
echo "To interact with Kafka, use:"
echo "kubectl exec -it kafka-0 -n $NAMESPACE -- kafka-topics.sh --bootstrap-server localhost:9092 --list"
echo "kubectl exec -it kafka-0 -n $NAMESPACE -- kafka-console-producer.sh --topic test-topic-new --bootstrap-server localhost:9092"
echo "kubectl exec -it kafka-0 -n $NAMESPACE -- kafka-console-consumer.sh --topic test-topic-new --from-beginning --bootstrap-server localhost:9092" 