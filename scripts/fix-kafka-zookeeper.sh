#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Kafka and Zookeeper Troubleshooting Script${NC}"
echo -e "========================================="

# Function to check pod status
check_pod_status() {
  local namespace=$1
  local label=$2
  
  local ready_count=$(kubectl get pods -n $namespace -l app=$label -o jsonpath='{.items[?(@.status.phase=="Running")].status.containerStatuses[0].ready}' | grep -o "true" | wc -l)
  local total_count=$(kubectl get pods -n $namespace -l app=$label -o jsonpath='{.items[*].status.containerStatuses[0].name}' | wc -w)
  
  echo -e "${BLUE}$label Status:${NC} $ready_count/$total_count pods ready"
  
  if [ "$ready_count" -eq "$total_count" ] && [ "$total_count" -gt 0 ]; then
    return 0
  else
    return 1
  fi
}

# Check if Kafka and Zookeeper exist
check_service_exists() {
  local service=$1
  kubectl get statefulset -n analytics-platform $service &>/dev/null
  return $?
}

# Check for namespace
echo -e "${BLUE}Checking namespace...${NC}"
if ! kubectl get namespace analytics-platform &>/dev/null; then
  echo -e "${RED}analytics-platform namespace does not exist.${NC}"
  echo -e "${GREEN}Creating namespace...${NC}"
  kubectl create namespace analytics-platform
fi

# Option menu
while true; do
  echo
  echo -e "${BLUE}What would you like to do?${NC}"
  echo -e "1. Check status of Kafka and Zookeeper"
  echo -e "2. View logs for Zookeeper"
  echo -e "3. View logs for Kafka"
  echo -e "4. Restart Zookeeper"
  echo -e "5. Restart Kafka"
  echo -e "6. Full reset of Kafka and Zookeeper (warning: data loss)"
  echo -e "7. Create Kafka topics"
  echo -e "8. Test Kafka connectivity"
  echo -e "9. Exit"
  read -p "Enter your choice [1-9]: " choice
  
  case $choice in
    1)
      echo -e "${BLUE}Checking status of services...${NC}"
      kubectl get pods,statefulsets,pvc,svc -n analytics-platform -l 'app in (kafka, zookeeper)'
      check_pod_status "analytics-platform" "zookeeper"
      check_pod_status "analytics-platform" "kafka"
      ;;
    2)
      echo -e "${BLUE}Fetching Zookeeper logs...${NC}"
      if check_service_exists "zookeeper"; then
        kubectl logs -n analytics-platform -l app=zookeeper --tail=100
      else
        echo -e "${RED}Zookeeper not found.${NC}"
      fi
      ;;
    3)
      echo -e "${BLUE}Fetching Kafka logs...${NC}"
      if check_service_exists "kafka"; then
        kubectl logs -n analytics-platform -l app=kafka --tail=100
      else
        echo -e "${RED}Kafka not found.${NC}"
      fi
      ;;
    4)
      echo -e "${BLUE}Restarting Zookeeper...${NC}"
      if check_service_exists "zookeeper"; then
        kubectl rollout restart statefulset/zookeeper -n analytics-platform
        echo -e "${GREEN}Zookeeper restart initiated.${NC}"
      else
        echo -e "${RED}Zookeeper not found. Deploying...${NC}"
        kubectl apply -f k8s/zookeeper-statefulset.yaml
      fi
      ;;
    5)
      echo -e "${BLUE}Restarting Kafka...${NC}"
      if check_service_exists "kafka"; then
        kubectl rollout restart statefulset/kafka -n analytics-platform
        echo -e "${GREEN}Kafka restart initiated.${NC}"
      else
        echo -e "${RED}Kafka not found. Deploying...${NC}"
        kubectl apply -f k8s/kafka-statefulset.yaml
      fi
      ;;
    6)
      echo -e "${RED}WARNING: This will delete all Kafka and Zookeeper data.${NC}"
      read -p "Are you sure you want to continue? (y/n): " confirm
      if [ "$confirm" = "y" ]; then
        echo -e "${BLUE}Deleting Kafka and Zookeeper resources...${NC}"
        kubectl delete statefulset -n analytics-platform kafka zookeeper --ignore-not-found
        kubectl delete pvc -n analytics-platform data-kafka-0 data-zookeeper-0 --ignore-not-found
        kubectl delete svc -n analytics-platform kafka zookeeper --ignore-not-found
        
        echo -e "${BLUE}Waiting for resources to be deleted...${NC}"
        sleep 10
        
        echo -e "${BLUE}Redeploying Zookeeper...${NC}"
        kubectl apply -f k8s/zookeeper-statefulset.yaml
        
        echo -e "${YELLOW}Waiting for Zookeeper to start...${NC}"
        sleep 30
        
        echo -e "${BLUE}Redeploying Kafka...${NC}"
        kubectl apply -f k8s/kafka-statefulset.yaml
        
        echo -e "${GREEN}Reset complete. It may take a few minutes for the services to be ready.${NC}"
      else
        echo -e "${BLUE}Reset canceled.${NC}"
      fi
      ;;
    7)
      echo -e "${BLUE}Creating Kafka topics...${NC}"
      if check_pod_status "analytics-platform" "kafka"; then
        echo -e "${BLUE}Creating sensor-data topic...${NC}"
        kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --create --bootstrap-server kafka:9092 --replication-factor 1 --partitions 3 --topic sensor-data || true
        
        echo -e "${BLUE}Creating processed-data topic...${NC}"
        kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --create --bootstrap-server kafka:9092 --replication-factor 1 --partitions 3 --topic processed-data || true
        
        echo -e "${BLUE}Creating sensor-data-clean topic...${NC}"
        kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --create --bootstrap-server kafka:9092 --replication-factor 1 --partitions 3 --topic sensor-data-clean || true
        
        echo -e "${BLUE}Listing topics:${NC}"
        kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-topics.sh --list --bootstrap-server kafka:9092
      else
        echo -e "${RED}Kafka is not running. Please start Kafka first.${NC}"
      fi
      ;;
    8)
      echo -e "${BLUE}Testing Kafka connectivity...${NC}"
      if check_pod_status "analytics-platform" "kafka"; then
        echo -e "${BLUE}Testing producer...${NC}"
        kubectl exec -n analytics-platform -it kafka-0 -- bash -c 'echo "test message" | /opt/bitnami/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic sensor-data'
        
        echo -e "${BLUE}Testing consumer (will show 1 message if successful)...${NC}"
        kubectl exec -n analytics-platform -it kafka-0 -- /opt/bitnami/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:9092 --topic sensor-data --from-beginning --max-messages 1
        
        echo -e "${GREEN}Test complete.${NC}"
      else
        echo -e "${RED}Kafka is not running. Please start Kafka first.${NC}"
      fi
      ;;
    9)
      echo -e "${GREEN}Exiting.${NC}"
      exit 0
      ;;
    *)
      echo -e "${RED}Invalid choice. Please try again.${NC}"
      ;;
  esac
done 