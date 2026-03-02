# API Gateway Service

A production-ready API Gateway/Proxy service in Go that validates Bearer tokens stored in Redis, enforces per-token rate limiting with a sliding window algorithm, checks allowed routes, and proxies requests to configurable backends.

## Architecture

```
Client → [Recovery → RequestID → Logging → Metrics → Auth → RouteCheck → RateLimit] → Proxy → Backend
```

### Key Components

| Component | Description |
|-----------|-------------|
| **Token Store** | Redis-backed storage for API tokens with TTL |
| **Rate Limiter** | Sliding window algorithm via Redis Lua script (atomic, distributed) |
| **Route Check** | Prefix-based path matching against per-token allowed routes (`/api/v1/users/*` matches all sub-paths) |
| **Circuit Breaker** | Wraps reverse proxy; opens on consecutive backend failures |
| **Metrics** | Prometheus counters and histograms for HTTP requests |

### Project Structure

```
cmd/
  api-gw/    → Main server entry point with graceful shutdown
  seed/      → CLI tool to seed tokens into Redis from JSON
internal/
  config/    → YAML-based configuration with validation
  redis/     → Redis client factory
  token/     → Token model, Store interface, Redis implementation
  ratelimiter/ → Limiter interface, Redis sliding window implementation
  middleware/  → Auth, RouteCheck, RateLimit, Logging, Metrics, Recovery, RequestID
  proxy/     → Reverse proxy wrapper with circuit breaker
  health/    → /healthz and /readyz endpoints
  gateway/   → Chi router assembly
```

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Task](https://taskfile.dev/) (task runner)
- [golangci-lint](https://golangci-lint.run/) (for linting)
- [Docker](https://www.docker.com/) + Docker Compose

## Quick Start

### With Docker Compose

```bash
# Start Redis + upstream services + API gateway + seed tokens
task compose-up

# Test a request
curl http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer full-access-key"

# Stop everything
task compose-down
```

### Local Development

```bash
# Start Redis locally (or via Docker)
docker run -d -p 6379:6379 redis:7-alpine

# Seed tokens
task seed

# Run the gateway
task run
```

## Configuration

Configuration is loaded from a YAML file (`config.yaml` by default, overridable with `--config`).
Environment variables can be referenced using `${VAR}` or `${VAR:-default}` syntax.
Use `$$` to produce a literal `$` (e.g. a Redis password containing `$`).

```yaml
server:
  port: 8080
  metrics_port: 9090
  max_body_bytes: 10485760  # 10 MB
  read_timeout: 10s
  write_timeout: 15s
  idle_timeout: 120s
  shutdown_timeout: 10s
  read_header_timeout: 5s

redis:
  addr: ${REDIS_ADDR:-localhost:6379}
  password: ${REDIS_PASSWORD:-}
  db: 0
  pool_size: 128
  min_idle_conns: 16
  tls_enabled: false

upstreams:
  - prefix: /api/v1/users
    url: ${UPSTREAM_USERS_URL:-http://upstream-1:8081}
    strip_prefix: false
  - prefix: /api/v1/products
    url: ${UPSTREAM_PRODUCTS_URL:-http://upstream-2:8082}
    strip_prefix: false

proxy:
  dial_timeout: 5s
  response_header_timeout: 10s
  idle_conn_timeout: 90s
  max_idle_conns: 128
  max_idle_conns_per_host: 32

circuit_breaker:
  max_failures: 5
  timeout: 60s

rate_limit:
  window: 60s

token_cache:
  enabled: false
  ttl: 30s
```

#### Server

| Field | Default | Description |
|-------|---------|-------------|
| `server.port` | `8080` | Main server listen port |
| `server.metrics_port` | `9090` | Metrics/health server listen port |
| `server.max_body_bytes` | `10485760` | Max request body size (bytes) |
| `server.read_timeout` | `10s` | HTTP read timeout |
| `server.write_timeout` | `15s` | HTTP write timeout |
| `server.idle_timeout` | `120s` | HTTP idle timeout |
| `server.shutdown_timeout` | `10s` | Graceful shutdown timeout |
| `server.read_header_timeout` | `5s` | HTTP read header timeout |

#### Redis

| Field | Default | Description |
|-------|---------|-------------|
| `redis.addr` | `localhost:6379` | Redis address |
| `redis.password` | *(empty)* | Redis password |
| `redis.db` | `0` | Redis database number |
| `redis.pool_size` | `128` | Connection pool size |
| `redis.min_idle_conns` | `16` | Minimum idle connections |
| `redis.tls_enabled` | `false` | Enable TLS (minimum TLS 1.2) |

#### Upstreams

| Field | Default | Description |
|-------|---------|-------------|
| `upstreams[].prefix` | — | Path prefix to match (must start with `/`, must not end with `/`) |
| `upstreams[].url` | — | Backend service URL (http or https) |
| `upstreams[].strip_prefix` | `false` | Strip the prefix before forwarding |

#### Proxy

| Field | Default | Description |
|-------|---------|-------------|
| `proxy.dial_timeout` | `5s` | TCP dial timeout to backends |
| `proxy.response_header_timeout` | `10s` | Timeout waiting for backend response headers |
| `proxy.idle_conn_timeout` | `90s` | Idle connection timeout |
| `proxy.max_idle_conns` | `128` | Total max idle connections across all backends |
| `proxy.max_idle_conns_per_host` | `32` | Max idle connections per backend |

#### Circuit Breaker

| Field | Default | Description |
|-------|---------|-------------|
| `circuit_breaker.max_failures` | `5` | Consecutive failures before opening the circuit |
| `circuit_breaker.timeout` | `60s` | Time in open state before transitioning to half-open |

#### Rate Limit

| Field | Default | Description |
|-------|---------|-------------|
| `rate_limit.window` | `60s` | Sliding window duration (must be a whole number of seconds) |

#### Token Cache

| Field | Default | Description |
|-------|---------|-------------|
| `token_cache.enabled` | `false` | Enable in-memory token cache |
| `token_cache.ttl` | — | Cache entry TTL (required when enabled) |

## Token Management

Tokens are seeded into Redis using the `cmd/seed` CLI tool:

```bash
go run ./cmd/seed --config config.yaml tests/tokens.json
```

### Token Structure

```json
{
  "api_key": "xxx-xxx-xxx",
  "rate_limit": 100,
  "expires_at": "2027-12-31T23:59:59Z",
  "allowed_routes": [
    "/api/v1/users/*",
    "/api/v1/products/*"
  ]
}
```

Tokens are stored in Redis with key prefix `token:` and TTL derived from `expires_at`.

## API Endpoints

### Proxy (protected — all HTTP methods)
```
{GET,POST,PUT,DELETE,PATCH} /api/v1/*
Authorization: Bearer <api_key>
```

### Health & Observability (metrics port, unprotected)

Served on the metrics port (default `9090`), not the main port:
```
GET :9090/healthz   → Liveness check
GET :9090/readyz    → Readiness check (pings Redis)
GET :9090/metrics   → Prometheus metrics
```

### Error Responses

All errors return consistent JSON:
```json
{
  "error": {
    "code": 401,
    "message": "invalid token"
  }
}
```

| Status | Meaning |
|--------|---------|
| `401` | Missing/invalid/expired token |
| `403` | Route not in allowed list |
| `429` | Rate limit exceeded |
| `502` | Backend unreachable |
| `503` | Circuit breaker open |

### Rate Limit Headers

Every proxied response includes:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 97
X-RateLimit-Reset: 1709000000
```

## Design Decisions

1. **Sliding Window Rate Limiting** — Uses a Redis Lua script with sorted sets for atomic, distributed rate limiting. Avoids the boundary burst problem of fixed windows.

2. **Interfaces for Testability** — `token.Store` and `ratelimiter.Limiter` are interfaces, enabling unit tests with mocks and easy swapping of implementations.

3. **Stdlib slog** — Structured logging with zero external dependencies.

4. **Prefix-based Route Matching** — Patterns ending in `/*` use prefix matching to support multi-level sub-paths (e.g. `/api/v1/users/*` matches `/api/v1/users/123/orders/456`). Exact patterns use Go's stdlib `path.Match`.

5. **Circuit Breaker** — Wraps the reverse proxy using `gobreaker`. Opens after consecutive failures (default 5), waits a configurable timeout (default 60s) in open state before transitioning to half-open, preventing cascade failures.

6. **Graceful Shutdown** — Handles SIGINT/SIGTERM, drains in-flight requests, closes Redis connection.

7. **YAML Config with Env Var Expansion** — YAML was chosen over pure env vars for readability of nested and list-based config (upstreams). Environment variables can still be injected via `${VAR}` / `${VAR:-default}` syntax, giving the best of both worlds for 12-factor deployments. Use `$$` to escape a literal `$`.

## Development

```bash
# Run tests
task test

# Run tests with coverage
task test-cover

# Run linter
task lint

# Build binary
task build

# Build Docker image
task docker-build
```

## Load Testing

Load tests use [k6](https://grafana.com/docs/k6/) and live under `tests/k6/`. They require the full compose stack to be running.

### Quick Start

```bash
# Start the stack (Redis + upstreams + gateway + seed tokens)
task compose-up

# Run a quick smoke test
task k6-smoke

# Run all test suites sequentially
task k6

# Stop the stack
task compose-down
```

### Test Profiles

| Profile | Command | Duration | VUs | Purpose |
|---------|---------|----------|-----|---------|
| **Smoke** | `task k6-smoke` | ~10s | 1 | Sanity check — all endpoints, all tokens, correct status codes |
| **Load** | `task k6-load` | ~2min | 50 | Normal traffic with multi-token contention |
| **Stress** | `task k6-stress` | ~2min | 250 | Push past limits to find the breaking point |
| **Spike** | `task k6-spike` | ~1min | 5→150→5 | Sudden burst followed by recovery |
| **Rate Limit** | `task k6-rate-limit` | ~30s | 17 | Rate limiting verification + auth failure testing |

### Thresholds

| Profile | p95 | p99 | Max Error Rate |
|---------|-----|-----|----------------|
| Smoke | 500ms | 1s | 0% |
| Load | 300ms | 500ms | 1% |
| Stress | 1s | 2s | 5% |
| Spike | 1.5s | 3s | 10% |

### Targeting a Different Environment

All profiles accept k6 flags via `CLI_ARGS`:

```bash
task k6-smoke -- --env BASE_URL=http://staging:8080
```

### Project Structure

```
tests/k6/
  helpers/
    config.js      → Base URL, endpoint paths
    tokens.js      → Token constants, auth header builders
    checks.js      → Custom metrics, reusable check functions
    requests.js    → Request functions per endpoint, grouped flows
  smoke.js         → Smoke test
  load.js          → Normal load test
  stress.js        → Stress test
  spike.js         → Spike test
  rate-limit.js    → Rate limit + auth failure tests
```
