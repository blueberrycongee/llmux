# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /llmux \
    ./cmd/server

# Final stage - distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=builder /llmux /llmux

# Copy default config
COPY --from=builder /app/config/config.example.yaml /config/config.yaml

# Use non-root user
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/llmux"]
CMD ["--config", "/config/config.yaml"]
