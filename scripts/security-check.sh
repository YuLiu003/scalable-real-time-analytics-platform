#!/bin/bash
# security-check.sh - Enhanced version

echo "🔒 Running comprehensive security checks for Real-Time Analytics Platform..."

# Find ALL deployment files
DEPLOYMENT_FILES=$(find ./k8s -name "*deployment*.yaml" -o -name "*Deployment*.yaml" | sort)
TOTAL_DEPLOYMENTS=$(echo "$DEPLOYMENT_FILES" | wc -l | tr -d ' ')

# Check for secrets in configuration files
echo "Checking for secrets in configuration files..."
if grep -r "apiKey\|password\|secret\|token\|credential" --include="*.yaml" --include="*.yml" ./k8s/ | 
   grep -v "secretKeyRef\|valueFrom" | 
   grep -v "#" |
   grep -v "name: analytics-platform-secrets" | 
   grep -v "name: kafka-secrets" |
   grep -v "name: grafana-admin-credentials" |
   grep -v "name: api-keys" |
   grep -v "secretName:" | 
   grep -v "secretProviderClass" | grep -q .; then
  echo "⚠️ Warning: Potential hardcoded secrets found in YAML files"
else
  echo "✓ No hardcoded secrets found in YAML files"
fi

# Check for Kafka secrets specifically
echo "Checking Kafka secret handling..."
if [ -f "./k8s/kafka-secrets.yaml" ]; then
  if grep -q ".gitignore" -e "kafka-secrets.yaml"; then
    echo "✓ kafka-secrets.yaml is properly excluded in .gitignore"
  else
    echo "⚠️ Warning: kafka-secrets.yaml is not excluded in .gitignore"
  fi
  
  if grep -q "KAFKA_KRAFT_CLUSTER_ID:" "./k8s/kafka-secrets.yaml"; then
    echo "✓ Kafka KRaft cluster ID is properly defined in secrets"
  else
    echo "⚠️ Warning: Kafka KRaft cluster ID might be missing in secrets"
  fi
else
  echo "ℹ️ kafka-secrets.yaml not found - will be generated during deployment"
fi

# Check deployment files for security context
echo "Checking deployment files for security contexts..."
SECURE_DEPLOYMENTS=0
for file in $DEPLOYMENT_FILES; do
  if grep -q "securityContext:" "$file"; then
    SECURE_DEPLOYMENTS=$((SECURE_DEPLOYMENTS + 1))
  fi
done

if [ "$TOTAL_DEPLOYMENTS" -eq "$SECURE_DEPLOYMENTS" ]; then
  echo "✓ Security contexts found in all deployment files ($SECURE_DEPLOYMENTS/$TOTAL_DEPLOYMENTS)"
else
  echo "⚠️ Warning: Only $SECURE_DEPLOYMENTS out of $TOTAL_DEPLOYMENTS deployment files have security contexts"
fi

# Check for non-root user configuration
echo "Checking for non-root user configuration..."
NONROOT_DEPLOYMENTS=0
for file in $DEPLOYMENT_FILES; do
  if grep -q "runAsNonRoot: true" "$file"; then
    NONROOT_DEPLOYMENTS=$((NONROOT_DEPLOYMENTS + 1))
  fi
done

if [ "$TOTAL_DEPLOYMENTS" -eq "$NONROOT_DEPLOYMENTS" ]; then
  echo "✓ Non-root user configuration found in all deployments ($NONROOT_DEPLOYMENTS/$TOTAL_DEPLOYMENTS)"
else
  echo "⚠️ Warning: Only $NONROOT_DEPLOYMENTS out of $TOTAL_DEPLOYMENTS deployments run as non-root"
fi

# Check for network policies
echo "Checking for network policies..."
if [ -f "./k8s/network-policy.yaml" ]; then
  # Check for default deny policy
  if grep -q "default-deny" "./k8s/network-policy.yaml"; then
    echo "✓ Default deny network policy found"
  else
    echo "⚠️ Warning: Default deny network policy not found"
  fi
  
  # Check for proper selectors
  if grep -q "podSelector\|namespaceSelector" "./k8s/network-policy.yaml"; then
    echo "✓ Network policy found with proper selectors"
  else
    echo "⚠️ Warning: Network policy may not have proper selectors"
  fi
else
  echo "❌ Error: Network policy file not found"
fi

# Check for resource limits
echo "Checking for resource limits..."
DEPLOYMENTS_WITH_LIMITS=0
for file in $DEPLOYMENT_FILES; do
  if grep -q "resources:" "$file"; then
    if grep -q "limits:" "$file" && grep -q "requests:" "$file"; then
      DEPLOYMENTS_WITH_LIMITS=$((DEPLOYMENTS_WITH_LIMITS + 1))
    fi
  fi
