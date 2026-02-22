# Multi-stage Docker build for coolify-patrol
FROM golang:1.23-alpine AS builder

# Install git for go mod and ca-certificates for HTTPS
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go modules files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version info
ARG VERSION=dev
ARG COMMIT=unknown  
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /coolify-patrol ./cmd/patrol

# Final stage - minimal Alpine image
FROM alpine:3.21

# Install ca-certificates and tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -s /bin/sh patrol

# Copy the binary from builder
COPY --from=builder /coolify-patrol /usr/local/bin/coolify-patrol

# Set proper ownership and permissions
RUN chown patrol:patrol /usr/local/bin/coolify-patrol && \
    chmod +x /usr/local/bin/coolify-patrol

# Create config directory
RUN mkdir -p /config && chown patrol:patrol /config

# Switch to non-root user
USER patrol

# Set working directory
WORKDIR /config

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default entrypoint
ENTRYPOINT ["coolify-patrol"]