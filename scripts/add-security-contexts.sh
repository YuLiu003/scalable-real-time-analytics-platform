#!/bin/bash
# add-security-contexts.sh

echo "🔒 Adding security contexts to deployment files..."

for deployment in ./k8s/*deployment*.yaml; do
  if [ -f "$deployment" ] && ! grep -q "securityContext:" "$deployment"; then
    echo "Adding security context to $deployment"
    
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Process the file to add security context
    awk '
    /containers:/ {
      print $0;
      print "        securityContext:";
      print "          runAsNonRoot: true";
      print "          runAsUser: 1000";
      print "          readOnlyRootFilesystem: true";
      print "          allowPrivilegeEscalation: false";
      print "          capabilities:";
      print "            drop:";
      print "            - ALL";
      next;
    }
    { print $0; }
    ' "$deployment" > "$tmp_file"
    
    # Replace the original file
    mv "$tmp_file" "$deployment"
  fi
done

echo "✅ Added security contexts to deployments!"
