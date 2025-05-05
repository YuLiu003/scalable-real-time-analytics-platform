#!/bin/bash

# Colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Building Docker images for all services...${NC}"

# Connect to minikube's Docker daemon (if not already connected)
eval $(minikube docker-env) || true

# Keep track of success/failure
BUILD_SUCCESS=true
BUILT_IMAGES=()
FAILED_IMAGES=()

# Build data-ingestion-go
echo -e "${BLUE}Building data-ingestion-go image...${NC}"
if [ -d "./data-ingestion-go" ]; then
  if docker build -t data-ingestion-go:latest ./data-ingestion-go; then
    echo -e "${GREEN}Successfully built data-ingestion-go image${NC}"
    BUILT_IMAGES+=("data-ingestion-go")
  else
    echo -e "${YELLOW}Warning: Failed to build data-ingestion-go image, but continuing...${NC}"
    FAILED_IMAGES+=("data-ingestion-go")
  fi
else
  echo -e "${YELLOW}data-ingestion-go directory not found, skipping...${NC}"
fi

# Build clean-ingestion-go
echo -e "${BLUE}Building clean-ingestion-go image...${NC}"
if [ -d "./clean-ingestion-go" ]; then
  if docker build -t clean-ingestion-go:latest ./clean-ingestion-go; then
    echo -e "${GREEN}Successfully built clean-ingestion-go image${NC}"
    BUILT_IMAGES+=("clean-ingestion-go")
  else
    echo -e "${YELLOW}Warning: Failed to build clean-ingestion-go image, but continuing...${NC}"
    FAILED_IMAGES+=("clean-ingestion-go")
  fi
else
  echo -e "${YELLOW}clean-ingestion-go directory not found, skipping...${NC}"
fi

# Check for processing-engine-go
if [ -d "./processing-engine-go" ]; then
  echo -e "${BLUE}Building processing-engine-go image...${NC}"
  if docker build -t processing-engine-go:latest ./processing-engine-go; then
    echo -e "${GREEN}Successfully built processing-engine-go image${NC}"
    BUILT_IMAGES+=("processing-engine-go")
  else
    echo -e "${YELLOW}Warning: Failed to build processing-engine-go image, but continuing...${NC}"
    FAILED_IMAGES+=("processing-engine-go")
  fi
elif [ -d "./storage-layer-go/processing-engine-go" ]; then
  echo -e "${BLUE}Building processing-engine-go image...${NC}"
  echo -e "${YELLOW}Note: Using directory at ./storage-layer-go/processing-engine-go${NC}"
  if docker build -t processing-engine-go:latest ./storage-layer-go/processing-engine-go; then
    echo -e "${GREEN}Successfully built processing-engine-go image${NC}"
    BUILT_IMAGES+=("processing-engine-go")
  else
    echo -e "${YELLOW}Warning: Failed to build processing-engine-go image, but continuing...${NC}"
    FAILED_IMAGES+=("processing-engine-go")
  fi
else
  echo -e "${YELLOW}processing-engine-go directory not found, skipping...${NC}"
fi

# Build storage-layer-go
echo -e "${BLUE}Building storage-layer-go image...${NC}"
if [ -d "./storage-layer-go" ]; then
  if docker build -t storage-layer-go:latest ./storage-layer-go; then
    echo -e "${GREEN}Successfully built storage-layer-go image${NC}"
    BUILT_IMAGES+=("storage-layer-go")
  else
    echo -e "${YELLOW}Warning: Failed to build storage-layer-go image, but continuing...${NC}"
    FAILED_IMAGES+=("storage-layer-go")
  fi
else
  echo -e "${YELLOW}storage-layer-go directory not found, skipping...${NC}"
fi

# Build visualization-go
echo -e "${BLUE}Building visualization-go image...${NC}"
if [ -d "./visualization-go" ]; then
  if docker build -t visualization-go:latest ./visualization-go; then
    echo -e "${GREEN}Successfully built visualization-go image${NC}"
    BUILT_IMAGES+=("visualization-go")
  else
    echo -e "${YELLOW}Warning: Failed to build visualization-go image, but continuing...${NC}"
    FAILED_IMAGES+=("visualization-go")
    
    # Try a simpler build approach for visualization-go
    echo -e "${BLUE}Attempting alternate build for visualization-go...${NC}"
    if docker build -t visualization-go:latest -f - ./visualization-go <<EOF
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o visualization-go .

FROM alpine:3.18
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=build /app/visualization-go .
COPY --from=build /app/templates ./templates
COPY --from=build /app/static ./static
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 5003
CMD ["./visualization-go"]
EOF
    then
      echo -e "${GREEN}Successfully built visualization-go image with alternate approach${NC}"
      BUILT_IMAGES+=("visualization-go")
    else
      echo -e "${RED}All attempts to build visualization-go failed${NC}"
    fi
  fi
else
  echo -e "${YELLOW}visualization-go directory not found, skipping...${NC}"
fi

# Build tenant-management-go
echo -e "${BLUE}Building tenant-management-go image...${NC}"
if [ -d "./tenant-management-go" ]; then
  if docker build -t tenant-management-go:latest ./tenant-management-go; then
    echo -e "${GREEN}Successfully built tenant-management-go image${NC}"
    BUILT_IMAGES+=("tenant-management-go")
  else
    echo -e "${YELLOW}Warning: Failed to build tenant-management-go image, but continuing...${NC}"
    FAILED_IMAGES+=("tenant-management-go")
  fi
else
  echo -e "${YELLOW}tenant-management-go directory not found, skipping...${NC}"
fi

# Summarize build results
echo -e "\n${BLUE}Build Summary:${NC}"
echo -e "${GREEN}Successfully built images: ${#BUILT_IMAGES[@]}${NC}"
for img in "${BUILT_IMAGES[@]}"; do
  echo -e "  ${GREEN}✓ ${img}${NC}"
done

if [ ${#FAILED_IMAGES[@]} -gt 0 ]; then
  echo -e "${YELLOW}Failed images: ${#FAILED_IMAGES[@]}${NC}"
  for img in "${FAILED_IMAGES[@]}"; do
    echo -e "  ${YELLOW}✗ ${img}${NC}"
  done
  echo -e "${YELLOW}You may need to manually fix and build these images.${NC}"
fi

echo -e "\n${GREEN}Image building process complete.${NC}"
echo -e "${YELLOW}You can now run './manage.sh deploy' to deploy the platform.${NC}"

# Return success if at least some images were built successfully
if [ ${#BUILT_IMAGES[@]} -gt 0 ]; then
  exit 0
else
  exit 1
fi 