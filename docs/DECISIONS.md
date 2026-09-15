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

## Phase 4 — Analytics Engine: DRAFT metric list & formula decisions (menunggu konfirmasi user, 2026-09-15)

Semua fungsi pure (no I/O); input disiapkan di layer service lalu di-pass sebagai parameter. Konvensi: rasio fundamental `func X(numerator, denominator float64) (float64, error)`; indikator teknikal berbasis harga `func X(prices []float64, window int) ([]float64, error)` — return series penuh, error kalau data kurang/kosong. Harga = close dari `models.DailyPrice` (service yang ekstrak).

### Valuation (section 6)
- P/E = close_price / dilutedEPS laporan tahun fiskal terakhir (annual). Bukan TTM.
  - Alasan annual vs TTM: laporan tahunan selalu lengkap & bisa dibandingkan antar-perusahaan pada periode yang sama; TTM harus menjumlahkan 4 kuartal terakhir yang waktu publikasinya beda-beda per perusahaan, menyulitkan perbandingan konsisten. Memilih annual; TTM = upgrade opsional bila user butuh.
  - eps == 0 -> error; eps < 0 -> dikembalikan sebagai P/E negatif (valid, "company rugi").
- P/B = close_price / book_value_per_share, BVPS = totalShareholderEquity / commonStockSharesOutstanding.
  - equity <= 0 atau sharesOutstanding <= 0 -> error, termasuk saat sharesOutstanding = 0 atau tidak tersedia datanya (nilai 0) — pola sama seperti rasio lain: return error, bukan panic/Inf.

### Profitability
- ROE = net income / total shareholder equity; equity <= 0 -> error (denominator negatif mengubah makna). Net income negatif valid -> ROE negatif.
- ROA = net income / total assets; total_assets <= 0 -> error.
- Net Margin = net income / total revenue; revenue == 0 -> error; revenue negatif valid (rare, dikembalikan), net income negatif valid -> margin negatif.

### Liquidity
- Current Ratio = total current assets / total current liabilities; liabilities <= 0 -> error.

### Leverage
- Debt-to-Equity = total Debt / total shareholder equity; equity <= 0 -> error (rasio tak bermakna saat ekuitas negatif/nol, bukan hanya "hasil negatif").

### Growth (annual over annual: laporan TS terakhir vs TS sebelumnya, period yang sama)
- Revenue Growth = (rev_latest - rev_prev) / rev_prev; **base <= 0 -> error** (tidak terdefinisi secara matematis, bukan dianggap 0 atau negatif).
- Earnings Growth = (ni_latest - ni_prev) / ni_prev; **base <= 0 -> error**. Pencatatan: ini wajib dibedakan dari "hasilnya negatif" — saat basis negatif growth rate tidak ada maknanya.