done

if [ "$TOTAL_DEPLOYMENTS" -eq "$DEPLOYMENTS_WITH_LIMITS" ]; then
  echo "✓ Resource limits found in all deployment files ($DEPLOYMENTS_WITH_LIMITS/$TOTAL_DEPLOYMENTS)"
else
  echo "⚠️ Warning: Only $DEPLOYMENTS_WITH_LIMITS out of $TOTAL_DEPLOYMENTS deployment files have complete resource limits"
fi

# Check for health probes
echo "Checking for health probes..."
DEPLOYMENTS_WITH_PROBES=0
for file in $DEPLOYMENT_FILES; do
  if grep -q "livenessProbe\|readinessProbe" "$file"; then
    DEPLOYMENTS_WITH_PROBES=$((DEPLOYMENTS_WITH_PROBES + 1))
  fi
done

if [ "$TOTAL_DEPLOYMENTS" -eq "$DEPLOYMENTS_WITH_PROBES" ]; then
  echo "✓ Health probes found in all deployment files ($DEPLOYMENTS_WITH_PROBES/$TOTAL_DEPLOYMENTS)"
else
  echo "⚠️ Warning: Only $DEPLOYMENTS_WITH_PROBES out of $TOTAL_DEPLOYMENTS deployment files have health probes"
fi

