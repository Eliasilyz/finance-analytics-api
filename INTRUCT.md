Untuk Financial Data Analytics API, kira-kira levelnya begini:



\### 1. Tujuan project



Project ini adalah backend API yang mengambil data perusahaan dan harga saham dari provider eksternal, menyimpan data tersebut ke database, lalu mengolahnya menjadi informasi yang bisa digunakan aplikasi lain.



API ini bukan sekadar meneruskan response dari Alpha Vantage. Data akan disimpan, dinormalisasi, dihitung ulang, dan disediakan melalui endpoint milik project sendiri.



Jadi alurnya:



`External API → Ingestion → PostgreSQL → Analytics → Redis → REST API`



\### 2. Data yang digunakan



Gunakan Alpha Vantage sebagai sumber data awal.



Data yang diambil bisa mencakup:



\-   informasi perusahaan

&#x20;   

\-   harga saham harian

&#x20;   

\-   volume

&#x20;   

\-   laporan income statement

&#x20;   

\-   balance sheet

&#x20;   

\-   cash flow

&#x20;   



Jangan mengambil semuanya sekaligus. Mulai dari data yang benar-benar dipakai oleh fitur.



Alpha Vantage hanya berfungsi sebagai sumber data. User API tidak seharusnya langsung bergantung pada Alpha Vantage setiap kali meminta data.



\### 3. Backend



Gunakan Go untuk backend.



Alasannya sederhana: project ini memang ingin menunjukkan kemampuan backend engineering, bukan sekadar membuat CRUD menggunakan framework.



Backend bertugas menangani:



\-   HTTP request

&#x20;   

\-   validasi input

&#x20;   

\-   business logic

&#x20;   

\-   komunikasi database

&#x20;   

\-   pengambilan data eksternal

&#x20;   

\-   perhitungan analytics

&#x20;   

\-   caching

&#x20;   

\-   authentication

&#x20;   

\-   rate limiting

&#x20;   

\-   error handling

&#x20;   



Struktur kode harus memisahkan HTTP handler, business logic, database, provider eksternal, dan analytics.



\### 4. Database



Gunakan PostgreSQL.



Data utama disimpan di PostgreSQL karena data finansial memiliki hubungan yang jelas.



Contohnya:



`companies`



menyimpan informasi perusahaan.



`price\_history`



menyimpan harga saham berdasarkan tanggal.



`income\_statements`



menyimpan laporan pendapatan.



`balance\_sheets`



menyimpan kondisi neraca.



`cash\_flow\_statements`



menyimpan laporan arus kas.



Database juga harus mempunyai primary key, foreign key, unique constraint, dan index yang memang diperlukan.



Jangan membuat 25 tabel cuma supaya kelihatan enterprise. Database juga bisa menjadi tempat manusia menyimpan dosa desain.



\### 5. Ingestion



Ingestion adalah proses mengambil data dari provider lalu memasukkannya ke database.



Contohnya:



`AAPL → Alpha Vantage → validasi → normalisasi → PostgreSQL`



Proses ini tidak dilakukan setiap kali user memanggil endpoint.



Misalnya data AAPL hari ini sudah disimpan, request berikutnya cukup mengambil data dari PostgreSQL.



Ingestion juga harus menangani kondisi seperti:



\-   API provider gagal

&#x20;   

\-   response tidak lengkap

&#x20;   

\-   data invalid

&#x20;   

\-   request terkena rate limit

&#x20;   

\-   database gagal

&#x20;   

\-   data sudah pernah disimpan

&#x20;   



Setiap proses ingestion sebaiknya dicatat supaya kita tahu proses tersebut berhasil atau gagal.



\### 6. Analytics



Ini bagian yang membuat project lebih menarik daripada sekadar database + CRUD.



Backend menghitung metric berdasarkan data yang sudah disimpan.



Contohnya:



\*\*Valuation\*\*



\-   P/E

&#x20;   

\-   P/B

&#x20;   



\*\*Profitability\*\*



\-   ROE

&#x20;   

\-   ROA

&#x20;   

\-   Net Margin

&#x20;   



\*\*Liquidity\*\*



\-   Current Ratio

&#x20;   



\*\*Leverage\*\*



\-   Debt-to-Equity

&#x20;   



\*\*Growth\*\*



\-   Revenue Growth

&#x20;   

\-   Earnings Growth

&#x20;   



Untuk data harga saham, bisa ditambahkan:



\-   daily return

&#x20;   

\-   SMA 20

&#x20;   

\-   SMA 50

&#x20;   

\-   SMA 200

&#x20;   

\-   EMA

&#x20;   

\-   RSI

&#x20;   

\-   volatility

&#x20;   

\-   average volume

&#x20;   



Perhitungan dilakukan oleh aplikasi sendiri berdasarkan data yang tersedia, bukan sekadar meminta hasilnya dari AI.



\### 7. REST API



Endpoint awal tidak perlu terlalu banyak.



Contohnya:



```text

GET /health



GET /api/v1/companies

GET /api/v1/companies/:symbol



GET /api/v1/stocks/:symbol/history

GET /api/v1/stocks/:symbol/income-statement

GET /api/v1/stocks/:symbol/balance-sheet

GET /api/v1/stocks/:symbol/cash-flow



GET /api/v1/stocks/:symbol/analytics

GET /api/v1/stocks/:symbol/technical



GET /api/v1/compare?symbols=AAPL,MSFT,GOOGL



GET /api/v1/screener

```



Contoh:



```text

GET /api/v1/stocks/AAPL/analytics

```



Response-nya berisi hasil analytics untuk AAPL.



Sementara:



```text

GET /api/v1/compare?symbols=AAPL,MSFT,GOOGL

```



