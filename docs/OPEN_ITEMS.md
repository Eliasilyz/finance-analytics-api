# Open Items

Open item ini harus diverifikasi sebelum phase terkait dianggap fully closed.

## [P1-DOCKER-SMOKE] Phase 1 Docker Compose smoke test BELUM dijalankan
- Status: Open
- Dicatat: 2026-09-15
- Detail:
  - Phase 1 Docker Compose smoke test (docker compose up + GET /health -> 200) belum dijalankan.
  - Penyebab: Docker daemon tidak tersedia di environment development ini (Windows; Docker Desktop terinstall tapi daemon tidak berjalan).
  - Yang sudah diverifikasi: docker compose config valid, go build ./..., go vet ./..., go fmt green, image Dockerfile masuk CI.
  - Update: Docker daemon sejak itu berhasil dinyalakan (Docker Desktop). Phase 2 integration test via testcontainers sudah diverifikasi jalan lokal. Smoke test docker compose up tetap belum dieksekusi.
- Syarat close:
  1. Docker daemon tersedia (local atau CI runner).
  2. docker compose up --build -d sukses.
  3. GET /health mengembalikan HTTP 200.
  4. Ditandai verified di laporan progress.
- Tenggat: Sebelum mulai Phase 8. Wajib disebutkan statusnya di setiap laporan progress phase.
