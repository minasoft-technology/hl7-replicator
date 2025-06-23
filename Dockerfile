# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hl7-replicator ./cmd/server

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

# Set timezone to Istanbul
ENV TZ=Europe/Istanbul

# Create data directory
RUN mkdir -p /data && chown -R app:app /data

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/hl7-replicator .

# Change ownership
RUN chown -R app:app /app

# Switch to non-root user
USER app

# Expose ports
EXPOSE 7001 7002 5678

# Volume for persistent data
VOLUME ["/data"]

# Run the application
CMD ["./hl7-replicator"]