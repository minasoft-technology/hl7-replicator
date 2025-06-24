# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -extldflags '-static'" \
    -a -installsuffix cgo \
    -o hl7-replicator ./cmd/server

# Compress binary with UPX (optional, saves ~50% size)
RUN apk add --no-cache upx && \
    upx --best --lzma hl7-replicator || true

# Final stage - using distroless for security and minimal size
FROM gcr.io/distroless/static:nonroot

# Copy timezone data from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set timezone to Istanbul
ENV TZ=Europe/Istanbul

# Copy binary from builder
COPY --from=builder /build/hl7-replicator /hl7-replicator

# The distroless image runs as nonroot user by default (uid 65532)
# No need to create user or change permissions

# Expose ports
EXPOSE 7001 7002 5678

# Run the application
ENTRYPOINT ["/hl7-replicator"]