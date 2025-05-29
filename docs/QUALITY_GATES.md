# Pre-Merge Quality Gates

This document describes the comprehensive pre-merge testing and quality gates system implemented for the Real-Time Analytics Platform. This system ensures that all code changes meet high standards for quality, security, and reliability before being merged into the main branch.

## Overview

The quality gates system consists of 8 critical stages that every pull request must pass before being merged:

1. **Code Quality & Linting** - Static code analysis and formatting checks
2. **Unit Tests & Coverage** - Comprehensive test suite with coverage requirements
3. **Security Analysis** - Security vulnerability scanning and checks
4. **Docker Build & Scan** - Container security and build verification
5. **Integration Tests** - End-to-end testing with service dependencies
6. **Performance Tests** - Benchmark testing and performance regression detection
7. **Documentation Tests** - Documentation completeness and accuracy validation
8. **Quality Gate** - Final validation that blocks merge if critical jobs fail

## Workflow Configuration

### Triggers

The quality gates workflow is triggered on:
- Pull requests to `main` or `develop` branches
- Push events to `main` or `develop` branches
- When pull requests are marked as ready for review (not draft)

### Concurrency Control

The workflow prevents multiple runs on the same branch to avoid resource conflicts and ensure clean test environments.

## Quality Gate Stages

### 1. Code Quality & Linting

**Purpose**: Ensures code follows consistent style guidelines and best practices.

**Checks**:
- `golangci-lint` with comprehensive rule set (30+ linters)
- `staticcheck` for advanced static analysis
- `go mod tidy` verification
- `go vet` analysis

**Configuration**: Uses `.golangci.yml` with service-specific exclusions

**Success Criteria**: All linting checks must pass with zero violations

### 2. Unit Tests & Coverage

**Purpose**: Validates individual component functionality and maintains code coverage standards.

**Checks**:
- Unit tests for all Go services
- Code coverage measurement and reporting
- 60% minimum coverage threshold
- Coverage reports posted as PR comments

**Services Covered**:
- `data-ingestion-go`
- `clean-ingestion-go`
- `processing-engine-go`
- `storage-layer-go`
- `visualization-go`
- `tenant-management-go`

**Success Criteria**: 
- All tests must pass
- Coverage must be ≥ 60% for each service
- No test failures allowed

### 3. Security Analysis

**Purpose**: Identifies security vulnerabilities and ensures secure coding practices.

**Checks**:
- Custom security script validation (minimum score 7/9)
- `gosec` static security analysis with SARIF upload
- `truffleHog` secrets detection
- Security findings uploaded to GitHub Security tab

**Success Criteria**:
- Security score ≥ 7/9
- No high-severity security vulnerabilities
- No exposed secrets or credentials

### 4. Docker Build & Scan

**Purpose**: Ensures containers can be built successfully and are free from vulnerabilities.

**Checks**:
- Multi-service Docker builds using matrix strategy
- `Trivy` vulnerability scanning for all built images
- Security findings uploaded as SARIF reports
- Build artifacts properly tagged

**Success Criteria**:
- All Docker builds must succeed
- No critical container vulnerabilities
- Images must pass security scans

### 5. Integration Tests

**Purpose**: Validates service interactions and end-to-end workflows.

**Checks**:
- Kafka-based integration testing
- Service dependency validation
- Cross-service communication testing
- Integration test tags (`//go:build integration`)

**Environment**:
- Docker Compose with Kafka KRaft
- Service mesh connectivity
- Real data flow simulation

**Success Criteria**:
- All integration tests must pass
- Services must communicate correctly
- Data pipeline integrity verified

### 6. Performance Tests

**Purpose**: Prevents performance regressions and validates system scalability.

**Checks**:
- Go benchmark tests for all services
- Performance baseline comparison
- Regression detection against historical data
- Resource usage validation

**Baselines**: Defined in `config/performance-baselines.yaml`

**Success Criteria**:
- Benchmarks complete successfully
- No significant performance degradation (>10%)
- Resource usage within acceptable limits

### 7. Documentation Tests

**Purpose**: Ensures documentation is complete, accurate, and up-to-date.

**Checks**:
- README completeness validation
- Kubernetes manifest validation
- API documentation verification
- Link checking for external references

**Success Criteria**:
- All documentation checks pass
- Kubernetes manifests are valid
- API docs match implementation

### 8. Quality Gate

**Purpose**: Final validation that determines if code can be merged.

**Validation**:
- Checks status of all critical jobs
- Blocks merge if any critical stage fails
- Allows warnings for non-critical issues
- Provides detailed summary report

**Critical Jobs** (must pass):
- Unit Tests & Coverage
- Security Analysis
- Docker Build & Scan

**Warning Jobs** (can have issues):
- Documentation Tests
- Performance Tests (if within tolerance)

## Branch Protection Rules

### Required Status Checks

The following status checks are required before merge:

- `Code Quality & Linting`
- `Unit Tests & Coverage`
- `Security Analysis`
- `Docker Build & Scan`
- `Integration Tests`
- `Performance Tests`
- `Documentation Tests`

### Pull Request Requirements

- **2 approving reviews** required
- **Dismiss stale reviews** when new commits are pushed
- **Require code owner reviews** (see `.github/CODEOWNERS`)
- **Require conversation resolution** before merge
- **No force pushes** allowed to protected branches

### Setup Instructions

