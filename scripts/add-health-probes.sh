#!/bin/bash
# add-health-probes.sh

echo "🔒 Adding health probes to deployment files..."

for deployment in ./k8s/*deployment*.yaml; do
  if [ -f "$deployment" ] && ! grep -q "livenessProbe:" "$deployment"; then
    echo "Adding health probes to $deployment"
    
    # Extract the container port
    port=$(grep -A 5 "containerPort:" "$deployment" | grep -o '[0-9]\+' | head -1)
    if [ -z "$port" ]; then
      port=5000  # Default port if not found
    fi
    
    # Create a temporary file
    tmp_file=$(mktemp)
    
    # Process the file to add health probes
    awk -v port="$port" '
    /containers:/ {
      print $0;
      print "        livenessProbe:";
      print "          httpGet:";
      print "            path: /health";
      print "            port: " port;
      print "          initialDelaySeconds: 30";
      print "          periodSeconds: 10";
      print "          timeoutSeconds: 5";
      print "          failureThreshold: 3";
      print "        readinessProbe:";
      print "          httpGet:";
      print "            path: /health";
      print "            port: " port;
      print "          initialDelaySeconds: 10";
      print "          periodSeconds: 5";
      print "          timeoutSeconds: 2";
      print "          successThreshold: 1";
      print "          failureThreshold: 3";
      next;
    }
    { print $0; }
    ' "$deployment" > "$tmp_file"
    
    # Replace the original file
    mv "$tmp_file" "$deployment"
    
    echo "  - Added probes to $deployment with port $port"
  fi
done

echo "✅ Added health probes to deployments!"
