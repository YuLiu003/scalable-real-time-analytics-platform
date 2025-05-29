# Build stage
FROM golang:1.23-alpine AS build

# Set working directory
WORKDIR /app

# Install required dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum* ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o admin-ui-go .

# Final stage
FROM alpine:latest

# Add non root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy binary from build stage
COPY --from=build /app/admin-ui-go .

# Copy static files and templates
COPY --from=build /app/templates ./templates
COPY --from=build /app/static ./static

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 5050

# Run the application
CMD ["./admin-ui-go"] 