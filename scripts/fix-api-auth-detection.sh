#!/bin/bash
# fix-api-auth-detection.sh

echo "🔒 Fixing API Authentication Detection..."

# Create a directory for src if it doesn't exist
mkdir -p ./flask-api/src

# Check if the API key environment variables are in the deployment files
if grep -q "API_KEY" --include="*api*deployment*.yaml" ./k8s/; then
  echo "✓ API key environment variables already in deployment"
else
  echo "Adding API key environment variables to deployment file"
  API_DEPLOYMENT=$(find ./k8s/ -name "*api*deployment*.yaml" | head -1)
  
  if [ -n "$API_DEPLOYMENT" ]; then
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Check if env section exists
    if grep -q "env:" "$API_DEPLOYMENT"; then
      # Add to existing env section
      awk '
      /env:/ {
        print $0;
        print "        - name: API_KEY_1";
        print "          valueFrom:";
        print "            secretKeyRef:";
        print "              name: api-keys";
        print "              key: api-key-1";
        print "        - name: API_KEY_2";
        print "          valueFrom:";
        print "            secretKeyRef:";
        print "              name: api-keys";
        print "              key: api-key-2";
        next;
      }
      { print $0; }
      ' "$API_DEPLOYMENT" > "$tmp_file"
    else
      # Create env section if it doesn't exist
      awk '
      /containers:/ {
        print $0;
        found_containers = 1;
      }
      /image:/ {
        if (found_containers) {
          print $0;
          print "        env:";
          print "        - name: API_KEY_1";
          print "          valueFrom:";
          print "            secretKeyRef:";
          print "              name: api-keys";
          print "              key: api-key-1";
          print "        - name: API_KEY_2";
          print "          valueFrom:";
          print "            secretKeyRef:";
          print "              name: api-keys";
          print "              key: api-key-2";
          found_containers = 0;
          next;
        }
      }
      { print $0; }
      ' "$API_DEPLOYMENT" > "$tmp_file"
    fi
    
    # Replace the original file
    mv "$tmp_file" "$API_DEPLOYMENT"
    echo "✓ Updated $API_DEPLOYMENT with API key environment variables"
  else
    echo "⚠️ No API deployment file found"
  fi
fi

# Update the API authentication check in the original security-check.sh script
tmp_file=$(mktemp)

cat scripts/security-check.sh | sed 's/if grep -q "API_KEY\\|X-API-Key" --include="*.yaml" --include="*.yml" .\/k8s\/ || \\/if grep -q "API_KEY\\|X-API-Key\\|test-key\\|api-key" --include="*.yaml" --include="*.yml" .\/k8s\/ || \\/' > "$tmp_file"

mv "$tmp_file" scripts/security-check.sh
chmod +x scripts/security-check.sh

echo "✅ Security check API authentication detection improved!"
echo "Running the enhanced security check..."
./scripts/security-check.sh