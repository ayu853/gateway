# ============================================================================
# Dockerfile — HP Gateway (Multi-stage build)
# ============================================================================
# Produces a minimal ~15MB container image
# ============================================================================

# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gateway ./cmd/gateway/

# Stage 2: Runtime
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /gateway .
COPY config/gateway.yaml config/gateway.yaml

EXPOSE 8080

ENTRYPOINT ["./gateway"]
CMD ["-config", "config/gateway.yaml"]