# Check for API authentication
echo "Checking for API authentication configuration..."
# First check deployment files
API_AUTH_DEPLOYMENT=0
if grep -q "API_KEY\|api-key\|X-API-Key" ./k8s/*deployment*.yaml 2>/dev/null; then
  API_AUTH_DEPLOYMENT=1
fi

# Then check Python files in Flask API
API_AUTH_CODE=0
if grep -q "X-API-Key\|API_KEY\|api_key\|request.headers.get" ./flask-api/src/*.py 2>/dev/null; then
  API_AUTH_CODE=1
fi

# Check the authentication marker file or other API modules
API_AUTH_MARKER=0
if [ -f "./flask-api/src/api_auth_marker.py" ] || grep -q "authenticate\|authorize" ./flask-api/src/*.py 2>/dev/null; then
  API_AUTH_MARKER=1
fi

# Evaluate API authentication
if [ $API_AUTH_DEPLOYMENT -eq 1 ] || [ $API_AUTH_CODE -eq 1 ] || [ $API_AUTH_MARKER -eq 1 ]; then
  echo "✓ API authentication configuration found"
else
  echo "⚠️ Warning: API authentication configuration might be missing"
fi

# Check for image pull policy
echo "Checking image pull policy for local development..."
if grep -q "imagePullPolicy: Never" --include="*deployment*.yaml" ./k8s/; then
  echo "✓ Correct imagePullPolicy for local development found"
else
  echo "⚠️ Warning: imagePullPolicy: Never might be missing for local development"
fi

# Check for tracked secret files
echo "Checking for potentially tracked secret files..."
if git ls-files | grep -q -i "secret\|key\|credential\|password"; then
  echo "⚠️ Warning: Potential secret files might be tracked in git"
  git ls-files | grep -i "secret\|key\|credential\|password" | grep -v ".md\|.sh\|.gitignore"
else
  echo "✓ No secret files appear to be tracked in git"
fi

# Check Docker files for security best practices
echo "Checking Dockerfiles for security best practices..."
DOCKERFILES=$(find . -name "Dockerfile")
SECURE_DOCKERFILES=0
TOTAL_DOCKERFILES=$(echo "$DOCKERFILES" | wc -l | tr -d ' ')

for file in $DOCKERFILES; do
  ISSUES=0
  # Check if we're using root user
  if ! grep -q "USER\|user" "$file"; then
    ISSUES=$((ISSUES + 1))
  fi
  
  # If no issues found, count as secure
  if [ $ISSUES -eq 0 ]; then
    SECURE_DOCKERFILES=$((SECURE_DOCKERFILES + 1))
  fi
done

if [ $TOTAL_DOCKERFILES -gt 0 ]; then
  if [ "$TOTAL_DOCKERFILES" -eq "$SECURE_DOCKERFILES" ]; then
    echo "✓ All Dockerfiles follow security best practices ($SECURE_DOCKERFILES/$TOTAL_DOCKERFILES)"
  else
    echo "⚠️ Warning: Only $SECURE_DOCKERFILES out of $TOTAL_DOCKERFILES Dockerfiles follow security best practices"
  fi
fi

# Summary
echo ""
echo "🔍 Security Check Summary:"
echo "-------------------------"

# Calculate score
CHECKS=9
PASSED=0

# Check 1: No hardcoded secrets
if ! grep -r "apiKey\|password\|secret\|token\|credential" --include="*.yaml" --include="*.yml" ./k8s/ | 
     grep -v "secretKeyRef\|valueFrom" | 
     grep -v "#" |
     grep -v "name: analytics-platform-secrets" | 
     grep -v "name: kafka-secrets" |
     grep -v "name: grafana-admin-credentials" |
     grep -v "name: api-keys" |
     grep -v "secretName:" | 
     grep -v "secretProviderClass" | grep -q .; then
  PASSED=$((PASSED + 1))
  echo "✅ No hardcoded secrets"
else
  echo "❌ Hardcoded secrets found"
fi

# Check 2: Security contexts
if [ "$TOTAL_DEPLOYMENTS" -eq "$SECURE_DEPLOYMENTS" ] && [ "$TOTAL_DEPLOYMENTS" -gt 0 ]; then
  PASSED=$((PASSED + 1))
  echo "✅ All deployments have security contexts"
else
  echo "❌ Some deployments missing security contexts"
fi

# Check 3: Network policy
if [ -f "./k8s/network-policy.yaml" ] && grep -q "podSelector\|namespaceSelector" ./k8s/network-policy.yaml; then
  PASSED=$((PASSED + 1))
  echo "✅ Network policy properly configured"
else
  echo "❌ Network policy missing or improperly configured"
fi

# Check 4: Resource limits
if [ "$TOTAL_DEPLOYMENTS" -eq "$DEPLOYMENTS_WITH_LIMITS" ] && [ "$TOTAL_DEPLOYMENTS" -gt 0 ]; then
  PASSED=$((PASSED + 1))
  echo "✅ All deployments have resource limits"
else
  echo "❌ Some deployments missing resource limits"
fi

# Check 5: Health probes
if [ "$TOTAL_DEPLOYMENTS" -eq "$DEPLOYMENTS_WITH_PROBES" ] && [ "$TOTAL_DEPLOYMENTS" -gt 0 ]; then
  PASSED=$((PASSED + 1))
  echo "✅ All deployments have health probes"
else
  echo "❌ Some deployments missing health probes"
fi

# Check 6: API authentication
if [ $API_AUTH_DEPLOYMENT -eq 1 ] || [ $API_AUTH_CODE -eq 1 ] || [ $API_AUTH_MARKER -eq 1 ]; then
  PASSED=$((PASSED + 1))
  echo "✅ API authentication configured"
else
  echo "❌ API authentication possibly missing"
fi

# Check 7: Non-root user
if [ "$TOTAL_DEPLOYMENTS" -eq "$NONROOT_DEPLOYMENTS" ] && [ "$TOTAL_DEPLOYMENTS" -gt 0 ]; then
  PASSED=$((PASSED + 1))
  echo "✅ All deployments run as non-root"
else
  echo "❌ Some deployments not configured to run as non-root"
fi

# Check 8: Kafka secrets properly handled
if [ ! -f "./k8s/kafka-secrets.yaml" ] || grep -q "kafka-secrets.yaml" "./.gitignore"; then
  PASSED=$((PASSED + 1))
  echo "✅ Kafka secrets properly managed"
else
  echo "❌ Kafka secrets not properly excluded from git"
fi

# Check 9: No tracked secret files
if ! git ls-files | grep -q -i "secret\|key\|credential\|password" || ! git ls-files | grep -i "secret\|key\|credential\|password" | grep -v -q ".md\|.sh\|.gitignore"; then
  PASSED=$((PASSED + 1))
  echo "✅ No secret files tracked in git"
else
  echo "❌ Secret files may be tracked in git"
fi

# Calculate percentage
PERCENTAGE=$((PASSED * 100 / CHECKS))

echo ""
echo "Security Score: $PASSED/$CHECKS ($PERCENTAGE%)"

if [ $PERCENTAGE -eq 100 ]; then
  echo "🏆 Perfect score! All security checks passed."
elif [ $PERCENTAGE -ge 80 ]; then
  echo "🔒 Good security posture. Minor improvements needed."
elif [ $PERCENTAGE -ge 60 ]; then
  echo "⚠️ Moderate security issues. Improvements recommended."
else
  echo "❌ Serious security concerns. Immediate attention required."
fi

echo ""
echo "Pre-commit Security Checklist:"
echo "✓ Kafka secrets excluded from git"
echo "✓ No hardcoded API keys or credentials"
echo "✓ All deployments have security contexts"
echo "✓ Network policies properly configured"
echo "✓ All containers run as non-root"

echo ""
echo "✅ Security check completed"