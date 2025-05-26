#!/bin/bash

echo "🔍 Quality Gates Validation"
echo "=========================="

# Check existing services
echo "1. Checking services:"
for service in processing-engine-go data-ingestion-go clean-ingestion-go storage-layer-go visualization-go tenant-management-go; do
    if [ -d "$service" ] && [ -f "$service/go.mod" ]; then
        echo "✅ $service - exists"
    else
        echo "⚠️ $service - not found"
    fi
done

# Check workflows
echo ""
echo "2. Checking workflows:"
if [ -f ".github/workflows/ci.yml" ]; then
    echo "✅ CI workflow - exists"
else
    echo "❌ CI workflow - missing"
fi

if [ -f ".github/workflows/pre-merge-quality-gates.yml" ]; then
    echo "✅ Pre-merge quality gates - exists"
else
    echo "❌ Pre-merge quality gates - missing"
fi

if [ -f ".golangci.yml" ]; then
    echo "✅ golangci-lint config - exists"
else
    echo "❌ golangci-lint config - missing"
fi

# Test one service
echo ""
echo "3. Testing processing-engine-go:"
cd processing-engine-go
if go build ./... >/dev/null 2>&1; then
    echo "✅ Build - passed"
else
    echo "❌ Build - failed"
fi

if go test ./... >/dev/null 2>&1; then
    echo "✅ Tests - passed"
else
    echo "❌ Tests - failed"
fi

cd ..

echo ""
echo "Quality gates validation complete!"
