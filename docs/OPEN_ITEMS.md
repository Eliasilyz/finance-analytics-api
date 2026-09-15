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

## [KEDEPAN] Diluar scope Phase 5 — belum dijadwalkan
- Middleware 429 RATE_LIMITED (kontrak error code sudah ada) — aktifkan saat layer rate limiting §9 dibangun.
- Caching Redis untuk respons baca (END_POINT_AND_QUERY_PER_SECOND dll dari konfigurasi) — §9, butuh redis client di service.


