# ── Stage 1: Build ──────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files first (cache-friendly)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source
COPY . .

# Build with race detector disabled for production binary, CGO off for scratch
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /bin/recourse ./cmd/server

# ── Stage 2: Production image ────────────────────────────────────────
FROM gcr.io/distroless/static-debian12

# Copy timezone data and CA certs from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /bin/recourse /recourse

# Copy migrations
COPY --from=builder /app/migrations /migrations

# Copy web assets (documentation frontend)
COPY --from=builder /app/web /web

EXPOSE 8083

USER nonroot:nonroot

ENTRYPOINT ["/recourse"]
