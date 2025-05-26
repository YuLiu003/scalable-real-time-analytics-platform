FROM golang:1.23-alpine AS build

# Install required dependencies (combining apk commands for better layer caching)
RUN apk update && apk add --no-cache git ca-certificates tzdata wget

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o processing-engine-go-new .

# Create final image
FROM alpine:latest

# Install required runtime dependencies including core libraries
RUN apk update && apk add --no-cache \
    ca-certificates \
    tzdata \
    wget \
    pcre2 \
    libidn2 \
    && rm -rf /var/cache/apk/*

# Create non-root user with specific UID/GID for better security
RUN addgroup -g 10001 -S appgroup && adduser -u 10001 -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy binary from build stage (change ownership during copy)
COPY --from=build --chown=appuser:appgroup /app/processing-engine-go-new .

# Copy config files if needed (change ownership during copy)
COPY --from=build --chown=appuser:appgroup /app/config ./config

# Use non-root user
USER appuser

# Add security labels
LABEL security.non-root=true \
      security.no-new-privileges=true \
      maintainer="analytics-platform-team"

# Expose port
EXPOSE 8000

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health || exit 1

# Run the binary with security options
CMD ["./processing-engine-go-new"]
