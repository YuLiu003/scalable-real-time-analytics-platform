#!/bin/bash

# GitHub Branch Protection Setup Script
# This script helps configure branch protection rules for the Real-Time Analytics Platform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="${REPO_OWNER:-}"
REPO_NAME="${REPO_NAME:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
BRANCH="${BRANCH:-main}"

# GitHub API base URL
API_BASE="https://api.github.com"

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        print_error "jq is required but not installed"
        print_info "Install with: brew install jq (macOS) or apt-get install jq (Ubuntu)"
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Function to get repository information
get_repo_info() {
    if [[ -z "$REPO_OWNER" ]] || [[ -z "$REPO_NAME" ]]; then
        # Try to extract from git remote
        if git remote get-url origin &> /dev/null; then
            local remote_url=$(git remote get-url origin)
            if [[ $remote_url =~ github\.com[:/]([^/]+)/([^/]+)\.git ]]; then
                REPO_OWNER="${BASH_REMATCH[1]}"
                REPO_NAME="${BASH_REMATCH[2]}"
                print_info "Detected repository: $REPO_OWNER/$REPO_NAME"
            fi
        fi
    fi
    
    if [[ -z "$REPO_OWNER" ]] || [[ -z "$REPO_NAME" ]]; then
        print_error "Repository owner and name are required"
        echo "Please set REPO_OWNER and REPO_NAME environment variables"
        echo "Example: export REPO_OWNER=myorg && export REPO_NAME=myrepo"
        exit 1
    fi
}

# Function to check GitHub token
check_github_token() {
    if [[ -z "$GITHUB_TOKEN" ]]; then
        print_error "GitHub token is required"
        echo "Please set GITHUB_TOKEN environment variable"
        echo "Create a token at: https://github.com/settings/tokens"
        echo "Required scopes: repo, admin:repo_hook"
        exit 1
    fi
    
    # Validate token
    local response=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
        "$API_BASE/user" | jq -r '.login // empty')
    
    if [[ -z "$response" ]]; then
        print_error "Invalid GitHub token"
        exit 1
    fi
    
    print_success "GitHub token validated for user: $response"
}

# Function to create branch protection rule
create_branch_protection() {
    print_info "Creating branch protection rule for '$BRANCH' branch..."
    
    local protection_config=$(cat <<EOF
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Code Quality & Linting",
      "Unit Tests & Coverage", 
      "Security Analysis",
      "Docker Build & Scan",
      "Integration Tests",
      "Performance Tests",
      "Documentation Tests"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 2,
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": true,
    "require_last_push_approval": true
  },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "allow_fork_syncing": true
}
EOF
)
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/branch_protection_response.json \
        -X PUT \
        -H "Authorization: token $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        -H "Content-Type: application/json" \
        -d "$protection_config" \
        "$API_BASE/repos/$REPO_OWNER/$REPO_NAME/branches/$BRANCH/protection")
    
    local http_code="${response: -3}"
    
    if [[ "$http_code" == "200" ]]; then
        print_success "Branch protection rule created successfully"
        return 0
    elif [[ "$http_code" == "403" ]]; then
        print_error "Permission denied. Make sure your token has admin:repo scope"
        return 1
    elif [[ "$http_code" == "404" ]]; then
        print_error "Repository or branch not found"
        return 1
    else
        print_error "Failed to create branch protection rule (HTTP $http_code)"
        if [[ -f /tmp/branch_protection_response.json ]]; then
            cat /tmp/branch_protection_response.json | jq -r '.message // .error' 2>/dev/null || cat /tmp/branch_protection_response.json
        fi
        return 1
    fi
}

