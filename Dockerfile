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
RUN CGO_ENABLED=0 GOOS=linux go build -o visualization-go .

# Final stage
FROM alpine:latest

# Add non root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy binary from build stage
COPY --from=build /app/visualization-go .

# Copy static files and templates
COPY --from=build /app/templates ./templates
COPY --from=build /app/static ./static

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Define environment variables
ENV PORT=5003
ENV DATA_SERVICE_URL=http://data-ingestion-service
ENV MAX_DATA_POINTS=100
ENV DEBUG_MODE=false
ENV GIN_MODE=release

# Expose port
EXPOSE 5003

# Run the binary
CMD ["./visualization-go"] 