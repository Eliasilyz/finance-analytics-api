# Open Items & Pending Verification

This file tracks items that must be verified before a phase can be considered fully closed.

## Phase 1 — Foundation

### [OPEN] Docker Compose smoke test — `/health` returns 200
- **Status:** Not verified
- **Reason:** Docker daemon unavailable in the development environment at time of setup.
- **Must be verified:** Before Phase 1 is marked fully closed. Target: by Phase 8 at the latest.
- **Verification steps:**
  1. `docker compose up -d`
  2. `curl http://localhost:8080/health` → expect `{"status":"ok"}` HTTP 200
  3. `docker compose down`
- **Why it matters:** The DoD for Phase 1 explicitly requires `docker compose up` succeeds and `/health` returns 200.

## Phase 2 — Database & Migrations
- [ ] Migration up/down tested from clean state (pending PostgreSQL availability)
- [ ] Integration test verifying unique constraint on duplicate insert

## Phase 3 — Provider & Ingestion
- [ ] All 5 provider mock scenarios tested (success, timeout, error, rate-limited, invalid data)

## Phase 4 — Analytics Engine
- [ ] Unit tests for every metric including edge cases

## Phase 5 — REST API Layer
- [ ] Handler tests covering 200/400/404/429/500 status codes

## Phase 6 — Redis Caching & Rate Limiting
- [ ] Cache hit integration test
- [ ] Rate limiting blocking test

## Phase 7 — Security Hardening
- [ ] SQL injection / malicious input rejection tests

## Phase 8 — Docs, Polish, Full CI
- [ ] GitHub Actions CI verifies Docker-in-Docker works for testcontainers-go
- [ ] Complete OpenAPI spec
- [ ] README with architecture and setup instructions
