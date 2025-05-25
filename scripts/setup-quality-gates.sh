#!/bin/bash

# Quality Gates Setup Script
# Automates the setup process for GitHub repository quality gates

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

echo "🔧 Quality Gates Setup Script"
echo "==============================="
echo

# Check if we're in the right directory
if [[ ! -f ".github/workflows/pre-merge-quality-gates.yml" ]]; then
    print_error "Quality gates workflow not found. Make sure you're in the project root directory."
    exit 1
fi

print_info "Step 1: Validating workflow files..."
if command -v yamllint &> /dev/null; then
    yamllint .github/workflows/*.yml && print_success "Workflow YAML syntax is valid"
else
    print_warning "yamllint not found. Skipping YAML validation."
fi

print_info "Step 2: Checking git status..."
if git diff --quiet && git diff --cached --quiet; then
    print_info "No uncommitted changes found."
else
    print_warning "You have uncommitted changes. The setup will commit them."
    git status --porcelain
    echo
    read -p "Continue with commit? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Setup cancelled. Please commit your changes manually."
        exit 0
    fi
fi

print_info "Step 3: Committing quality gates implementation..."
git add .
git commit -m "feat: implement comprehensive pre-merge quality gates

- Add 8-stage quality gates workflow with code quality, testing, security
- Include integration tests with build tags for data-ingestion and clean-ingestion services  
- Add OpenAPI documentation for data ingestion and clean ingestion APIs
- Configure performance baselines for development/CI environment
- Add branch protection setup script with CODEOWNERS configuration
- Include comprehensive documentation and troubleshooting guide

Quality gates include:
- Code quality & linting with golangci-lint and staticcheck
- Unit tests with 60% coverage requirement
- Security analysis with gosec, truffleHog, and custom security checks
- Docker builds with Trivy vulnerability scanning
- Integration tests with Kafka dependencies
- Performance benchmarking with regression detection
- Documentation validation for APIs and Kubernetes manifests
- Final quality gate that blocks merge on critical failures" || true

print_info "Step 4: Pushing to GitHub..."
CURRENT_BRANCH=$(git branch --show-current)
git push origin "$CURRENT_BRANCH"
print_success "Changes pushed to GitHub on branch: $CURRENT_BRANCH"

echo
print_info "🎯 Next Steps (Manual Action Required):"
echo
echo "1. 📝 Configure GitHub Secrets:"
echo "   - Go to: https://github.com/YuLiu003/real-time-analytics-platform/settings/secrets/actions"
echo "   - Add the following secrets:"
echo "     • DOCKER_USERNAME: Your Docker Hub username"
echo "     • DOCKER_PASSWORD: Your Docker Hub password/token"
echo "     • API_KEY_1: demo-key-for-testing-only"
echo "     • API_KEY_2: another-demo-key"
echo

echo "2. 🛡️ Set Up Branch Protection:"
echo "   - Get a GitHub token with 'repo' scope: https://github.com/settings/tokens"
echo "   - Run:"
echo "     export GITHUB_TOKEN=your_token_here"
echo "     ./scripts/setup-branch-protection.sh setup"
echo

echo "3. 🧪 Test the Workflow:"
echo "   - Create a feature branch:"
echo "     git checkout -b test/quality-gates"
echo "     echo '# Test change' >> README.md"
echo "     git add README.md && git commit -m 'test: verify quality gates'"
echo "     git push origin test/quality-gates"
echo "   - Create a PR on GitHub and watch the quality gates run"
echo

echo "4. 📖 Review Documentation:"
echo "   - Read: docs/QUALITY_GATES.md for detailed information"
echo "   - Check: docs/github-secrets.md for secret configuration details"
echo

print_success "Quality gates setup (local part) completed successfully!"
print_info "The workflow will be active once you complete the GitHub configuration steps above."

echo
echo "🔍 Quick Status Check:"
echo "✅ Quality gates workflow: $(ls -la .github/workflows/pre-merge-quality-gates.yml | awk '{print $9}')"
echo "✅ Linting configuration: $(ls -la .golangci.yml | awk '{print $9}')"
echo "✅ Performance baselines: $(ls -la config/performance-baselines.yaml | awk '{print $9}')"
echo "✅ Integration tests: $(find . -name "*integration_test.go" | wc -l | tr -d ' ') files"
echo "✅ API documentation: $(find docs/api -name "*.yaml" | wc -l | tr -d ' ') specs"
echo "✅ Setup scripts: $(ls -la scripts/setup-branch-protection.sh | awk '{print $9}')"
