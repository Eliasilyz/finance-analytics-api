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

## [P7-AUTH] Phase 7 API Key Auth & Per-Key Rate Limiting — CLOSED (2026-09-15)
- Status: **Closed**. Auth API key (X-API-Key) + per-API-key rate limiting + auth-failure flood guard per-IP, semuanya terimplementasi, unit & integration test (testcontainers) hijau, CI hijau.
- Keputusan keamanan (bcrypt → SHA-256), kebijakan rate (per-IP retired untuk data, per-key jadi primary, flood guard khusus auth-failure), dan commit list tercatat di DECISIONS.md Phase 7.
- Item [KEDEPAN] "WAJIB evaluasi pindah rate limiting per-IP → per-API-key" di bawah: **TELAH DIJALANKAN** (keputusan per-key, tercatat di DECISIONS Phase 7).

## [KEDEPAN] Diluar scope Phase 7 — belum dijadwalkan
- **SELESAI (Phase 7)**: item "WAJIB evaluasi pindah rate limiting dari per-IP ke per-API-key" — dieksekusi; hasil final per-key tercatat di DECISIONS.md Phase 7.
- Key rotation/revoke lifecycle penuh (auto-expire job), admin self-service (generate key via API/web): bukan kebutuhan Phase 7; bisa dijadwalkan sebagai fase opsional.
- Caching invalidation staleness edge case: jika Ingest sukses tapi cache.Del gagal, TTL 15m menjamin eventual consistency — tidak ada persyaratan untuk retry/queue invalidation di fase ini.


