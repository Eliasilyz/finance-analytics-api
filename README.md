# Finance Data Analytics API

A backend REST API that ingests company and stock data from an external provider
(Alpha Vantage), stores it in PostgreSQL, computes its own financial and
technical metrics, and serves the results to clients over an API-key-protected
REST interface with Redis-backed caching and rate limiting.

This is not a thin proxy: data is ingested, normalized, recomputed, and cached
on our side. Clients only ever talk to this API.

```
Alpha Vantage ──> Ingestion ──> PostgreSQL ──> Analytics engine ──> Redis ──> REST API
```

## Stack

| Concern        | Choice                                  |
|----------------|-----------------------------------------|
| Language       | Go 1.25+                                 |
| HTTP           | Gin                                       |
| Database       | PostgreSQL 16 + golang-migrate (embedded) |
| Cache/limiter  | Redis 7 (go-redis v9)                     |
| Auth           | API key (`X-API-Key`), SHA-256 hashed     |
| Testing        | stdlib + testcontainers-go (integration)  |
| Lint           | golangci-lint v2                          |
| Containers     | Docker Compose v2                         |
| CI             | GitHub Actions (build + integration)      |

## Quickstart

```sh
# 1. Start the stack (api + postgres + redis). Migrations run automatically.
docker compose up --build -d
# -> api log: "migrations applied (or already up to date)"

# 2. Issue an API key. The raw key is shown exactly once.
#    See "Managing API keys" for how to run keygen against the compose database.

# 3. Hit an endpoint.
curl http://localhost:8080/health
curl -H "X-API-Key: <key>" http://localhost:8080/api/v1/companies
```

The API's Postgres/Redis are reachable from the host on `localhost:5432` /
`localhost:6379`. If another Postgres already owns those host ports, either stop
that service or override the compose port mapping.

## Managing API keys

Keys are issued by the `keygen` CLI (raw tokens are never stored; only a SHA-256
hash and an 8-char lookup prefix are persisted):

```sh
# Build for linux so it runs inside the alpine api container:
GOOS=linux CGO_ENABLED=0 go build -o keygen ./cmd/keygen
docker cp keygen finance-data-analytics-api-api-1:/tmp/keygen

# Issue a key (raw token printed exactly once):
docker compose exec \
  -e DATABASE_URL=postgres://postgres:postgres@postgres:5432/finance_db?sslmode=disable \
  api /tmp/keygen create --label frontend --expires-in 365d

# other subcommands: list, revoke --id <id>
```

Endpoints under `/api/v1` reject requests without a valid key with a uniform
`401`; per-API-key rate limiting returns `429 RATE_LIMITED`.

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable                       | Required | Default | Meaning                              |
|--------------------------------|----------|---------|--------------------------------------|
| `DATABASE_URL`                 | yes      | —       | Postgres connection string            |
| `REDIS_URL`                    | yes      | —       | Redis connection string               |
| `ALPHA_VANTAGE_API_KEY`        | no       | —       | Provider key (empty = ingestion dry)  |
| `PORT`                         | no       | `8080`  | HTTP listen port                      |
| `RATE_LIMIT_LIMIT`             | no       | `100`   | Per-key requests per window           |
| `RATE_LIMIT_WINDOW_SECONDS`    | no       | `60`    | Rate-limit window                     |
| `AUTH_GUARD_IP_LIMIT`          | no       | `500`   | Per-IP failed-auth attempts per window|
| `AUTH_GUARD_IP_WINDOW_SECONDS` | no       | `60`    | Auth-guard window                     |
| `APP_ENV`                      | no       | `development` | Runtime environment label       |

The API fails open on Redis outages: a cache miss falls through to the DB and a
dead limiter allows the request (300 ms timeouts). Only the auth paths are
strictly guarded.

## API

Full machine-readable spec: [`docs/openapi.yaml`](docs/openapi.yaml) (OpenAPI 3).
Summary of endpoints:

