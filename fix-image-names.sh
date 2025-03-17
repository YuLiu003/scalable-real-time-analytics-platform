#!/bin/bash
set -e

echo "🔧 Updating Kubernetes deployment files to use correct image names..."

# Find all deployment files
DEPLOYMENT_FILES=$(find k8s -name "*-deployment.yaml")

# Function to fix a deployment file
fix_deployment() {
  local file=$1
  local service=$(basename "$file" | sed 's/-deployment.yaml//')
  
  echo "📄 Updating $file for $service..."
  
  # Check if the file contains the prefix pattern
  if grep -q "real-time-analytics-platform-" "$file"; then
    # Replace prefixed image names with simple ones
    sed -i.bak "s|image: real-time-analytics-platform-$service:latest|image: $service:latest|g" "$file"
    echo "   - Fixed image name for $service"
    
    # Ensure imagePullPolicy is Never
    if grep -q "imagePullPolicy:" "$file"; then
      sed -i.bak "s|imagePullPolicy: IfNotPresent|imagePullPolicy: Never|g" "$file"
      echo "   - Updated imagePullPolicy to Never"
    else
      # Add imagePullPolicy if not present
      sed -i.bak "/image: $service:latest/a\\        imagePullPolicy: Never" "$file"
      echo "   - Added imagePullPolicy: Never"
    fi
    
    # Remove backup files
    rm -f "$file.bak"
  else
    echo "   - No changes needed for $file (no prefixed names found)"
  fi
}

# Process each deployment file
for file in $DEPLOYMENT_FILES; do
  fix_deployment "$file"
done

echo ""
echo "✅ All deployment files updated successfully"
echo "🚀 You can now run './deploy-platform.sh' to deploy with correct image names"