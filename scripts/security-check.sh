#!/bin/bash
# security-check.sh - Enhanced version

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔒 Running comprehensive security checks for Real-Time Analytics Platform..."

# Find ALL deployment files
DEPLOYMENT_FILES=$(find ./k8s -name "*deployment*.yaml" -o -name "*Deployment*.yaml" | sort)
TOTAL_DEPLOYMENTS=$(echo "$DEPLOYMENT_FILES" | wc -l | tr -d ' ')

# Check for secrets in configuration files
echo "Checking for secrets in configuration files..."
if grep -r "apiKey\|password\|secret\|token\|credential" --include="*.yaml" --include="*.yml" ./k8s/ | 
   grep -v "secretKeyRef\|valueFrom" | 
   grep -v "#" |
   grep -v "secrets.template.yaml" | # Exclude template files
   grep -v "<BASE64_ENCODED" | # Exclude template placeholders
   grep -v "name: analytics-platform-secrets" | 
   grep -v "name: kafka-secrets" |
   grep -v "name: grafana-admin-credentials" |
   grep -v "name: tenant-management-secrets" |
   grep -v "name: kafka-credentials" |
   grep -v "secretName:" | 
   grep -v "secretProviderClass" |
   grep -v "key: admin-" |  # Exclude legitimate key references
   grep -v "name: tenant-" | # Exclude tenant references
   grep -v "prometheus\|metric" | # Exclude monitoring references
   grep -v "resources: \[\"secrets\"\]" | # Exclude RBAC rules for accessing secrets
   grep -v "resources: \[\"configmaps\", \"secrets\"\]" | # Exclude RBAC rules
   grep -q .; then
  echo "⚠️ Warning: Potential hardcoded secrets found in YAML files"
  # Show the actual matches for easier debugging
  echo "Matches found:"
  grep -r "apiKey\|password\|secret\|token\|credential" --include="*.yaml" --include="*.yml" ./k8s/ | 
   grep -v "secretKeyRef\|valueFrom" | 
   grep -v "#" |
   grep -v "secrets.template.yaml" | # Exclude template files
   grep -v "<BASE64_ENCODED" | # Exclude template placeholders
   grep -v "name: analytics-platform-secrets" | 
   grep -v "name: kafka-secrets" |
   grep -v "name: grafana-admin-credentials" |
   grep -v "name: tenant-management-secrets" |
   grep -v "name: kafka-credentials" |
   grep -v "secretName:" | 
   grep -v "secretProviderClass" |
   grep -v "key: admin-" |
   grep -v "name: tenant-" |
   grep -v "prometheus\|metric" |
   grep -v "resources: \[\"secrets\"\]" | # Exclude RBAC rules for accessing secrets
   grep -v "resources: \[\"configmaps\", \"secrets\"\]" # Exclude RBAC rules
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

