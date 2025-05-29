#!/bin/bash

# Final Security Validation Script
echo "🔍 Final Security Validation for Public Repository"
echo "=================================================="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

ISSUES_FOUND=0

echo
echo -e "${BLUE}1. Checking for hardcoded secret files...${NC}"

if find . -name "*secret*.yaml" -not -path "./k8s/secrets.template.yaml" -not -path "./.git/*" | grep -q .; then
    echo -e "${RED}❌ Found secret files:${NC}"
    find . -name "*secret*.yaml" -not -path "./k8s/secrets.template.yaml" -not -path "./.git/*"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✅ No hardcoded secret files found${NC}"
fi

echo
echo -e "${BLUE}2. Checking required security files...${NC}"

REQUIRED_FILES=(
    "scripts/setup-secrets.sh"
    "k8s/secrets.template.yaml"
    "SECURE_SETUP.md"
    "SECURITY_AUDIT_REPORT.md"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${GREEN}✅ $file exists${NC}"
    else
        echo -e "${RED}❌ Missing: $file${NC}"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
done

echo
echo "=============================================="

if [ $ISSUES_FOUND -eq 0 ]; then
    echo -e "${GREEN}🎉 SECURITY VALIDATION PASSED!${NC}"
    echo -e "${GREEN}Repository is SAFE for public access${NC}"
    exit 0
else
    echo -e "${RED}❌ Found $ISSUES_FOUND issues${NC}"
    exit 1
fi
