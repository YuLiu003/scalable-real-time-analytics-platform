# --- Build Stage ---
FROM golang:1.24-alpine AS builder

# Set necessary environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Copy go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application binary
# -ldflags="-w -s" strips debug information and symbols for a smaller binary
RUN go build -ldflags="-w -s" -o /app/gin-api .

# --- Runtime Stage ---
# Use a minimal image. Distroless static is good for static Go binaries.
FROM gcr.io/distroless/static-debian11
# Alternatively, use alpine:
# FROM alpine:latest
# RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from the build stage
COPY --from=builder /app/gin-api /app/gin-api

# Copy any other needed non-code assets (e.g., templates, static files)
# COPY --from=builder /app/templates ./templates

# Set the user (match Kubernetes security context if possible)
# Distroless static runs as non-root by default.
# If using Alpine, uncomment these lines:
# RUN addgroup -S nonroot && adduser -S nonroot -G nonroot
# USER nonroot:nonroot

# Expose the port the app runs on (matches the PORT env var default)
EXPOSE 8080

# Define the entrypoint
ENTRYPOINT ["/app/gin-api"] 