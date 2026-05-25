<p align="center">
  <h1 align="center">⚡ HP Gateway</h1>
  <p align="center">
    <strong>High-Performance API Gateway & Load Balancer</strong>
  </p>
  <p align="center">
    A production-grade API gateway built in Go — reverse proxying, intelligent load balancing, rate limiting, authentication, caching, circuit breaking, and real-time analytics.
  </p>
  <p align="center">
    <a href="#features">Features</a> •
    <a href="#architecture">Architecture</a> •
    <a href="#quick-start">Quick Start</a> •
    <a href="#configuration">Configuration</a> •
    <a href="#benchmarks">Benchmarks</a> •
    <a href="#api">API</a>
  </p>
</p>

---

## 🎯 What Is This?

HP Gateway is a mini **NGINX / Kong / Envoy** — a reverse proxy that sits between clients and your backend servers, providing:

- **Traffic routing** with intelligent load balancing
- **Rate limiting** to protect backends from abuse
- **Authentication** via JWT and API keys
- **Circuit breaking** to prevent cascading failures
- **Response caching** with LRU eviction
- **Real-time analytics** and Prometheus-compatible metrics
- **Health monitoring** with automatic failover

Built entirely in Go using the standard library's `net/http` and `httputil.ReverseProxy`.

---

## ✨ Features

### Core
| Feature | Description |
|---------|-------------|
| 🔄 **Reverse Proxy** | HTTP request forwarding with connection pooling, X-Forwarded headers |
| ⚖️ **Load Balancing** | 4 algorithms: Round Robin, Least Connections, Weighted RR, IP Hash |
| 💚 **Health Checks** | Background goroutines with configurable thresholds and auto-recovery |
| 🚦 **Rate Limiting** | Token Bucket algorithm, per-IP, configurable rate and burst |
| 🔐 **Authentication** | JWT (HS256) validation + API key auth with path exclusions |
| ⚡ **Circuit Breaker** | Three-state (Closed/Open/Half-Open) pattern for fault tolerance |
| 📦 **Response Cache** | In-memory LRU cache with TTL and hit/miss statistics |
| 📊 **Analytics** | Real-time request tracking, p99 latency, status code distribution |
| 📈 **Metrics** | Prometheus-compatible `/metrics` endpoint |
| 🌐 **CORS** | Configurable cross-origin resource sharing |
| 📝 **Logging** | Color-coded structured request logging |

### Infrastructure
| Feature | Description |
|---------|-------------|
| 🐳 **Docker** | Multi-stage build, ~15MB final image |
| 🐙 **Docker Compose** | Full stack: Gateway + Backends + Redis + Postgres + Prometheus + Grafana |
| 🔁 **CI/CD** | GitHub Actions pipeline (lint, test, build) |
| 🛑 **Graceful Shutdown** | SIGINT/SIGTERM handling with connection draining |

---

## 🏗️ Architecture

```
            Clients (HTTP)
                 │
                 ▼
    ┌────────────────────────┐
    │      HP Gateway        │
    │                        │
    │   Logger → CORS →      │
    │   Metrics → Analytics → │
    │   RateLimiter → Auth →  │
    │   CircuitBreaker →      │
    │                        │
    │   ┌────────────────┐   │
    │   │ Load Balancer  │   │
    │   │ (4 algorithms) │   │
    │   └───────┬────────┘   │
    │           │            │
    │   ┌───────▼────────┐   │
    │   │ Reverse Proxy  │   │
    │   └───────┬────────┘   │
    └───────────┼────────────┘
         ┌──────┼──────┐
         ▼      ▼      ▼
     Backend  Backend  Backend
      :9001   :9002    :9003
```

Detailed architecture docs: [`docs/architecture.md`](docs/architecture.md)

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.22+** (`brew install go`)

### 1. Clone the repository

```bash
git clone https://github.com/ayu853/gateway.git
cd gateway
```

### 2. Install dependencies

```bash
go mod download
```

### 3. Start test backends

```bash
# Terminal 1: Start 3 test backend servers
make backends
```

### 4. Start the gateway

```bash
# Terminal 2: Build and run the gateway
make run
```

### 5. Send requests through the gateway

```bash
# Requests are automatically load-balanced across backends
curl http://localhost:8080/
curl http://localhost:8080/
curl http://localhost:8080/

# View analytics
curl http://localhost:8080/api/analytics | python3 -m json.tool

# View backend health
curl http://localhost:8080/api/health | python3 -m json.tool

# View Prometheus metrics
curl http://localhost:8080/metrics

# View cache stats
curl http://localhost:8080/api/cache/stats | python3 -m json.tool
```

