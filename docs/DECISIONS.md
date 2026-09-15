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




### 15. Integration test infra: testcontainers-go KONFIRMASI tetap dipakai (clarification 2026-09-15)
- Klarifikasi: selama Phase 2 pengembangan sempat dibuat draft test berbasis DSN env (TEST_DATABASE_URL) yang mengharuskan Postgres lokal manual. Itu PENYIMPANGAN sementara dan TIDAK dipakai dan TIDAK di-commit. Disadari bukan keputusan sendiri sehingga ditanyakan ke user; user menegaskan testcontainers-go tetap approach yang benar (tidak perlu password/DB manual).
- Keputusan final: integration test memakai testcontainers-go (module postgres), memerlukan Docker daemon hidup (local dev: Docker Desktop; CI: GitHub-hosted runner sudah punya daemon, tanpa setup DinD tambahan). Test men-spawn postgres:16-alpine sendiri, apply migrations via golang-migrate (source file), verifikasi unique violation (SQLSTATE 23505), lalu migrate down.
- Alasan DSN-env ditolak: butuh setup DB/password manual di luar kontrol repo, gagal memenuhi janji testcontainers, dan tidak portabel antar developer.
- Opsi lain ditolak: go-redis/v8; miniredis; mockery/gomock (code-gen overhead).

### 16. Provider interface: satu method, satu snapshot utuh (Phase 3, 2026-09-15)
- Chosen: `FinancialDataProvider.FetchCompanySnapshot(ctx, symbol)` mengembalikan `CompanySnapshot{Company, Prices, Income, Balance}` — semua endpoint Alpha Vantage (OVERVIEW, TIME_SERIES_DAILY, INCOME_STATEMENT, BALANCE_SHEET) dalam SATU pemanggilan; kegagalan endpoint mana pun menggagalkan snapshot.
- Why: spec II.1 provider abstraction + ingestion wajib log sukses/gagal per run. Satu snapshot = satu run yang atomik, status integrity terjaga; alternatif per-endpoint menambah state complexity tanpa kebutuhan nyata saat ini.
- Note: CASH FLOW tidak di-fetch (tidak ada fitur yang memakainya) — ditunda, cukup tambah `fetchStatements` bila dipakai.
- Note: money disimpan sebagai float64 (`NUMERIC` di Postgres menerimanya); skala ini tidak sensitif desimal bank.

### 17. Ingestion audit: ingestion_runs + ON CONFLICT DO NOTHING (Phase 3, 2026-09-15)
- Chosen: tabel `ingestion_runs` (migration 000002) mencatat setiap run sukses/gagal + jumlah baris ter-insert + error; seluruh insert data idempoten via `ON CONFLICT DO NOTHING` (company di-upsert via `ON CONFLICT (symbol) DO UPDATE ... RETURNING id`).
- Why: spec II.2 (proses ingestion tercatat, sukses/gagal terlihat); re-ingest data yang sudah ada = run sukses dengan hitungan 0, tidak duplikat.
- Note: protokol validasi di trust boundary (symbol regex `^[A-Z0-9.]{1,10}$`, harga non-negatif + high>=low, fiscal date non-zero, period annual|quarterly); baris invalid dibuang, bukan menghentikan run (kecuali tidak ada satu pun harga valid -> run failure).
