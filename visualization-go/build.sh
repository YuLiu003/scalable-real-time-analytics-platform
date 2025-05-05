#!/bin/bash

# Build script for the visualization-go service

echo "Building visualization-go service..."

# Make sure dependencies are up to date
echo "Updating dependencies..."
go mod tidy

# Build the service
echo "Building binary..."
go build -o visualization-go

echo "Build complete!"
echo "You can run the service with ./visualization-go"
