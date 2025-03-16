#!/bin/bash
# fix-security-issues.sh

echo "🔒 Starting comprehensive security fix for Real-Time Analytics Platform..."

# Run individual scripts
echo "Step 1/6: Adding security contexts..."
./scripts/add-security-contexts.sh

echo "Step 2/6: Adding resource limits..."
./scripts/add-resource-limits.sh

echo "Step 3/6: Adding health probes..."
./scripts/add-health-probes.sh

echo "Step 4/6: Configuring API authentication..."
./scripts/configure-api-auth.sh

echo "Step 5/6: Adding non-root user configuration..."
./scripts/add-nonroot-config.sh

echo "Step 6/6: Creating/updating network policies..."
./scripts/create-network-policy.sh

# Run the security check again to verify improvements
echo ""
echo "🔍 Verifying security improvements..."
./scripts/security-check.sh

echo ""
echo "✅ Security enhancement process completed!"
echo "Please review the changes to ensure they are appropriate for your services."
echo "Some manual adjustments may be needed for service-specific configurations."