1. **Configure Secrets** (see `docs/github-secrets.md`):
   ```bash
   # Required secrets
   DOCKER_USERNAME=your-docker-username
   DOCKER_PASSWORD=your-docker-password
   API_KEY_1=your-secure-api-key-1
   API_KEY_2=your-secure-api-key-2
   ```

2. **Set Up Branch Protection**:
   ```bash
   # Set environment variables
   export REPO_OWNER=your-github-org
   export REPO_NAME=real-time-analytics-platform
   export GITHUB_TOKEN=your-github-token
   
   # Run setup script
   ./scripts/setup-branch-protection.sh setup
   ```

3. **Verify Configuration**:
   ```bash
   # Check branch protection status
   ./scripts/setup-branch-protection.sh status
   ```

## Integration Test Development

### Adding Integration Tests

1. **Create test files** with build tags:
   ```go
   //go:build integration
   // +build integration
   
   package main
   
   import "testing"
   
   func TestIntegrationExample(t *testing.T) {
       if testing.Short() {
           t.Skip("Skipping integration test in short mode")
       }
       // Your integration test code
   }
   ```

2. **Use appropriate naming**:
   - File: `*_integration_test.go`
   - Functions: `TestIntegration*`

3. **Follow patterns**:
   - Check for external dependencies
   - Clean up test data
   - Use realistic test scenarios
   - Test error conditions

### Running Integration Tests

```bash
# Run with integration tag
go test -tags=integration ./...

# Run only integration tests
go test -tags=integration -run=TestIntegration ./...

# Skip integration tests
go test -short ./...
```

## Performance Baseline Management

### Baseline Configuration

Performance baselines are defined in `config/performance-baselines.yaml`:

```yaml
services:
  data-ingestion-go:
    endpoints:
      "/api/data":
        rps_baseline: 1000
        p95_latency_ms: 50
        memory_per_request_kb: 10
```

### Updating Baselines

1. **After performance improvements**:
   ```bash
   # Update baseline values in config file
   vi config/performance-baselines.yaml
   ```

2. **Verify new baselines**:
   ```bash
   # Run performance tests locally
   go test -bench=. ./...
   ```

### Performance Monitoring

- Benchmarks run on every PR
- Results compared against baselines
- Alerts triggered for >10% degradation
- Historical data stored for trend analysis

## API Documentation

### OpenAPI Specifications

API documentation is maintained in `docs/api/`:

- `data-ingestion-api.yaml` - Data ingestion service API
- `clean-ingestion-api.yaml` - Clean ingestion service API

### Documentation Validation

The workflow validates:
- OpenAPI spec syntax
- Example data consistency
- Schema completeness
- Endpoint coverage

### Updating API Docs

1. **Update OpenAPI specs** when changing APIs
2. **Validate changes** locally:
   ```bash
   # Install swagger-cli
   npm install -g swagger-cli
   
   # Validate spec
   swagger-cli validate docs/api/data-ingestion-api.yaml
   ```

## Troubleshooting

### Common Issues

#### 1. Coverage Below Threshold
```
Error: Coverage 45.2% is below minimum threshold of 60%
```

**Solution**: Add more unit tests or adjust coverage threshold in workflow.

#### 2. Security Score Too Low
```
Error: Security score too low: 5/9
```

**Solution**: Fix security issues identified by `scripts/security-check.sh`.

#### 3. Docker Build Failures
```
Error: Docker build failed for service data-ingestion-go
```

**Solution**: Check Dockerfile syntax and dependencies.

#### 4. Integration Test Failures
```
Error: Kafka connection timeout
```

**Solution**: Ensure Kafka is properly configured in test environment.

### Debug Commands

```bash
# Check service directories
ls -la *-go/

# Validate Docker builds locally
docker build -t test-image ./data-ingestion-go/

# Run security checks locally
./scripts/security-check.sh

# Test branch protection setup
./scripts/setup-branch-protection.sh verify
```

### Getting Help

1. **Check workflow logs** in GitHub Actions tab
2. **Review this documentation** for configuration details
3. **Run tests locally** to reproduce issues
4. **Contact platform team** for complex issues

## Best Practices

### For Developers

1. **Run tests locally** before pushing:
   ```bash
   # Quick test suite
   make test-quick
   
   # Full test suite
   make test-full
   ```

2. **Keep PRs focused** and small for faster review
3. **Update documentation** when changing APIs
4. **Write integration tests** for new features
5. **Monitor performance** impact of changes

### For Reviewers

1. **Focus on critical paths** and security implications
2. **Verify test coverage** for new code
3. **Check documentation** completeness
4. **Validate performance** impact
5. **Ensure integration** with existing services

### For Platform Team

1. **Monitor baseline drift** over time
2. **Update security rules** regularly
3. **Review failed pipelines** for systemic issues
4. **Optimize build times** for developer productivity
5. **Keep dependencies** up to date

## Metrics and Monitoring

### Key Metrics

- **Pipeline Success Rate**: Target >95%
- **Average Pipeline Duration**: Target <10 minutes
- **Test Coverage**: Target >60% (service average)
- **Security Score**: Target >7/9
- **Performance Regression Rate**: Target <5%

### Monitoring Dashboards

Quality gates metrics are available in:
- GitHub Actions dashboard
- Custom Grafana dashboards
- Weekly quality reports

---

*This quality gates system ensures high code quality and prevents regressions while maintaining developer velocity. For questions or improvements, please contact the platform team.*
