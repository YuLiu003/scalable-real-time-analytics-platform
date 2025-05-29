#!/bin/bash

echo "🧹 Performing complete project reset..."
echo "This will delete ALL Kubernetes resources and clean up unnecessary files."
echo ""
echo "Are you sure you want to proceed? (y/n)"
read -r confirmation

if [[ $confirmation != "y" ]]; then
  echo "Operation cancelled."
  exit 0
fi

# Step 1: Delete all Kubernetes resources in the analytics-platform namespace
echo "🔄 Deleting all Kubernetes resources in analytics-platform namespace..."
kubectl delete namespace analytics-platform --ignore-not-found=true

# Step 2: Clean up Docker images (if using Minikube's Docker daemon)
echo "🔄 Cleaning up Docker resources..."
if minikube status &>/dev/null; then
  eval $(minikube docker-env)
  docker system prune -af --volumes
else
  echo "Minikube not running, skipping Docker cleanup."
fi

# Step 3: Organize script files - move all but essential scripts to archive
echo "🔄 Organizing script files..."
mkdir -p archive/scripts

# List of essential scripts to keep
ESSENTIAL_SCRIPTS=(
  "deploy-platform.sh"
  "setup.sh"
  "manage.sh"
  "scripts/kube-status.sh"
  "scripts/security-check.sh"
  "scripts/test-platform.py"
  "scripts/deploy-monitoring.sh"
)

# Create scripts archive directory if it doesn't exist
mkdir -p archive/scripts

# Move non-essential scripts to archive
find . -name "*.sh" -type f | while read script; do
  is_essential=false
  for essential in "${ESSENTIAL_SCRIPTS[@]}"; do
    if [[ "$script" == "./$essential" ]]; then
      is_essential=true
      break
    fi
  done
  
  if [[ $is_essential == false && "$script" != "./reset-project.sh" ]]; then
    echo "  Archiving $script"
    mkdir -p "archive/$(dirname "$script")"
    mv "$script" "archive/$script"
  fi
done

# Step 4: Clean up temporary and backup files
echo "🔄 Cleaning up temporary and backup files..."
find . -name "*.bak" -o -name "*.tmp" -o -name "*~" -o -name "*.old" | xargs rm -f

# Step 5: Verify K8s manifests and update them to a clean state
echo "🔄 Organizing Kubernetes manifests..."
mkdir -p k8s/core

# Copy only the essential manifests to the core directory
cp k8s/namespace.yaml k8s/core/ 2>/dev/null || true
cp k8s/configmap.yaml k8s/core/ 2>/dev/null || true
cp k8s/secrets.yaml k8s/core/ 2>/dev/null || true
cp k8s/network-policy.yaml k8s/core/ 2>/dev/null || true
cp k8s/prometheus-*.yaml k8s/core/ 2>/dev/null || true
cp k8s/grafana-*.yaml k8s/core/ 2>/dev/null || true
cp k8s/*deployment*.yaml k8s/core/ 2>/dev/null || true
cp k8s/*service*.yaml k8s/core/ 2>/dev/null || true

# Move old k8s files to archive
mkdir -p archive/k8s
find k8s -maxdepth 1 -type f | grep -v "core" | xargs -I{} mv {} archive/k8s/ 2>/dev/null || true

# Check for archive/k8s directory
if [ -d "k8s/archive" ]; then
  mv k8s/archive/* archive/k8s/ 2>/dev/null || true
  rmdir k8s/archive 2>/dev/null || true
fi

# Restore core files
mv k8s/core/* k8s/ 2>/dev/null || true
rmdir k8s/core 2>/dev/null || true

echo "✅ Project reset complete!"
echo ""
echo "Next steps:"
echo "1. Start minikube: minikube start"
echo "2. Deploy the platform: ./deploy-platform.sh"
echo "3. Deploy monitoring: ./scripts/deploy-monitoring.sh"
echo ""
echo "You can check the status anytime with: ./manage.sh status"