| Method | Path                                          | Auth | Description                          |
|--------|-----------------------------------------------|------|--------------------------------------|
| GET    | `/health`                                     | no   | Liveness probe                       |
| GET    | `/api/v1/companies`                           | yes  | List companies                       |
| GET    | `/api/v1/companies/{symbol}`                  | yes  | Company details                      |
| GET    | `/api/v1/stocks/{symbol}/history?limit=`      | yes  | Daily OHLCV history                  |
| GET    | `/api/v1/stocks/{symbol}/income-statement`    | yes  | Income statements (`period=`)        |
| GET    | `/api/v1/stocks/{symbol}/balance-sheet`       | yes  | Balance sheets (`period=`)           |
| GET    | `/api/v1/stocks/{symbol}/cash-flow`           | yes  | Cash flow statements (`period=`)     |
| GET    | `/api/v1/stocks/{symbol}/analytics`           | yes  | Financial metrics                    |
| GET    | `/api/v1/stocks/{symbol}/technical`           | yes  | Technical indicators                 |
| GET    | `/api/v1/compare?symbols=AAPL,MSFT`           | yes  | Cross-symbol metrics (max 10)        |
| GET    | `/api/v1/screener?sector=&min_roe=&max_pe=&min_revenue_growth=` | yes | Filter stocks |

Error responses use a single envelope:

```json
{ "error": { "code": "NOT_FOUND", "message": "..." } }
```

Codes: `VALIDATION_ERROR` (400), `NOT_FOUND` (404), `INSUFFICIENT_DATA` (422),
`RATE_LIMITED` (429), `UNAUTHORIZED` (401), `INTERNAL` (500).

## Project layout

```
cmd/api        HTTP server entrypoint (migrate-up at startup, wiring)
cmd/keygen     API key CLI (create / list / revoke)
internal/
  auth         API key issuing + validation (SHA-256, prefix lookup)
  cache        Redis read-through cache (TTL 15m, active invalidation on ingest)
  rate         fixed-window rate limiter (per-API-key + per-IP auth guard)
  handler      REST handlers + shared error envelope + validation
  models       shared domain types
  provider     Alpha Vantage client (snapshot fetch)
  repository   PostgreSQL access (read + write + screener)
  service      query service + analytics/technical computation
  analytics    pure functions for financial & technical metrics
migrations/    SQL migrations embedded into the binary
test/integration  end-to-end tests (testcontainers, -tags integration)
docs/          DECISIONS.md (ADR log), OPEN_ITEMS.md, openapi.yaml
```

## Testing

```sh
go test ./internal/...                       # unit tests (no Docker needed)
go test -tags integration -v ./test/integration/...   # needs Docker
```

Integration tests spin up real Postgres/Redis via testcontainers-go. CI runs the
unit + lint + build in one job and the integration suite in another.

## Known limitations / Future work

Documented scope boundaries (see also `docs/OPEN_ITEMS.md`):

- **No automatic key rotation / expiry job.** Keys carry an optional
  `expires_at` that is enforced on every request, and can be revoked manually
  with `keygen revoke --id`, but there is no background job that rotates or
  cleans up keys. If automated rotation is needed, schedule a job that issues a
  new key and revokes the old one.
- **No admin self-service for keys.** Keys are created via the `keygen` CLI
  with direct database access; there is no API/web endpoint that issues keys.
- **Cache invalidation is best-effort.** After a successful ingest the cache is
  actively purged; if the cache delete fails, the 15-minute TTL guarantees
  eventual consistency. There is no retry/queue for invalidation failures.

## Documentation

- [`docs/DECISIONS.md`](docs/DECISIONS.md) — architecture decision log per phase.
- [`docs/OPEN_ITEMS.md`](docs/OPEN_ITEMS.md) — audit trail of open/closed items.
- [`docs/openapi.yaml`](docs/openapi.yaml) — API specification.