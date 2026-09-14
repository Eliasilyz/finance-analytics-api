# Architecture Decisions Log

## Project: Financial Data Analytics API

### 1. Language: Go
- Chosen: Go 1.22+
- Why: Required by spec. Demonstrates backend engineering beyond CRUD.

### 2. HTTP Framework: Gin-gonic
- Chosen: gin-gonic/gin v1.9+
- Why: Mature, minimal overhead, good middleware support.
- Rejected: Echo, Fiber, net/http (too verbose for this scope).

### 3. Database: PostgreSQL + golang-migrate
- Chosen: PostgreSQL, golang-migrate/migrate v4
- Why: Spec mandates Postgres. Explicit versioned SQL migrations with up/down.
- Rejected: goose, raw SQL scripts (no versioning).

### 4. Redis Client: go-redis v9
- Chosen: redis/go-redis v9
- Why: Official client, context-based, built-in retry.
- Rejected: v8 (deprecated), miniredis (test-only).

### 5. Auth: API Key (bcrypt-hashed in DB, X-API-Key header)
- Chosen: API key hashed (bcrypt) in DB, checked via X-API-Key header. Full design logged here before Phase 7.
- Why: Spec allows API key or simple JWT. API key simpler; bcrypt hash means no plaintext secret at rest.
- Rejected: JWT (unnecessary complexity), OAuth (out of scope).

### 6. Provider: Alpha Vantage
- Chosen: Alpha Vantage REST API, interface-based client.
- Rejected: Yahoo Finance (unreliable), FMP (paid).

### 7. Testing: stdlib + testcontainers-go
- Chosen: testing stdlib, testcontainers/testcontainers-go for real Postgres/Redis in integration tests.
- Rejected: mockery, gomock (code-gen overhead).

### 8. Linting: golangci-lint v1.60+
- Chosen: golangci-lint
- Rejected: go vet alone (too limited).

### 9. Containers: Docker Compose v2
- Rejected: Kubernetes (prohibited by spec s16).

### 10. CI: GitHub Actions
- Note: Must verify Docker-in-Docker works for testcontainers before Phase 8 sign-off.

### 11. Module Path
- Chosen: github.com/eliasilyz/finance-analytics-api (real username, confirmed by user).

### 12. Config: godotenv + os.Getenv
- Why: Env-based, no hardcoded secrets.
- Rejected: viper (too heavy).

### 13. Analytics: pure functions only
- Why: Spec s11. No DB/HTTP deps, trivially testable.

### 14. Migrations: explicit SQL files
- Why: Auditable, reviewable, up/down.
