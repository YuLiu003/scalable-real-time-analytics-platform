#!/bin/bash
set -e
echo "Building processing-engine-go-new..."
go mod tidy
go build -v
echo "Build succeeded!"
