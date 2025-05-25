# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first (if exists) to leverage Docker caching
COPY go.mod .
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o clean-ingestion .

# Stage 2: Create a minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install CA certificates and wget for HTTPS support and health checks
RUN apk --no-cache add ca-certificates wget

# Copy the binary from the builder stage
COPY --from=builder /app/clean-ingestion .

# Run as non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Expose the port the application runs on
EXPOSE 5000

# Command to run the application
CMD ["./clean-ingestion"] 