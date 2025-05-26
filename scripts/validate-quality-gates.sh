#!/bin/bash

# Quality Gates Validation Script
# This script validates all quality gates locally before CI/CD

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Starting Quality Gates Validation${NC}"
echo "======================================="

# Function to print status
print_status() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✅ $2${NC}"
    else
        echo -e "${RED}❌ $2${NC}"
        return 1
    fi
}

# Function to print warning
print_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

# Function to print step
print_step() {
    echo -e "${BLUE}🔧 $1${NC}"
}

# Variables
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# List of Go services to test
SERVICES=(
    "data-ingestion-go"
    "clean-ingestion-go" 
    "processing-engine-go"
    "storage-layer-go"
    "visualization-go"
    "tenant-management-go"
)

print_step "1. Checking Go services existence"
for service in "${SERVICES[@]}"; do
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if [ -d "$service" ] && [ -f "$service/go.mod" ]; then
        print_status 0 "Service $service exists"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        print_warning "Service $service not found or missing go.mod"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
done

print_step "2. Running Go module validation"
for service in "${SERVICES[@]}"; do
    if [ -d "$service" ] && [ -f "$service/go.mod" ]; then
        echo "Testing $service..."
        cd "$service"
        
        # Test go mod tidy
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if go mod tidy; then
            print_status 0 "go mod tidy - $service"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            print_status 1 "go mod tidy - $service"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        
        # Test go fmt
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if go fmt ./... >/dev/null 2>&1; then
            print_status 0 "go fmt - $service"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            print_status 1 "go fmt - $service"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        
        # Test go vet
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if go vet ./... >/dev/null 2>&1; then
            print_status 0 "go vet - $service"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            print_status 1 "go vet - $service"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        
        # Test go build
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if go build ./... >/dev/null 2>&1; then
            print_status 0 "go build - $service"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            print_status 1 "go build - $service"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        
        # Test go test
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if go test ./... >/dev/null 2>&1; then
            print_status 0 "go test - $service"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            print_status 1 "go test - $service"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        
        cd ..
    fi
done

print_step "3. Running security checks"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ -f "scripts/security-check.sh" ]; then
    chmod +x scripts/security-check.sh
    if ./scripts/security-check.sh >/dev/null 2>&1; then
        print_status 0 "Security checks"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        print_status 1 "Security checks"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
else
    print_warning "Security check script not found"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

print_step "4. Running vulnerability scans"
if command -v govulncheck >/dev/null 2>&1; then
    for service in "${SERVICES[@]}"; do
        if [ -d "$service" ] && [ -f "$service/go.mod" ]; then
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            cd "$service"
            if govulncheck ./... >/dev/null 2>&1; then
                print_status 0 "govulncheck - $service"
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                print_status 1 "govulncheck - $service (vulnerabilities found)"
                FAILED_TESTS=$((FAILED_TESTS + 1))
            fi
            cd ..
        fi
    done
else
    print_warning "govulncheck not installed"
fi

print_step "5. Checking GitHub Actions workflows"
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ -f ".github/workflows/ci.yml" ]; then
    print_status 0 "CI workflow exists"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    print_status 1 "CI workflow missing"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ -f ".github/workflows/pre-merge-quality-gates.yml" ]; then
    print_status 0 "Pre-merge quality gates workflow exists"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    print_status 1 "Pre-merge quality gates workflow missing"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

TOTAL_TESTS=$((TOTAL_TESTS + 1))
if [ -f ".golangci.yml" ]; then
    print_status 0 "golangci-lint configuration exists"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    print_status 1 "golangci-lint configuration missing"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

# Final summary
echo ""
echo "======================================="
echo -e "${BLUE}📊 Quality Gates Validation Summary${NC}"
echo "======================================="
echo "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}🎉 All quality gates passed!${NC}"
    echo "Your code is ready for CI/CD pipeline."
    exit 0
else
    echo -e "${RED}🚨 Some quality gates failed!${NC}"
    echo "Please fix the issues before pushing to CI/CD."
    exit 1
fi
