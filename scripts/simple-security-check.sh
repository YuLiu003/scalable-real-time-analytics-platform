#!/bin/bash
# simple-security-check.sh - A more reliable security check

echo "🔒 Running simplified security check..."

# Find all deployment files
DEPLOYMENT_FILES=$(find ./k8s -name "*deployment*.yaml" -o -name "*Deployment*.yaml")
TOTAL_DEPLOYMENTS=$(echo "$DEPLOYMENT_FILES" | wc -l | tr -d ' ')
echo "Found $TOTAL_DEPLOYMENTS deployment files"

# Initialize counters
SECURE_CONTEXT=0
RESOURCE_LIMITS=0
HEALTH_PROBES=0

# Check each file
for file in $DEPLOYMENT_FILES; do
  if grep -q "securityContext:" "$file"; then
    SECURE_CONTEXT=$((SECURE_CONTEXT + 1))
  fi
  
  if grep -q "resources:" "$file"; then
    RESOURCE_LIMITS=$((RESOURCE_LIMITS + 1))
  fi
  
  if grep -q "livenessProbe:\|readinessProbe:" "$file"; then
    HEALTH_PROBES=$((HEALTH_PROBES + 1))
  fi
done

# Print results
echo "Security Context: $SECURE_CONTEXT/$TOTAL_DEPLOYMENTS"
echo "Resource Limits: $RESOURCE_LIMITS/$TOTAL_DEPLOYMENTS"
echo "Health Probes: $HEALTH_PROBES/$TOTAL_DEPLOYMENTS"

# Check for network policy
if [ -f "./k8s/network-policy.yaml" ] && grep -q "podSelector\|namespaceSelector" "./k8s/network-policy.yaml"; then
  echo "Network Policy: ✓ Present"
  NETWORK=1
else
  echo "Network Policy: ❌ Missing"
  NETWORK=0
fi

# Check for API authentication
if grep -q "X-API-Key\|API_KEY" $(find ./flask-api -name "*.py") 2>/dev/null; then
  echo "API Authentication in Code: ✓ Present"
  API_CODE=1
else
  echo "API Authentication in Code: ❌ Missing"
  API_CODE=0
fi

# Check for API keys in deployment
if grep -q "API_KEY" $(find ./k8s -name "*api*deployment*.yaml" -o -name "*API*deployment*.yaml") 2>/dev/null; then
  echo "API Keys in Deployment: ✓ Present"
  API_DEPLOY=1
else
  echo "API Keys in Deployment: ❌ Missing"
  API_DEPLOY=0
fi

# Check for Non-Root User
if grep -q "runAsNonRoot\|runAsUser" $(find ./k8s -name "*deployment*.yaml" -o -name "*Deployment*.yaml") 2>/dev/null; then
  echo "Non-Root User: ✓ Configured"
  NON_ROOT=1
else
  echo "Non-Root User: ❌ Missing"
  NON_ROOT=0
fi

# Calculate score
SCORE=0

if [ $SECURE_CONTEXT -eq $TOTAL_DEPLOYMENTS ]; then
  SCORE=$((SCORE + 1))
  echo "✅ Security Context: Pass"
else
  echo "❌ Security Context: Fail"
fi

if [ $RESOURCE_LIMITS -eq $TOTAL_DEPLOYMENTS ]; then
  SCORE=$((SCORE + 1))
  echo "✅ Resource Limits: Pass"
else
  echo "❌ Resource Limits: Fail"
fi

if [ $HEALTH_PROBES -eq $TOTAL_DEPLOYMENTS ]; then
  SCORE=$((SCORE + 1))
  echo "✅ Health Probes: Pass"
else
  echo "❌ Health Probes: Fail"
fi

if [ $NETWORK -eq 1 ]; then
  SCORE=$((SCORE + 1))
  echo "✅ Network Policy: Pass"
else
  echo "❌ Network Policy: Fail"
fi

if [ $API_CODE -eq 1 ] || [ $API_DEPLOY -eq 1 ]; then
  SCORE=$((SCORE + 1))
  echo "✅ API Authentication: Pass"
else
  echo "❌ API Authentication: Fail"
fi

if [ $NON_ROOT -eq 1 ]; then
  SCORE=$((SCORE + 1))
  echo "✅ Non-Root User: Pass"
else
  echo "❌ Non-Root User: Fail"
fi

# Print final score
PERCENTAGE=$((SCORE * 100 / 6))
echo ""
echo "Security Score: $SCORE/6 ($PERCENTAGE%)"

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
echo "✅ Security check completed"