### Technical (harga)
- daily return = (close_t / close_{t-1}) - 1 (simple close-to-close, BUKAN log return). Series dari index 1.
- SMA 20/50/200 = mean(close di window). Butuh len >= window.
- EMA = seed SMA(window) lalu EMA_t = close_t*a + EMA_{t-1}*(1-a), a = 2/(window+1). Default window 20. Formula standar Wilder-independent EMA.
- RSI (period 14, **Wilder's smoothing**): delta = close_t - close_{t-1}; gain/loss per day. Seed: simple mean gain/loss window pertama (14). Selanjutnya Wilder smoothing: avg = (prev*(n-1) + cur)/n. RS = avgGain/avgLoss; RSI = 100 - 100/(1+RS). Edge: loss==0 && gain>0 -> RSI=100; gain==0 && loss==0 (harga konstan) -> RSI=50 (netral). Butuh len >= n+1.
  - MENENTUKAN vs SMA-approach: Wilder's smoothing adalah standar de facto (standar Wilder 1978, dipakai TradingView/stockscharts); RSI versi simple-moving-average (disetaraakan dengan Wilder) menghasilkan nilai berbeda signifikan dan tidak konsisten dengan mean-reversion thresholds 30/70 yang familiar. Pilih Wilder.
- volatility = sample standard deviation (ddof=1) dari daily returns, window default 20 hari, **TIDAK di-annualized** (daily vol). Butuh len >= window+1 (karena butuh returns dalam window).
  - Alasan non-annualized vs annualized: konsisten dengan metric "daily return" di konteks yang sama (baik angka maupun skala); window pendek mencerminkan kondisi terkini; annualization (x sqrt(252)) adalah transformasi skalar sederhana yang bisa dilakukan caller bila endpoint butuh, dan mengasumsikan 252 trading day yang tak kami paksakan.
- average volume = mean(volume di window), default 20 hari. Butuh len >= window.

### Edge case lintas fungsi (DoD)
1. Pembagi nol: return error, tidak pernah 0/NaN/Inf.
2. Data < window atau slice nil/empty -> error (bukan panic, bukan hasil menyesatkan).
3. Denomator negatif yang mengubah makna (equity<=0 untuk ROE/D/E/P/B, base<=0 untuk growth) -> error.
4. Numerator negatif yang tetap bermakna (net income<0 -> margin/ROE/ROA/P/E negatif) -> dikembalikan apa adanya, tidak di-guard.
5. RSI flat-price -> 50 (netral), bukan error.

### Struktur kode (commit per concern, Phase 4)
- internal/analytics/valuation.go + valuation_test.go
- internal/analytics/profitability.go + profitability_test.go
- internal/analytics/liquidity_leverage.go + test
- internal/analytics/growth.go + growth_test.go
- internal/analytics/technical.go + technical_test.go
- Sentinel error bersama `ErrInsufficientData`, `ErrInvalidInput` (dan `ErrUndefined` untuk rasio domain-negatif) di `internal/analytics/errors.go`

## Phase 5 — REST API Layer: DRAFT endpoint design (menunggu konfirmasi user, 2026-09-15)

Semua endpoint GET, prefix /health + /api/v1. Response envelope konsisten: sukses `{"data": ...}`, error `{"error": {"code", "message"}}`. Satu helper `httpError(w, status, code, msg)` & `httpJSON(w, status, body)` di internal/handler (respond.go) — TIDAK ada handler yang bikin gaya JSON sendiri.

### Status code policy (seragam, semua endpoint)
- 400 VALIDATION_ERROR — symbol/query param invalid (regex gagal, limit negatif, dst)
- 404 NOT_FOUND — symbol tidak ada DI DATABASE, atau data resource kosong (company ada tapi 0 baris)
- 422 INSUFFICIENT_DATA — data ADA tapi tidak cukup untuk komputasi yang diminta (mis. history < 200 hari utk SMA200, fundamentals tidak ada utk analytics)
- 429 RATE_LIMITED — middleware rate limit (sedikit informasi; Redis/middleware di fase terpisah, tapi kontrak status disepakati sekarang)
- 500 INTERNAL — DB error / panic tak terduga; TIDAK pernah dipakai untuk data kurang (itu 422)

### Validasi input (shared, satu implementasi)
- `symbol`: regex `^[A-Z0-9.]{1,10}$` — SAMA dengan yang dipakai ingestion. Diekspor dari service (`service.IsValidSymbol`) lalu dipakai handler, tidak duplikat literal.
- `/compare?symbols=`: split koma, tiap item di-Trim lalu divalidasi SATU-PERSATU dengan regex yang sama. Maksimal 10 simbol. Kosong/duplikat/invalid/setelah-trim-kosong -> 400 (jangan percaya mentah hasil split).
- Query param numerik (limit, min/max screener): wajib parse-able; gagal -> 400.

### Endpoint & shape
1. **GET /health** -> 200 `{"status":"ok"}`. Liveness sederhana, tanpa DB hit.
2. **GET /api/v1/companies** -> 200 `{"data":[{"symbol","name","exchange","sector","industry","currency"}]}`. Kosong = 200 `[]`.
3. **GET /api/v1/companies/:symbol** -> 200 `{"data":{company}}`. 400 symbol invalid; 404 symbol tak ditemukan.
4. **GET /api/v1/stocks/:symbol/history** -> 200 `{"data":[{"date","open","high","low","close","volume"}]}` (urut date ASC). Optional `?limit=n` (1..1000), validasi; default = semua. 404 company/none; 400 limit invalid.
5-7. **GET /api/v1/stocks/:symbol/{income-statement,balance-sheet,cash-flow}** -> `?period=annual|quarterly|all` (default annual). 200 `{"data":[...]}`. Invalid period -> 400; tak ada data -> 404.
8. **GET /api/v1/stocks/:symbol/analytics** -> 200 `{"data":{"symbol","price":latest_close,"metrics":{"pe","pb","roe","roa","net_margin","current_ratio","debt_to_equity","revenue_growth","earnings_growth"}}}`. Setiap metric nullable (`null`) saat matematis undefined (dll. ErrUndefined), yg lain tetap terisi. 404 company tak ada/fundamentals kosong total -> 404; company ada tapi income/balance kosong -> 422 INSUFFICIENT_DATA.
9. **GET /api/v1/stocks/:symbol/technical** -> 200 `{"data":{"symbol","last_date","values":{"daily_return","sma20","sma50","sma200","ema20","rsi14","volatility20","average_volume20"}}}` (nilai terbaru tiap indikator). 404 company tak ada; **history < 200 hari -> 422 INSUFFICIENT_DATA dengan message "need >=200 days, got N"** (SMA200 = window terpanjang, constraint paling ketat).
10. **GET /api/v1/compare?symbols=AAPL,MSFT** -> 200 `{"data":[{"symbol","metrics":{...}}]}`. Tiap simbol dihitung pakai alur analytics yang sama. 400 bila simbol tak valid/lebih dari 10/dup; 404 bila ada simbol tak ditemukan; 422 bila ada simbol tanpa data cukup.
11. **GET /api/v1/screener?min_roe=0.15&max_pe=30&sector=Technology** -> filtering di SQL (section 8 INTRUCT): 200 `{"data":[{"symbol","name","sector","roe","pe"}]}`. Param opsional `sector` (eksak), `min_roe`, `max_pe`, `min_revenue_growth`; invalid numeric -> 400; company ditemukan oleh kriteria -> 200 (bisa kosong).

### Handler -> service -> analytics (alur wajib)
- Handler: parse/validasi input, panggil service, render envelope. TIDAK hit DB/analytics langsung.
- Service: `QueryService` baru di internal/service/query.go dengan read methods (ListCompanies, GetCompany, GetPriceHistory, GetIncomeStatements, GetBalanceSheets, GetCashFlows, GetAnalytics, GetTechnical, Compare, Screener). Service yang fetch data dari repository, lalu **melempar data sebagai parameter** ke `analytics.*` (pure functions Phase 4). Contoh: service ambil price history penuh (perlu >= 201) untuk technical, hitung di sana, bukan handler.
- Repository: tambah read methods (GetCompany, ListCompanies, GetPriceHistory, GetRecentStatements per period, GetLatestPrice, dan method gabungan utk screener) pada interface `QueryStore` baru (diimplementasikan Repository; di-fake utk unit test, pola sama seperti DataStore Phase 3).
- **Prerequisite fix**: `InsertBalanceSheets` saat ini TIDAK menyimpan `shares_outstanding` (schema punya kolomnya, model punya field-nya, INSERT 9 kolom melewatkannya) -> P/B butuh argumen ini. Commit fix di awal Phase 5.

### Commit plan (per concern, tidak digabung)
1. fix(repository): persist shares_outstanding
2. feat(handler): shared respond helper + error envelope + service.IsValidSymbol export
3. feat(repository): read methods (QueryStore) + unit test
4. feat(service): QueryService + unit test (fake store)
5. feat(handler): companies endpoints (+ test)
6. feat(handler): stocks data endpoints: history/income/balance/cash-flow (+ test)
7. feat(handler+service): analytics & technical endpoints (SMA200 gap -> 422) (+ test)
8. feat(handler+service): compare (+ test)
9. feat(handler+service): screener SQL filtering (+ test)
10. integration test handler di test/integration (pakai testcontainers yg ada), lalu close phase.

Catatan non-scope fase ini: Redis caching sebelum analytics (section 9), auth/rate-limit middleware aktif (kontrak 429 disepakati). {: .no_match}
