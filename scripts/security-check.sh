#!/bin/bash

echo "🔒 Running security checks for Real-Time Analytics Platform..."

# Check for secrets in configuration files
echo "Checking for secrets in configuration files..."
if grep -r "apiKey\|password\|secret\|token\|credential" --include="*.yaml" --include="*.yml" ./k8s/ | grep -v "secretKeyRef\|valueFrom" | grep -v "#"; then
  echo "⚠️ Warning: Potential hardcoded secrets found in YAML files"
else
  echo "✓ No hardcoded secrets found in YAML files"
fi

# Check deployment files for security context
echo "Checking deployment files for security contexts..."
if grep -l "securityContext" ./k8s/*deployment*.yaml > /dev/null 2>&1; then
  echo "✓ Security contexts found in deployment files"
else
  echo "⚠️ Warning: Some deployment files may not have security contexts"
fi

# Check for network policies
echo "Checking for network policies..."
if [ -f "./k8s/network-policy.yaml" ]; then
  echo "✓ Network policy found"
else
  echo "⚠️ Warning: Network policy file not found"
fi

# Check for resource limits
echo "Checking for resource limits..."
if grep -l "resources" ./k8s/*deployment*.yaml > /dev/null 2>&1; then
  echo "✓ Resource limits found in deployment files"
else
  echo "⚠️ Warning: Some deployment files may not have resource limits"
fi

# Check for health probes
echo "Checking for health probes..."
if grep -l "livenessProbe\|readinessProbe" ./k8s/*deployment*.yaml > /dev/null 2>&1; then
  echo "✓ Health probes found in deployment files"
else
  echo "⚠️ Warning: Some deployment files may not have health probes"
fi

echo "✅ Security check completed"
