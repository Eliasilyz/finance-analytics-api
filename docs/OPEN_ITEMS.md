# Open Items

## [P1-DOCKER-SMOKE] Phase 1 Docker Compose smoke test — CLOSED
- Status: **Closed** (2026-09-15)
- Verifikasi: `docker compose up --build -d` sukses; `GET /health` -> 200 `{"status":"ok"}`; ketiga container (api, postgres, redis) healthy. `docker compose down -v` bersih.
- Catatan: smoke test jatuh ke hari yang sama dengan verifikasi Phase 2 karena Docker daemon baru tersedia setelah Docker Desktop dinyalakan.

## [P3-INGESTION] Phase 3 ingestion pipeline — CLOSED (2026-09-15)
- Status: **Closed**. Unit + integration test green dan di-deploy ke CI; verifikasi run GitHub Actions 34917635258 (build + integration job success), termasuk TestIngestPipelineEndToEnd idempoten dan cakupan cash flow.
- Catatan: cash flow statement di-fetch sejak Phase 3 (bukan ditunda), koreksi permintaan user — lihat DECISIONS #16 revisi.

Tidak ada open item lain saat ini.
