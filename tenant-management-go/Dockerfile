# Build stage
FROM golang:1.24-alpine AS build

# Set working directory
WORKDIR /app

# Install required dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o tenant-management-go .

# Final stage
FROM alpine:latest

# Install CA certificates and wget for HTTPS support and health checks
RUN apk --no-cache add ca-certificates wget

# Add non root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy binary from build stage
COPY --from=build /app/tenant-management-go .

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Define non-sensitive environment variables
# Note: Sensitive configuration should be injected at runtime
# via Kubernetes secrets or config maps, not hardcoded here
ENV PORT=5010
ENV DATABASE_TYPE=memory
ENV DATABASE_URL=memory
ENV DEBUG_MODE=false

# The following env vars should be provided at runtime:
# - API_KEY_ENABLED: Whether API key validation is enabled
# - JWT_SECRET: Secret for signing JWTs 
# - API_KEY_1, API_KEY_2, API_KEY_3: API keys for default tenants

# Expose port
EXPOSE 5010

# Run the binary
CMD ["./tenant-management-go"] 