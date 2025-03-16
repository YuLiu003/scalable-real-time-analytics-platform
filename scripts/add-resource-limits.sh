#!/bin/bash
# add-resource-limits.sh

echo "🔒 Adding resource limits to deployment files..."

for deployment in ./k8s/*deployment*.yaml; do
  if [ -f "$deployment" ] && ! grep -q "resources:" "$deployment"; then
    echo "Adding resource limits to $deployment"
    
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Process the file to add resource limits
    awk '
    /containers:/ {
      print $0;
      print "        resources:";
      print "          limits:";
      print "            cpu: \"500m\"";
      print "            memory: \"512Mi\"";
      print "          requests:";
      print "            cpu: \"100m\"";
      print "            memory: \"128Mi\"";
      next;
    }
    { print $0; }
    ' "$deployment" > "$tmp_file"
    
    # Replace the original file
    mv "$tmp_file" "$deployment"
  fi
done

echo "✅ Added resource limits to deployments!"
