# Architecture

## Overview

HP Gateway is a high-performance API gateway and load balancer built in Go. It acts as a reverse proxy, intelligently routing client requests to a pool of backend servers while providing essential middleware services.

## System Architecture

```
                    ┌─────────────────┐
                    │     Clients     │
                    │  (HTTP/HTTPS)   │
                    └────────┬────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │      API Gateway       │
                │                        │
                │  ┌──────────────────┐  │
                │  │  Middleware Chain │  │
                │  │                  │  │
                │  │  1. Logger       │  │
                │  │  2. CORS         │  │
                │  │  3. Metrics      │  │
                │  │  4. Analytics    │  │
                │  │  5. Rate Limiter │  │
                │  │  6. Auth (JWT)   │  │
                │  │  7. Circuit      │  │
                │  │     Breaker      │  │
                │  └───────┬──────────┘  │
                │          │             │
                │  ┌───────▼──────────┐  │
                │  │  Load Balancer   │  │
                │  │  ┌────────────┐  │  │
                │  │  │Round Robin │  │  │
                │  │  │Least Conn  │  │  │
                │  │  │Weighted RR │  │  │
                │  │  │IP Hash     │  │  │
                │  │  └────────────┘  │  │
                │  └───────┬──────────┘  │
                │          │             │
                │  ┌───────▼──────────┐  │
                │  │  Reverse Proxy   │  │
                │  │  (httputil)      │  │
                │  └───────┬──────────┘  │
                │          │             │
                │  ┌───────▼──────────┐  │
                │  │  Health Checker  │  │
                │  │  (background)    │  │
                │  └──────────────────┘  │
                │                        │
                │  ┌──────────────────┐  │
                │  │  In-Memory Cache │  │
                │  │  (LRU + TTL)     │  │
                │  └──────────────────┘  │
                └──────────┬─────────────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
            ▼              ▼              ▼
     ┌────────────┐ ┌────────────┐ ┌────────────┐
     │ Backend A  │ │ Backend B  │ │ Backend C  │
     │ :9001      │ │ :9002      │ │ :9003      │
     └────────────┘ └────────────┘ └────────────┘
```

## Request Flow

1. **Client** sends an HTTP request to the gateway (port 8080)
2. **Logger** middleware records the request start time
3. **CORS** middleware adds cross-origin headers
4. **Metrics** middleware increments request counters
5. **Analytics** middleware starts tracking the request
6. **Rate Limiter** checks if the client IP has exceeded their rate limit
7. **Auth** middleware validates JWT token or API key (if enabled)
8. **Circuit Breaker** checks if the backend circuit is open
9. **Load Balancer** selects the best backend based on the configured algorithm
10. **Reverse Proxy** forwards the request to the selected backend
11. Response flows back through the middleware chain (in reverse)

## Load Balancing Algorithms

### Round Robin
Simple rotation through backends. Each backend receives requests in turn.
- **Time Complexity:** O(n) worst case (when backends are unhealthy)
- **Thread Safety:** Atomic counter, lock-free

### Least Connections
Routes to the backend with the fewest active connections.
- **Time Complexity:** O(n) scan
- **Best For:** Backends with varying response times

### Weighted Round Robin (NGINX-style)
Smooth weighted distribution. Backends with higher weights get proportionally more traffic.
- **Algorithm:** Each round, add weight to current_weight, select highest, subtract total_weight
- **Best For:** Heterogeneous backend capacities

### IP Hash
Consistent hashing based on client IP. Same client always hits the same backend.
- **Algorithm:** FNV-1a hash of client IP → modulo backend count
- **Best For:** Session affinity / sticky sessions

## Circuit Breaker States

```
     ┌──────────┐    failure threshold    ┌──────────┐
     │  CLOSED  │ ──────────────────────▶ │   OPEN   │
     │ (normal) │                         │ (reject) │
     └──────────┘                         └──────────┘
          ▲                                    │
          │                              timeout elapsed
          │                                    │
          │         success threshold     ┌────▼─────┐
          └────────────────────────────── │HALF-OPEN │
                                          │ (probe)  │
                                          └──────────┘
```

- **Closed:** Normal operation, requests pass through
- **Open:** Backend failing, requests rejected immediately (503)
- **Half-Open:** Testing recovery, limited requests allowed

## Rate Limiting — Token Bucket

```
Bucket Capacity: burst (e.g., 20 tokens)
Refill Rate: requests_per_minute / 60 (tokens per second)

Each request consumes 1 token.
If bucket empty → 429 Too Many Requests
Tokens refill continuously based on elapsed time.
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Goroutines, stdlib `net/http`, infrastructure ecosystem |
| Proxy Engine | `httputil.ReverseProxy` | Battle-tested, part of stdlib |
| Config Format | YAML | Human-readable, widely used in infra |
| Cache | In-memory LRU | Zero dependencies, fast; Redis optional |
| Concurrency | `sync/atomic` + `sync.Mutex` | Lock-free where possible |
| Metrics Format | Prometheus text | Industry standard, scrape-ready |
