# Build stage
FROM golang:1.21-alpine AS builder

# Install SQLite with a specific gcc version to avoid build issues
RUN apk update && \
    apk add --no-cache git sqlite-dev g++ make && \
    apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/v3.15/main musl-dev

# Copy go mod and sum files
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o storage-layer

# Final stage
FROM alpine:latest

# Install SQLite and other runtime dependencies including wget for health checks
RUN apk add --no-cache ca-certificates tzdata sqlite wget

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Create data directory
RUN mkdir -p /data && chown -R appuser:appgroup /data

# Copy binary from build stage
COPY --from=builder /app/storage-layer .

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Set environment variables
ENV PORT=5002
ENV KAFKA_BROKER=kafka:9092
ENV KAFKA_TOPIC=processed-data
ENV DB_PATH=/data/analytics.db

# Expose port
EXPOSE 5002

# Run the binary
CMD ["./storage-layer"] 