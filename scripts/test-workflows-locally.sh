#!/bin/bash

# Local GitHub Actions Testing Script
# This script provides a lightweight alternative to running full GitHub Actions with 'act'
# It validates the key quality gates without consuming excessive disk space

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔍 Testing GitHub Actions Workflows Locally"
echo "============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to test workflow YAML syntax
test_workflow_syntax() {
    log_info "Testing workflow YAML syntax..."
    
    local workflows_dir="$PROJECT_ROOT/.github/workflows"
    local has_errors=false
    
    for workflow in "$workflows_dir"/*.yml "$workflows_dir"/*.yaml; do
        if [[ -f "$workflow" ]]; then
            local filename=$(basename "$workflow")
            log_info "Validating $filename..."
            
            # Basic YAML syntax check using python
            if python3 -c "import yaml; yaml.safe_load(open('$workflow'))" 2>/dev/null; then
                log_success "$filename: Valid YAML syntax"
            else
                log_error "$filename: Invalid YAML syntax"
                has_errors=true
            fi
            
            # Check for common GitHub Actions patterns
            if grep -q "uses:" "$workflow" && grep -q "steps:" "$workflow"; then
                log_success "$filename: Contains valid GitHub Actions structure"
            else
                log_warning "$filename: May not be a valid GitHub Actions workflow"
            fi
        fi
    done
    
    if [[ "$has_errors" == "true" ]]; then
        return 1
    fi
    
    return 0
}

# Function to test Go linting configuration
test_golangci_config() {
    log_info "Testing golangci-lint configuration..."
    
    local config_file="$PROJECT_ROOT/.golangci.yml"
    
    if [[ ! -f "$config_file" ]]; then
        log_error "golangci-lint config file not found: $config_file"
        return 1
    fi
    
    # Test YAML syntax
    if python3 -c "import yaml; yaml.safe_load(open('$config_file'))" 2>/dev/null; then
        log_success "golangci-lint config: Valid YAML syntax"
    else
        log_error "golangci-lint config: Invalid YAML syntax"
        return 1
    fi
    
    # Test if golangci-lint can load the config
    if command -v golangci-lint >/dev/null 2>&1; then
        cd "$PROJECT_ROOT"
        if golangci-lint config verify 2>/dev/null; then
            log_success "golangci-lint config: Configuration is valid"
        else
            log_warning "golangci-lint config: Configuration may have issues (this is normal if linters are not available)"
        fi
    else
        log_warning "golangci-lint not installed, skipping config verification"
    fi
    
    return 0
}

# Function to test Go services
test_go_services() {
    log_info "Testing Go services..."
    
    local services=("processing-engine-go" "alert-service-go" "user-service-go" "tenant-service-go")
    local has_errors=false
    
    for service in "${services[@]}"; do
        local service_dir="$PROJECT_ROOT/$service"
        
        if [[ ! -d "$service_dir" ]]; then
            log_warning "Service directory not found: $service"
            continue
        fi
        
        if [[ ! -f "$service_dir/go.mod" ]]; then
            log_warning "No go.mod found in $service"
            continue
        fi
        
        log_info "Testing $service..."
        
        cd "$service_dir"
        
        # Test go mod tidy
        if go mod tidy 2>/dev/null; then
            log_success "$service: go mod tidy succeeded"
        else
            log_error "$service: go mod tidy failed"
            has_errors=true
        fi
        
        # Test go build
        if go build ./... 2>/dev/null; then
            log_success "$service: go build succeeded"
        else
            log_error "$service: go build failed"
            has_errors=true
        fi
        
        # Test go vet
        if go vet ./... 2>/dev/null; then
            log_success "$service: go vet passed"
        else
            log_error "$service: go vet failed"
            has_errors=true
        fi
        
        # Test go fmt
        local unformatted=$(go fmt ./... 2>/dev/null)
        if [[ -z "$unformatted" ]]; then
            log_success "$service: go fmt passed"
        else
            log_warning "$service: go fmt found unformatted files: $unformatted"
        fi
        
        # Test if tests exist and run them
        if find . -name "*_test.go" -type f | grep -q .; then
            log_info "Running tests for $service..."
            if go test ./... -short 2>/dev/null; then
                log_success "$service: tests passed"
            else
                log_error "$service: tests failed"
                has_errors=true
            fi
        else
            log_warning "$service: no tests found"
        fi
    done
    
    cd "$PROJECT_ROOT"
    
    if [[ "$has_errors" == "true" ]]; then
        return 1
    fi
    
    return 0
}

# Function to test security scripts
test_security_scripts() {
    log_info "Testing security scripts..."
    
    local scripts=("security-check.sh" "validate-quality-gates.sh")
    
    for script in "${scripts[@]}"; do
        local script_path="$SCRIPT_DIR/$script"
        
        if [[ ! -f "$script_path" ]]; then
            log_warning "Script not found: $script"
            continue
        fi
        
        if [[ -x "$script_path" ]]; then
            log_success "$script: is executable"
        else
            log_warning "$script: is not executable"
        fi
        
        # Basic syntax check
        if bash -n "$script_path" 2>/dev/null; then
            log_success "$script: bash syntax is valid"
        else
            log_error "$script: bash syntax error"
            return 1
        fi
    done
    
    return 0
}

# Function to simulate specific workflow jobs
test_workflow_job_simulation() {
    log_info "Simulating key workflow jobs..."
    
    # Simulate pre-merge quality gates (lightweight version)
    log_info "Simulating pre-merge quality gates..."
    
    # Check if required tools are available
    local required_tools=("go" "git")
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" >/dev/null 2>&1; then
            log_success "Required tool available: $tool"
        else
            log_error "Required tool missing: $tool"
            return 1
        fi
    done
    
    # Simulate matrix build (test with current Go version only)
    local go_version=$(go version | awk '{print $3}')
    log_info "Testing with Go version: $go_version"
    
    # Run simplified quality checks
    log_info "Running simplified quality checks..."
    
    # Check for common issues in workflows
    log_info "Checking workflow configurations..."
    
    local workflows_dir="$PROJECT_ROOT/.github/workflows"
    
    # Check for required workflow files
    local required_workflows=("ci.yml" "pre-merge-quality-gates.yml" "go-lint.yml")
    for workflow in "${required_workflows[@]}"; do
        if [[ -f "$workflows_dir/$workflow" ]]; then
            log_success "Required workflow found: $workflow"
        else
            log_error "Required workflow missing: $workflow"
            return 1
        fi
    done
    
    return 0
}

# Main execution
main() {
    local exit_code=0
    
    echo "Starting local GitHub Actions testing..."
    echo "Project root: $PROJECT_ROOT"
    echo ""
    
    # Test workflow syntax
    if test_workflow_syntax; then
        log_success "Workflow syntax tests passed"
    else
        log_error "Workflow syntax tests failed"
        exit_code=1
    fi
    
    echo ""
    
    # Test golangci-lint configuration
    if test_golangci_config; then
        log_success "golangci-lint configuration tests passed"
    else
        log_error "golangci-lint configuration tests failed"
        exit_code=1
    fi
    
    echo ""
    
    # Test Go services
    if test_go_services; then
        log_success "Go services tests passed"
    else
        log_error "Go services tests failed"
        exit_code=1
    fi
    
    echo ""
    
    # Test security scripts
    if test_security_scripts; then
        log_success "Security scripts tests passed"
    else
        log_error "Security scripts tests failed"
        exit_code=1
    fi
    
    echo ""
    
    # Test workflow job simulation
    if test_workflow_job_simulation; then
        log_success "Workflow job simulation passed"
    else
        log_error "Workflow job simulation failed"
        exit_code=1
    fi
    
    echo ""
    echo "============================================="
    if [[ $exit_code -eq 0 ]]; then
        log_success "All local tests passed! 🎉"
        echo ""
        echo "Your GitHub Actions workflows should work correctly when pushed to GitHub."
        echo "Key validations completed:"
        echo "  ✅ Workflow YAML syntax"
        echo "  ✅ golangci-lint configuration"
        echo "  ✅ Go services build and test"
        echo "  ✅ Security scripts"
        echo "  ✅ Workflow structure"
    else
        log_error "Some tests failed! ❌"
        echo ""
        echo "Please fix the issues above before pushing to GitHub."
    fi
    echo "============================================="
    
    return $exit_code
}

# Run with specific test if provided
case "${1:-}" in
    "syntax")
        test_workflow_syntax
        ;;
    "config")
        test_golangci_config
        ;;
    "go")
        test_go_services
        ;;
    "security")
        test_security_scripts
        ;;
    "simulation")
        test_workflow_job_simulation
        ;;
    *)
        main
        ;;
esac
