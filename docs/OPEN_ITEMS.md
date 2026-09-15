# Open Items

## [P1-DOCKER-SMOKE] Phase 1 Docker Compose smoke test — CLOSED
- Status: **Closed** (2026-09-15)
- Verifikasi: `docker compose up --build -d` sukses; `GET /health` -> 200 `{"status":"ok"}`; ketiga container (api, postgres, redis) healthy. `docker compose down -v` bersih.
- Catatan: smoke test jatuh ke hari yang sama dengan verifikasi Phase 2 karena Docker daemon baru tersedia setelah Docker Desktop dinyalakan.

## [P3-INGESTION] Phase 3 ingestion pipeline — CLOSED (2026-09-15)
- Status: **Closed**. Unit + integration test green dan di-deploy ke CI; verifikasi run GitHub Actions 34917635258 (build + integration job success), termasuk TestIngestPipelineEndToEnd idempoten dan cakupan cash flow.
- Catatan: cash flow statement di-fetch sejak Phase 3 (bukan ditunda), koreksi permintaan user — lihat DECISIONS #16 revisi.

Tidak ada open item lain saat ini.

## [P5-API] Phase 5 REST API layer — CLOSED (2026-09-15)
- Status: **Closed**. Semua endpoint di INSTRUCTION §7 terimplementasi, unit test hijau, integration test (testcontainers Postgres) hijau, CI hijau di run GitHub Actions 34934279881 (commit f53a18d; build + integration job sukses).
- Yang dikerjakan: controller `internal/handler` (companies, stocks data, analytics, technical, compare, screener + envelope error), service query layer, repository read/screener, wiring ke `cmd/api/main.go`.
- Keputusan desain (partial-null /technical, all-or-nothing /compare, parameterized /screener) tercatat di DECISIONS.md Phase 5.

## [P6-API] Phase 6 Redis Caching & Rate Limiting — CLOSED (2026-09-15)
- Status: **Closed**. Redis read-through cache untuk /analytics + /technical (TTL 15m, invalidasi aktif saat Ingest sukses), fixed-window rate limiter per IP (429 RATE_LIMITED), keduanya fail-open jika Redis unreachable. Unit test + integration test (testcontainers Redis + Postgres) hijau; CI hijau.
- Kebijakan fail-open, counter semantics (semua /api/v1 request dihitung, /health dikecualikan), env config tercatat di DECISIONS.md Phase 6.

## [KEDEPAN] Diluar scope Phase 5 — belum dijadwalkan
- Phase 7 auth (JWT/API key) — saat auth aktif, evaluasi ulang rate limiting dari per-IP ke per-API-key (satu key bisa numpuk banyak client di belakang NAT/proxy; satu client bisa ganti IP untuk hindari limit).
- Caching invalidation staleness edge case: jika Ingest sukses tapi cache.Del gagal, TTL 15m menjamin eventual consistency — tidak ada persyaratan untuk retry/queue invalidation di fase ini.


