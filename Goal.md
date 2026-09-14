\### Goal



Membangun sebuah \*\*Financial Data Analytics API\*\* yang mengambil data keuangan dari API eksternal, menyimpan dan mengelolanya sendiri, lalu menyediakan data dan hasil analisis melalui REST API.



Project ini dibuat untuk menunjukkan kemampuan backend secara nyata, bukan sekadar membuat CRUD.



Fokus utamanya adalah:



\* mengambil data finansial dari sumber eksternal

\* menyimpan data secara terstruktur di PostgreSQL

\* melakukan perhitungan financial dan technical metrics sendiri

\* menyediakan endpoint REST API yang jelas

\* menggunakan Redis untuk caching dan rate limiting

\* menangani error dan data yang tidak valid

\* memiliki authentication dan validasi input

\* memiliki unit test dan integration test

\* menggunakan Docker agar environment mudah dijalankan

\* menggunakan CI untuk menjalankan test dan pemeriksaan kode secara otomatis

\* menyediakan dokumentasi API dengan OpenAPI



Hasil akhirnya adalah sebuah backend yang bisa digunakan aplikasi lain untuk \*\*mengambil, membandingkan, menyaring, dan menganalisis data perusahaan dan saham\*\* tanpa harus berkomunikasi langsung dengan provider data eksternal.