---

## ⚙️ Configuration

All settings are configured via `config/gateway.yaml`:

```yaml
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s

backends:
  - url: "http://localhost:9001"
    weight: 3
  - url: "http://localhost:9002"
    weight: 2
  - url: "http://localhost:9003"
    weight: 1

balancer:
  algorithm: "round-robin"   # round-robin | least-conn | weighted-rr | ip-hash

health:
  enabled: true
  interval: 10s
  unhealthy_threshold: 3

rate_limit:
  enabled: true
  requests_per_minute: 100
  burst: 20

auth:
  enabled: false
  jwt_secret: "your-secret"
  api_keys: ["key-1", "key-2"]

cache:
  enabled: true
  ttl: 5m
  max_size: 1000

metrics:
  enabled: true
  path: "/metrics"
```

See [`config/gateway.yaml`](config/gateway.yaml) for the full configuration reference.

---

## 📊 Load Balancing Algorithms

| Algorithm | Flag | Description | Best For |
|-----------|------|-------------|----------|
| **Round Robin** | `round-robin` | Sequential rotation | Equal-capacity backends |
| **Least Connections** | `least-conn` | Fewest active connections | Variable response times |
| **Weighted Round Robin** | `weighted-rr` | NGINX-style smooth weighted | Mixed-capacity backends |
| **IP Hash** | `ip-hash` | Consistent hashing on client IP | Session affinity |

---

## 🔌 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/*` | `ANY` | Proxied to backends via load balancer |
| `/api/health` | `GET` | Backend health status |
| `/api/analytics` | `GET` | Request analytics (latency, status codes, throughput) |
| `/api/cache/stats` | `GET` | Cache hit/miss statistics |
| `/metrics` | `GET` | Prometheus metrics |

---

## 🐳 Docker

### Build the image

```bash
docker build -f deployments/Dockerfile -t hp-gateway .
```

### Run with Docker Compose (full stack)

```bash
docker-compose -f deployments/docker-compose.yaml up
```

This starts: Gateway + 3 Backends + Redis + PostgreSQL + Prometheus + Grafana

---

## 📈 Benchmarks

Run benchmarks with:

```bash
# Install the benchmarking tool
go install github.com/rakyll/hey@latest

# Run the benchmark suite
make bench

# Or run the full benchmark script
chmod +x scripts/benchmark.sh
./scripts/benchmark.sh
```

---

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 📁 Project Structure

```
gateway/
├── cmd/gateway/          # Application entry point
├── internal/
│   ├── config/           # YAML configuration loader
│   ├── proxy/            # Reverse proxy engine
│   ├── loadbalancer/     # 4 LB algorithms
│   ├── healthcheck/      # Backend health monitoring
│   ├── middleware/        # Rate limiter, auth, logger, CORS, circuit breaker
│   ├── cache/            # LRU cache with TTL
│   ├── analytics/        # Request analytics
│   └── metrics/          # Prometheus metrics
├── config/               # YAML configuration
├── deployments/          # Docker & Compose files
├── scripts/              # Test backends & benchmarks
├── docs/                 # Architecture documentation
├── .github/workflows/    # CI/CD pipeline
├── Makefile              # Build automation
└── README.md
```

---

## 🛠️ Makefile Commands

```bash
make help       # Show all commands
make build      # Build the binary
make run        # Build and run
make test       # Run tests
make backends   # Start test backends
make bench      # Run benchmarks
make fmt        # Format code
make lint       # Run go vet
make clean      # Clean build artifacts
make all        # Format + lint + test + build
```

---

## 🗺️ Roadmap

- [x] Reverse proxy
- [x] 4 load balancing algorithms
- [x] Health checks with auto-failover
- [x] Rate limiting (Token Bucket)
- [x] JWT + API key authentication
- [x] Circuit breaker pattern
- [x] In-memory LRU cache
- [x] Request analytics
- [x] Prometheus metrics
- [x] Docker & Docker Compose
- [x] GitHub Actions CI/CD
- [ ] Redis distributed caching
- [ ] PostgreSQL analytics storage
- [ ] WebSocket proxying
- [ ] HTTP/2 + gRPC support
- [ ] Service discovery (etcd/Consul)
- [ ] Distributed rate limiting
- [ ] Admin dashboard UI

---


<p align="center">
  Built with ❤️ in Go
</p>
