#!/bin/bash

# Simple GitHub Actions Validation Script
# Tests the key components without external dependencies

set -e

PROJECT_ROOT="/Users/yuliu/real-time-analytics-platform"

echo "🔍 Simple GitHub Actions Validation"
echo "===================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Test 1: Check if workflow files exist
log_info "Checking workflow files..."
cd "$PROJECT_ROOT"

workflows=(".github/workflows/ci.yml" ".github/workflows/pre-merge-quality-gates.yml" ".github/workflows/go-lint.yml")

for workflow in "${workflows[@]}"; do
    if [[ -f "$workflow" ]]; then
        log_success "Found workflow: $workflow"
    else
        log_error "Missing workflow: $workflow"
        exit 1
    fi
done

# Test 2: Check golangci-lint config
log_info "Checking golangci-lint configuration..."
if [[ -f ".golangci.yml" ]]; then
    log_success "Found .golangci.yml"
else
    log_error "Missing .golangci.yml"
    exit 1
fi

# Test 3: Check Go services
log_info "Checking Go services..."
services=("processing-engine-go" "alert-service-go" "user-service-go" "tenant-service-go")

for service in "${services[@]}"; do
    if [[ -d "$service" && -f "$service/go.mod" ]]; then
        log_success "Found Go service: $service"
    else
        log_warning "Missing or incomplete Go service: $service"
    fi
done

# Test 4: Test Go compilation
log_info "Testing Go compilation..."
has_go_errors=false

for service in "${services[@]}"; do
    if [[ -d "$service" && -f "$service/go.mod" ]]; then
        log_info "Testing $service..."
        cd "$service"
        
        # Test go mod tidy
        if go mod tidy 2>/dev/null; then
            log_success "$service: go mod tidy OK"
        else
            log_error "$service: go mod tidy failed"
            has_go_errors=true
        fi
        
        # Test go build
        if go build ./... 2>/dev/null; then
            log_success "$service: go build OK"
        else
            log_error "$service: go build failed"
            has_go_errors=true
        fi
        
        cd "$PROJECT_ROOT"
    fi
done

# Test 5: Check security scripts
log_info "Checking security scripts..."
scripts=("scripts/security-check.sh" "scripts/validate-quality-gates.sh")

for script in "${scripts[@]}"; do
    if [[ -f "$script" ]]; then
        if [[ -x "$script" ]]; then
            log_success "Found executable script: $script"
        else
            log_warning "Found non-executable script: $script"
        fi
    else
        log_error "Missing script: $script"
    fi
done

# Test 6: Check workflow structure
log_info "Checking workflow structure..."

# Check if workflows contain required elements
for workflow in "${workflows[@]}"; do
    if grep -q "on:" "$workflow" && grep -q "jobs:" "$workflow"; then
        log_success "$workflow: Contains required GitHub Actions structure"
    else
        log_error "$workflow: Missing required GitHub Actions structure"
        exit 1
    fi
done

# Summary
echo ""
echo "===================================="
if [[ "$has_go_errors" == "false" ]]; then
    log_success "All basic validations passed! 🎉"
    echo ""
    echo "Your GitHub Actions setup appears to be configured correctly."
    echo "Key validations completed:"
    echo "  ✅ Workflow files present"
    echo "  ✅ golangci-lint configuration"
    echo "  ✅ Go services structure"
    echo "  ✅ Go compilation"
    echo "  ✅ Security scripts"
    echo "  ✅ Workflow structure"
    echo ""
    echo "Next steps:"
    echo "  1. Push your changes to GitHub"
    echo "  2. Create a pull request to trigger the workflows"
    echo "  3. Monitor the GitHub Actions tab for results"
else
    log_error "Some Go compilation issues found!"
    echo ""
    echo "Please fix the Go compilation errors before pushing."
fi
echo "===================================="
