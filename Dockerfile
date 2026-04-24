# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy dependency files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
    -o /app/bin/api \
    ./cmd/api/main.go

# =============================================================================
# Stage 2: Runner (minimal image)
# =============================================================================
FROM scratch

WORKDIR /app

# Copy certificates for HTTPS calls to payment providers
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/bin/api .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["./api"]