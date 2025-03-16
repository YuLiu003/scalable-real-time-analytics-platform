#!/bin/bash
# add-nonroot-config.sh

echo "🔒 Adding non-root user configuration to deployment files..."

for deployment in ./k8s/*deployment*.yaml; do
  if [ -f "$deployment" ] && ! grep -q "runAsNonRoot" "$deployment"; then
    echo "Adding non-root user configuration to $deployment"
    
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Process the file to add pod-level security context if not present
    if ! grep -q "spec:.*securityContext:" "$deployment"; then
      awk '
      /spec:/ {
        print $0;
        if (match($0, /^[ \t]+spec:$/)) {
          indent = substr($0, 1, RSTART - 1);
          print indent "  securityContext:";
          print indent "    runAsUser: 1000";
          print indent "    runAsGroup: 3000";
          print indent "    fsGroup: 2000";
        }
        next;
      }
      { print $0; }
      ' "$deployment" > "$tmp_file"
      
      # Replace the original file
      mv "$tmp_file" "$deployment"
    else
      echo "  - Pod-level security context already exists"
    fi
  fi
done

# Check if the security context is also set at the container level
for deployment in ./k8s/*deployment*.yaml; do
  if [ -f "$deployment" ] && ! grep -q "containers:.*securityContext:" "$deployment"; then
    echo "Adding container-level security context to $deployment"
    
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Process the file to add container-level security context
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

echo "✅ Added non-root user configuration to deployments!"