# Function to verify branch protection
verify_branch_protection() {
    print_info "Verifying branch protection rule..."
    
    local response=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
        "$API_BASE/repos/$REPO_OWNER/$REPO_NAME/branches/$BRANCH/protection")
    
    if echo "$response" | jq -e '.required_status_checks' &> /dev/null; then
        print_success "Branch protection rule is active"
        
        echo
        print_info "Protection settings:"
        echo "$response" | jq '{
            required_status_checks: .required_status_checks.contexts,
            required_reviewers: .required_pull_request_reviews.required_approving_review_count,
            dismiss_stale_reviews: .required_pull_request_reviews.dismiss_stale_reviews,
            require_code_owner_reviews: .required_pull_request_reviews.require_code_owner_reviews,
            enforce_admins: .enforce_admins
        }'
        
        return 0
    else
        print_warning "Branch protection rule not found or not properly configured"
        return 1
    fi
}

# Function to show current status
show_status() {
    print_info "Current branch protection status for $REPO_OWNER/$REPO_NAME ($BRANCH):"
    
    local response=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
        "$API_BASE/repos/$REPO_OWNER/$REPO_NAME/branches/$BRANCH/protection" 2>/dev/null)
    
    if echo "$response" | jq -e '.required_status_checks' &> /dev/null; then
        echo -e "${GREEN}✓${NC} Branch protection is enabled"
        
        local contexts=$(echo "$response" | jq -r '.required_status_checks.contexts[]?' 2>/dev/null)
        if [[ -n "$contexts" ]]; then
            echo
            print_info "Required status checks:"
            echo "$contexts" | while read -r context; do
                echo "  - $context"
            done
        fi
    else
        echo -e "${RED}✗${NC} Branch protection is not enabled"
    fi
}

# Function to create CODEOWNERS file
create_codeowners() {
    local codeowners_file=".github/CODEOWNERS"
    
    if [[ -f "$codeowners_file" ]]; then
        print_info "CODEOWNERS file already exists"
        return 0
    fi
    
    print_info "Creating CODEOWNERS file..."
    
    mkdir -p .github
    
    cat > "$codeowners_file" << 'EOF'
# Code Owners for Real-Time Analytics Platform
# https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners

# Global owners - will be requested for review on all PRs
* @YuLiu003

# Service-specific owners
/data-ingestion-go/ @YuLiu003
/clean-ingestion-go/ @YuLiu003
/processing-engine-go/ @YuLiu003
/storage-layer-go/ @YuLiu003
/visualization-go/ @YuLiu003
/tenant-management-go/ @YuLiu003

# Infrastructure and configuration
/.github/ @YuLiu003
/k8s/ @YuLiu003
/terraform/ @YuLiu003
/scripts/ @YuLiu003

# Documentation
/docs/ @YuLiu003
README.md @YuLiu003
*.md @YuLiu003

# Security-related files
/SECURITY.md @YuLiu003
/*security* @YuLiu003

# Performance and monitoring
/config/performance-baselines.yaml @YuLiu003
EOF
    
    print_success "CODEOWNERS file created at $codeowners_file"
    print_warning "Please update team names with your actual GitHub teams"
}

# Main function
main() {
    echo "=========================================="
    echo "GitHub Branch Protection Setup"
    echo "Real-Time Analytics Platform"
    echo "=========================================="
    echo
    
    check_prerequisites
    get_repo_info
    check_github_token
    
    case "${1:-setup}" in
        "setup")
            create_codeowners
            create_branch_protection
            verify_branch_protection
            ;;
        "status")
            show_status
            ;;
        "verify")
            verify_branch_protection
            ;;
        "codeowners")
            create_codeowners
            ;;
        *)
            echo "Usage: $0 [setup|status|verify|codeowners]"
            echo
            echo "Commands:"
            echo "  setup      - Create branch protection rules and CODEOWNERS (default)"
            echo "  status     - Show current branch protection status"
            echo "  verify     - Verify branch protection is working"
            echo "  codeowners - Create CODEOWNERS file only"
            exit 1
            ;;
    esac
}

# Cleanup function
cleanup() {
    rm -f /tmp/branch_protection_response.json
}

# Set trap for cleanup
trap cleanup EXIT

# Run main function
main "$@"
