# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first (if exists) to leverage Docker caching
COPY go.mod .
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o data-ingestion .

# Stage 2: Create a minimal runtime image
FROM alpine:3.18

WORKDIR /app

# Install CA certificates for HTTPS support
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/data-ingestion .

# Run as non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Expose the port the application runs on
EXPOSE 5000

# Command to run the application
CMD ["./data-ingestion"] 