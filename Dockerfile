# Multi-stage build for minimal image
FROM golang:1.23-alpine AS builder

# Enable automatic toolchain downloads
ENV GOTOOLCHAIN=auto

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o app ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget && \
  adduser -D -s /bin/sh appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/app /app/app

# Copy web assets
COPY --from=builder /build/web /app/web

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

CMD ["/app/app"]
