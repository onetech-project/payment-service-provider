# Build Stage
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build static binary with compiler flags
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app/server ./cmd/api

# Final Production Stage (Non-Root User)
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata \
    # Create dedicated non-root user
    && addgroup -S appgroup && adduser -S appuser -G appgroup -u 10001

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder /app/server /app/server
COPY --from=builder /app/db/migrations /app/db/migrations

# Assign ownership to non-root user
RUN chown -R appuser:appgroup /app

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/app/server"]