digunakan untuk membandingkan beberapa perusahaan.



\### 8. Screener



Screener adalah fitur untuk mencari perusahaan berdasarkan kondisi tertentu.



Misalnya:



```text

sector = Technology

ROE > 15%

PE < 30

```



Backend mengambil data dari PostgreSQL dan melakukan filtering.



Ini penting karena menunjukkan bahwa database bukan cuma tempat menyimpan data, tetapi benar-benar digunakan untuk melakukan query dan data processing.



\### 9. Redis



Redis digunakan untuk data yang sering diminta.



Misalnya user berkali-kali meminta:



```text

/api/v1/stocks/AAPL/analytics

```



Backend tidak perlu menghitung semuanya dari awal setiap kali.



Hasilnya bisa disimpan sementara:



```text

AAPL:analytics

```



Jika masih ada di Redis, backend langsung mengembalikan hasil tersebut.



Kalau cache sudah expired, backend mengambil ulang data dan membuat cache baru.



Redis juga bisa digunakan untuk rate limiting.



\### 10. Security



API harus melakukan validasi terhadap input user.



Contohnya symbol tidak boleh menerima input yang aneh atau query yang bisa merusak database.



SQL harus menggunakan parameterized query.



Secret seperti API key provider tidak boleh ditulis langsung di source code.



Kalau API menggunakan API key untuk user, key tersebut harus disimpan dengan aman dan tidak disimpan sebagai plaintext.



Rate limiting juga diperlukan supaya satu client tidak bisa membanjiri API.



\### 11. Testing



Testing jangan cuma:



> endpoint `/health` mengembalikan 200.



Itu testing level “saya menekan tombol dan ternyata lampunya nyala”.



Minimal ada tiga level.



\*\*Unit test\*\*



Menguji logic yang tidak membutuhkan database.



Contohnya:



```text

calculateROE()

calculatePE()

calculateRSI()

calculateDailyReturn()

```



Test juga harus mencakup kondisi seperti data kosong atau pembagi bernilai nol.



\*\*Handler/API test\*\*



Menguji endpoint HTTP.



Contohnya:



```text

GET /api/v1/stocks/AAPL/analytics

```



harus menghasilkan status dan response yang benar.



Test juga kondisi:



```text

200

400

404

429

500

```



sesuai kasusnya.



\*\*Integration test\*\*



Menguji bagian yang benar-benar berkomunikasi dengan PostgreSQL dan Redis.



Tujuannya memastikan kode yang terlihat benar secara unit test benar-benar bekerja ketika komponen digabung.



\### 12. External API testing



Provider eksternal jangan dipanggil langsung dalam setiap test.



Gunakan mock/fake provider.



Test beberapa kondisi:



```text

provider berhasil

provider timeout

provider mengembalikan error

provider terkena rate limit

provider memberikan data tidak valid

```



Dengan begitu kita bisa menguji bagaimana aplikasi bereaksi terhadap masalah eksternal.



\### 13. Docker



Gunakan Docker untuk menjalankan:



```text

API

PostgreSQL

Redis

```



Docker Compose bisa digunakan agar seluruh environment dapat dijalankan dengan satu konfigurasi.



Tujuannya bukan supaya README bisa menulis kata Docker dengan bangga. Tujuannya agar project mudah dijalankan di komputer lain dan environment development lebih konsisten.



\### 14. CI/CD



GitHub Actions menjalankan pemeriksaan otomatis setiap push atau pull request.



Minimal:



```text

go fmt

go vet

unit test

integration test

lint

build

docker build

```



Kalau salah satu gagal, perubahan tersebut tidak dianggap lolos CI.



\### 15. Dokumentasi



Gunakan OpenAPI/Swagger.



Setiap endpoint harus menjelaskan:



\-   method

&#x20;   

\-   URL

&#x20;   

\-   parameter

&#x20;   

\-   authentication

&#x20;   

\-   request

&#x20;   

\-   response

&#x20;   

\-   error response

&#x20;   



README juga menjelaskan architecture dan cara menjalankan project.



Bukan README:



> “This is a financial API. npm install and run.”



Itu bukan dokumentasi. Itu surat wasiat.



\### 16. Hal yang TIDAK perlu dulu



Jangan masukkan:



\-   AI prediction

&#x20;   

\-   buy/sell signal

&#x20;   

\-   chatbot

&#x20;   

\-   Kubernetes

&#x20;   

\-   microservices

&#x20;   

\-   Kafka

&#x20;   

\-   GraphQL

&#x20;   

\-   real-time trading

&#x20;   

\-   machine learning

&#x20;   



Semua itu bisa membuat project terlihat lebih besar, tetapi belum tentu membuat engineering-nya lebih bagus.



Fokusnya adalah membuktikan bahwa lu bisa membuat \*\*backend system yang datanya nyata, punya database, processing, caching, security, testing, documentation, dan CI/CD.\*\*



\### Gambaran akhirnya



Project yang kita kejar kira-kira seperti ini:



```text

&#x20;            Alpha Vantage

&#x20;                  │

&#x20;                  ▼

&#x20;            Data Ingestion

&#x20;                  │

&#x20;         Validate + Normalize

&#x20;                  │

&#x20;                  ▼

&#x20;             PostgreSQL

&#x20;             │         │

&#x20;             │         └── Financial Data

&#x20;             │

&#x20;             └── Price Data

&#x20;                  │

&#x20;                  ▼

&#x20;           Analytics Engine

&#x20;                  │

&#x20;            ┌─────┴─────┐

&#x20;            ▼           ▼

&#x20;          Redis       REST API

&#x20;                        │

&#x20;             ┌──────────┼──────────┐

&#x20;             ▼          ▼          ▼

&#x20;          History   Analytics   Screener

```