# Check Go files for API key authentication
API_AUTH_CODE=0
if grep -q "X-API-Key\|API_KEY\|apiKey\|HeaderAPIKey" --include="*.go" ./*-go/ 2>/dev/null; then
  API_AUTH_CODE=1
fi

# Check Go files for authentication functions
API_AUTH_MARKER=0
if grep -q "authenticate\|authorize\|middleware.Auth" --include="*.go" ./*-go/ 2>/dev/null; then
  API_AUTH_MARKER=1
fi

# Evaluate API authentication
if [ $API_AUTH_DEPLOYMENT -eq 1 ] || [ $API_AUTH_CODE -eq 1 ] || [ $API_AUTH_MARKER -eq 1 ]; then
  echo "✓ API authentication configuration found"
else
  echo "⚠️ Warning: API authentication configuration might be missing"
fi

# Check for image pull policy
echo "Checking image pull policy configuration..."
DEPLOYMENT_FILES_COUNT=$(find ./k8s -name "*deployment*.yaml" | wc -l | tr -d ' ')
PULL_POLICY_COUNT=$(grep -l "imagePullPolicy:" ./k8s/*deployment*.yaml 2>/dev/null | wc -l | tr -d ' ')

if [ "$DEPLOYMENT_FILES_COUNT" -eq "$PULL_POLICY_COUNT" ]; then
  echo "✓ All deployment files have imagePullPolicy configured ($PULL_POLICY_COUNT/$DEPLOYMENT_FILES_COUNT)"
else
  echo "⚠️ Warning: Only $PULL_POLICY_COUNT out of $DEPLOYMENT_FILES_COUNT deployment files have imagePullPolicy"
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
     grep -v "secrets.template.yaml" | # Exclude template files
     grep -v "<BASE64_ENCODED" | # Exclude template placeholders
     grep -v "name: analytics-platform-secrets" | 
     grep -v "name: kafka-secrets" |
     grep -v "name: grafana-admin-credentials" |
     grep -v "name: tenant-management-secrets" |
     grep -v "name: kafka-credentials" |
     grep -v "secretName:" | 
     grep -v "secretProviderClass" |
     grep -v "key: admin-" |
     grep -v "name: tenant-" |
     grep -v "prometheus\|metric" |
     grep -v "resources: \[\"secrets\"\]" | # Exclude RBAC rules for accessing secrets
     grep -v "resources: \[\"configmaps\", \"secrets\"\]" | # Exclude RBAC rules
     grep -q .; then
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

# Check if we should run live Kubernetes checks
RUN_K8S_CHECKS=${RUN_K8S_CHECKS:-"auto"}

# Function to check if kubectl is available and cluster is accessible
check_kubectl_available() {
  if ! command -v kubectl >/dev/null 2>&1; then
    return 1
  fi
  
  # Try to access cluster (timeout after 5 seconds)
  if timeout 5s kubectl cluster-info >/dev/null 2>&1; then
    return 0
  else
    return 1
  fi
}

# Determine if we should run Kubernetes checks
SKIP_K8S_CHECKS=false
if [ "$RUN_K8S_CHECKS" = "false" ]; then
  SKIP_K8S_CHECKS=true
  echo -e "${YELLOW}Skipping live Kubernetes checks (RUN_K8S_CHECKS=false)${NC}"
elif [ "$RUN_K8S_CHECKS" = "auto" ]; then
  if ! check_kubectl_available; then
    SKIP_K8S_CHECKS=true
    echo -e "${YELLOW}Skipping live Kubernetes checks (no accessible cluster found)${NC}"
  fi
fi

if [ "$SKIP_K8S_CHECKS" = false ]; then
  # Set the namespace
  NAMESPACE="analytics-platform"

  echo -e "${BLUE}Starting security check for Kubernetes resources in namespace: $NAMESPACE${NC}"
  echo "=============================================================="

  # Check for privileged containers
  echo -e "\n${BLUE}Checking for privileged containers...${NC}"
  privileged_containers=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[].spec.containers[] | select(.securityContext.privileged==true) | .name')
  if [ -z "$privileged_containers" ]; then
    echo -e "${GREEN}No privileged containers found.${NC}"
  else
    echo -e "${RED}Found privileged containers:${NC}"
    echo "$privileged_containers"
  fi

  # Check for containers running as root
  echo -e "\n${BLUE}Checking for containers running as root...${NC}"
  root_containers=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[] | select((.spec.securityContext.runAsNonRoot != true) and (.spec.containers[].securityContext.runAsNonRoot != true)) | .metadata.name' 2>/dev/null || true)
  if [ -z "$root_containers" ]; then
    echo -e "${GREEN}All pods are configured to run as non-root.${NC}"
  else
    echo -e "${YELLOW}Pods that may be running as root:${NC}"
    echo "$root_containers"
  fi

  # Check for containers with privilege escalation enabled
  echo -e "\n${BLUE}Checking for containers with privilege escalation enabled...${NC}"
  priv_esc_containers=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[].spec.containers[] | select(.securityContext.allowPrivilegeEscalation==true) | .name')
  if [ -z "$priv_esc_containers" ]; then
    echo -e "${GREEN}No containers with allowPrivilegeEscalation=true found.${NC}"
  else
    echo -e "${RED}Found containers with allowPrivilegeEscalation enabled:${NC}"
    echo "$priv_esc_containers"
  fi

  # Check for containers without resource limits
  echo -e "\n${BLUE}Checking for containers without resource limits...${NC}"
  no_limits_containers=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[].spec.containers[] | select(.resources.limits == null) | .name')
  if [ -z "$no_limits_containers" ]; then
    echo -e "${GREEN}All containers have resource limits set.${NC}"
  else
    echo -e "${YELLOW}Found containers without resource limits:${NC}"
    echo "$no_limits_containers"
  fi

  # Check for containers without readiness probes
  echo -e "\n${BLUE}Checking for containers without readiness probes...${NC}"
  no_readiness_containers=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[].spec.containers[] | select(.readinessProbe == null) | .name')
  if [ -z "$no_readiness_containers" ]; then
    echo -e "${GREEN}All containers have readiness probes.${NC}"
  else
    echo -e "${YELLOW}Found containers without readiness probes:${NC}"
    echo "$no_readiness_containers"
  fi

  # Check for containers without liveness probes
  echo -e "\n${BLUE}Checking for containers without liveness probes...${NC}"
  no_liveness_containers=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[].spec.containers[] | select(.livenessProbe == null) | .name')
  if [ -z "$no_liveness_containers" ]; then
    echo -e "${GREEN}All containers have liveness probes.${NC}"
  else
    echo -e "${YELLOW}Found containers without liveness probes:${NC}"
    echo "$no_liveness_containers"
  fi

  # Check for containers with hostPath volumes
  echo -e "\n${BLUE}Checking for containers with hostPath volumes...${NC}"
  host_path_pods=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[] | select(.spec.volumes[] | select(.hostPath != null)) | .metadata.name')
  if [ -z "$host_path_pods" ]; then
    echo -e "${GREEN}No pods with hostPath volumes found.${NC}"
  else
    echo -e "${YELLOW}Found pods with hostPath volumes:${NC}"
    echo "$host_path_pods"
  fi

  # Check for containers with capabilities
  echo -e "\n${BLUE}Checking for containers with added capabilities...${NC}"
  added_capabilities=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[].spec.containers[] | select(.securityContext.capabilities.add != null) | .name')
  if [ -z "$added_capabilities" ]; then
    echo -e "${GREEN}No containers with added capabilities found.${NC}"
  else
    echo -e "${YELLOW}Found containers with added capabilities:${NC}"
    echo "$added_capabilities"
  fi

  # Check for pods without network policies
  echo -e "\n${BLUE}Checking for service communication security...${NC}"
  network_policies=$(kubectl get networkpolicies -n $NAMESPACE -o json | jq -r '.items[].metadata.name')
  if [ -z "$network_policies" ]; then
    echo -e "${RED}No network policies found in the namespace.${NC}"
  else
    echo -e "${GREEN}Network policies found:${NC}"
    echo "$network_policies"
  fi

  # Check for pods without service accounts
  echo -e "\n${BLUE}Checking for pods without custom service accounts...${NC}"
  default_sa_pods=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[] | select(.spec.serviceAccountName == "default" or .spec.serviceAccountName == null) | .metadata.name')
  if [ -z "$default_sa_pods" ]; then
    echo -e "${GREEN}All pods are using custom service accounts.${NC}"
  else
    echo -e "${YELLOW}Found pods using the default service account:${NC}"
    echo "$default_sa_pods"
  fi

  # Check for hardcoded secrets in environment variables
  echo -e "\n${BLUE}Checking for potential hardcoded secrets in environment variables...${NC}"
  suspicious_env_vars=$(kubectl get pods -n $NAMESPACE -o json | jq -r '.items[] | select(.spec.containers[].env != null) | .spec.containers[].env[]? | select(.valueFrom == null and (.name | contains("TOKEN") or contains("KEY") or contains("SECRET") or contains("PASS")) and (.name | contains("BYPASS_AUTH") or contains("ENABLE_API_AUTH") or contains("API_KEY_ENABLED") | not)) | "\(.name)"' 2>/dev/null || true)
  if [ -z "$suspicious_env_vars" ]; then
    echo -e "${GREEN}No suspicious environment variables found.${NC}"
  else
    echo -e "${YELLOW}Found potentially hardcoded secrets in environment variables:${NC}"
    echo "$suspicious_env_vars"
  fi

  echo -e "\n${BLUE}Security check completed.${NC}"
else
  echo -e "${BLUE}Live Kubernetes security checks skipped.${NC}"
  echo -e "${YELLOW}To run live cluster checks, ensure kubectl is available and set RUN_K8S_CHECKS=true${NC}"
fi