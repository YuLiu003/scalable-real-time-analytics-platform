FROM golang:1.21-alpine AS build

# Install required dependencies
RUN apk add --no-cache git gcc libc-dev

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o processing-engine-go-new .

# Create final image
FROM alpine:3.18

# Install dependencies for Kafka
RUN apk add --no-cache librdkafka-dev ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy binary from build stage
COPY --from=build /app/processing-engine-go-new .

# Set ownership
RUN chown -R appuser:appgroup /app

# Use non-root user
USER appuser

# Expose port
EXPOSE 8000

# Run the binary
CMD ["./processing-engine-go-new"]
