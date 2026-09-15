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
- Chosen: `FinancialDataProvider.FetchCompanySnapshot(ctx, symbol)` mengembalikan `CompanySnapshot{Company, Prices, Income, Balance, CashFlow}` — semua endpoint Alpha Vantage (OVERVIEW, TIME_SERIES_DAILY, INCOME_STATEMENT, BALANCE_SHEET, CASH_FLOW) dalam SATU pemanggilan; kegagalan endpoint mana pun menggagalkan snapshot.
- Why: spec II.1 provider abstraction + ingestion wajib log sukses/gagal per run. Satu snapshot = satu run yang atomik, status integrity terjaga; alternatif per-endpoint menambah state complexity tanpa kebutuhan nyata saat ini.
- Note: CASH FLOW DI-FETCH sejak Phase 3. Tabel `cash_flow_statements` sudah ada dari migration Phase 2 (000001) dan dipakai endpoint `/cash-flow` di Phase 5, jadi gap sengaja ditutup sekarang (bukan ditunda) supaya Phase 5 tinggal query, tidak balik ke ingestion. Amplop CASH_FLOW diparse via `fetchStatements`+`parseCashFlow` (operating/investing/financing/net-change), disimpan idempoten, jumlah baris dicatat di `ingestion_runs.cash_flow_inserted`.
- Revisi (2026-09-15): keputusan awal yang menunda cash flow DIKOREKSI atas permintaan user karena alasan sebelumnya keliru (mengira tak ada consumer padahal Phase 5 sudah menetapkan endpoint `/cash-flow` dan tabel sudah ada sejak Phase 2). Tidak ada open item tersisa dari revisi ini.
- Note: money disimpan sebagai float64 (`NUMERIC` di Postgres menerimanya); skala ini tidak sensitif desimal bank.

### 17. Ingestion audit: ingestion_runs + ON CONFLICT DO NOTHING (Phase 3, 2026-09-15)
- Chosen: tabel `ingestion_runs` (migration 000002) mencatat setiap run sukses/gagal + jumlah baris ter-insert + error; seluruh insert data idempoten via `ON CONFLICT DO NOTHING` (company di-upsert via `ON CONFLICT (symbol) DO UPDATE ... RETURNING id`).
- Why: spec II.2 (proses ingestion tercatat, sukses/gagal terlihat); re-ingest data yang sudah ada = run sukses dengan hitungan 0, tidak duplikat.
- Note: protokol validasi di trust boundary (symbol regex `^[A-Z0-9.]{1,10}$`, harga non-negatif + high>=low, fiscal date non-zero, period annual|quarterly); baris invalid dibuang, bukan menghentikan run (kecuali tidak ada satu pun harga valid -> run failure). IngestionRun menambahkan kolom `cash_flow_inserted`.
- Revisi (2026-09-15): field `cash_flow_inserted` ditambahkan ke model IngestionRun saat koreksi cash flow; tabel `ingestion_runs` belum di-deploy ke env manapun jadi tidak butuh migrasi baru.

### 18. Error wrapping: double %w agar errors.Is bisa menjangkau context.DeadlineExceeded (Phase 3, 2026-09-15)
- Temuan empiris (bukan asumsi): `http.Client.Do` mengembalikan `*url.Error` yang membungkus `context.deadlineExceededError`, dan `errors.Is(doErr, context.DeadlineExceeded)` bernilai TRUE. Namun wrapper lama `fmt.Errorf("%w: %v", ErrProviderFailed, err)` memasukkan error asli hanya sebagai TEKS pesan (%v), bukan ke unwrap chain — akibatnya `errors.Is(err, context.DeadlineExceeded)` pada error akhir selalu false. Kesimpulan bahwa "Go HTTP client tidak mengekspos sentinel" yang sempat ditulis di test terlalu dini adalah SALAH setelah dibuktikan dengan debug; akar masalah adalah wrapper sendiri.
- Fix: `fmt.Errorf("%w: %w", ErrProviderFailed, err)` (Go 1.20+ mendukung multiple %w) sehingga chain menyimpan KEDUA sentinel. Test `TestFetchCompanySnapshotTimeout` sekarang meng-assert `errors.Is(err, context.DeadlineExceeded)` sebagai bukti paling langsung bahwa kegagalan memang karena deadline, plus `errors.Is(err, ErrProviderFailed)` sebagai kontrak provider layer.

## GUIDANCE (berlaku semua phase ke depan, bukan kasus tertutup)

Aturan error wrapping untuk layer baru (khususnya analytics/service di Phase 4-5):
1. Kalau error hasil wrap masih perlu di-match lewat `errors.Is` / `errors.As` di rantai atas, gunakan `%w`, BUKAN `%v`. Format `%w: %v` memasukkan error asli hanya sebagai teks pesan dan memutus unwrap chain — class of bug yang terbukti nyata di proyek ini (#18), bukan sekadar teori.
2. Jangan asumsikan HTTP client / library tidak mengekspos sentinel (mis. context.DeadlineExceeded). Sebelum meng-claim, buktikan dengan men-dump unwrap chain (`errors.Is` per level) seperti di #18 — biasanya `%w` di wrapper sendiri, bukan library, yang menimbun chain.
3. Golden rule: `%v` hanya untuk detail diagnosis yang sengaja TIDAK ingin bisa di-match (mis. status code lengkap); kalau ragu, `%w` lebih aman — error message tetap terbentuk dari kedua nilai.
4. Hint kapan `%w: %v` dipertanyakan: message error yang panjang (`: ...`) tapi rantai errors.Is pendek.
