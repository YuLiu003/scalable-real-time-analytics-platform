# GitHub Actions Local Testing Guide

## Quick Start

### Basic Validation (Recommended)
```bash
# Run complete validation
./scripts/simple-workflow-test.sh

# Expected output: All validations should pass ✅
```

### Advanced Options
```bash
# Test specific components (if using the advanced script)
./scripts/test-workflows-locally.sh syntax     # YAML syntax only
./scripts/test-workflows-locally.sh config     # golangci-lint config
./scripts/test-workflows-locally.sh go         # Go services
./scripts/test-workflows-locally.sh security   # Security scripts
```

## What Gets Validated

### ✅ Core Checks
- **Workflow Files**: CI, pre-merge quality gates, Go linting
- **Configuration**: golangci-lint settings
- **Go Services**: Build compilation and structure
- **Security Scripts**: Executable and syntax validation
- **YAML Structure**: GitHub Actions format compliance

### 🔧 Go Services Tested
- `processing-engine-go/` - Core processing engine
- `data-ingestion-go/` - Data ingestion service  
- `storage-layer-go/` - Storage management
- `visualization-go/` - Data visualization
- `tenant-management-go/` - Multi-tenant management
- `clean-ingestion-go/` - Data cleaning service
- `platform/admin-ui-go/` - Admin UI service

## Troubleshooting

### Common Issues

#### Go Build Failures
```bash
# Fix module issues
cd <service-directory>
go mod tidy
go build ./...
```

#### Missing Dependencies
```bash
# Install required tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

#### Workflow Syntax Errors
- Check YAML indentation
- Verify GitHub Actions syntax
- Use online YAML validators

### Success Indicators
- ✅ All services build successfully
- ✅ No syntax errors in workflows
- ✅ Security scripts are executable
- ✅ Configuration files are valid

## Alternative: GitHub Testing

### For Complete CI/CD Testing
1. **Push to feature branch**
   ```bash
   git checkout -b feature/my-changes
   git add .
   git commit -m "Add: feature implementation"
   git push origin feature/my-changes
   ```

2. **Create Pull Request**
   - Go to GitHub repository
   - Create PR from feature branch to main
   - Workflows will automatically trigger

3. **Monitor Results**
   - Check "Actions" tab in GitHub
   - Review workflow results
   - Fix any failures and push updates

## Docker Cleanup (If Needed)

### If Running Out of Space
```bash
# Clean up Docker resources
docker container prune -f
docker image prune -f
docker system prune -f

# Check space usage
docker system df
```

### Avoid `act` for Regular Testing
- `act` creates many containers
- Use local scripts instead
- Reserve `act` for special debugging only

## Integration with Development Workflow

### Before Committing
```bash
# 1. Run local validation
./scripts/simple-workflow-test.sh

# 2. If passed, commit changes
git add .
git commit -m "Your commit message"

# 3. Push to trigger real CI/CD
git push origin your-branch
```

### Pre-merge Checklist
- [ ] Local validation passes
- [ ] All Go services compile
- [ ] Security scripts work
- [ ] Workflows have correct syntax
- [ ] Dependencies are up to date

## Contact & Support

### Getting Help
- Check workflow logs in GitHub Actions tab
- Review `QUALITY_GATES_IMPLEMENTATION_COMPLETE.md` for details
- Use `DOCKER_CLEANUP_AND_TESTING_COMPLETE.md` for troubleshooting

### Key Files
- `.github/workflows/` - GitHub Actions workflows
- `.golangci.yml` - Go linting configuration
- `scripts/` - Validation and security scripts
