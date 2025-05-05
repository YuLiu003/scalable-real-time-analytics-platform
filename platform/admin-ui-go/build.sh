#!/bin/bash

# Build script for the Admin UI Go service

echo "Building Admin UI Go service..."

# Make sure dependencies are up to date
echo "Updating dependencies..."
go mod tidy

# Build the service
echo "Building binary..."
go build -o admin-ui-go

echo "Build complete!"
echo "You can run the service with ./admin-ui-go" 