# Build stage
FROM golang:1.25-alpine AS builder

# Install git and ca-certificates for fetching dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tailscale-bind-ddns .

# Final stage - distroless image
FROM gcr.io/distroless/static-debian12

# Add labels for container metadata
LABEL org.opencontainers.image.title="tailscale-bind-ddns"
LABEL org.opencontainers.image.description="Dynamic DNS service for Tailscale using BIND"
LABEL org.opencontainers.image.url="https://github.com/aauren/tailscale-bind-ddns"
LABEL org.opencontainers.image.source="https://github.com/aauren/tailscale-bind-ddns"
LABEL org.opencontainers.image.vendor="aauren"
LABEL org.opencontainers.image.licenses="MIT"

# Copy the binary from builder stage
COPY --from=builder /app/tailscale-bind-ddns /tailscale-bind-ddns

# Set the binary as the entrypoint
ENTRYPOINT ["/tailscale-bind-ddns"]
