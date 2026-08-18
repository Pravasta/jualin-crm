# ADR-010 — Fail-Fast Startup pada Kegagalan Dependensi Wajib

> **Status:** ✅ Accepted — 18 Agustus 2026
> **Konteks:** Muncul saat implementasi issue #2 ([PR #6](https://github.com/Pravasta/jualin-crm/pull/6)), dikonfirmasi eksplisit oleh pemilik produk sebagai prinsip umum, bukan keputusan lokal satu file.

## Konteks

`internal/shared/db.New` melakukan `Ping` bertimeout saat boot dan mengembalikan error bila database tidak terjangkau. `cmd/api/main.go` memperlakukan error itu sebagai fatal: log lalu `os.Exit(1)` **sebelum** HTTP server mulai menerima koneksi.

Ini ditanyakan secara eksplisit di deskripsi PR #6 sebagai keputusan yang layak dikonfirmasi, karena ada alternatif yang masuk akal — mis. server tetap start dan melapor "degraded" lewat `/health/ready` sampai DB pulih.

## Keputusan

> **Jualin CRM menggunakan fail-fast startup behavior.** Jika PostgreSQL tidak tersedia, atau inisialisasi dependensi wajib lain gagal, aplikasi **harus berhenti** dan **tidak boleh** menjalankan HTTP server.

Ini bukan aturan khusus untuk PostgreSQL. Ia berlaku untuk **setiap dependensi yang tanpanya aplikasi tidak bisa beroperasi dengan benar** — konfigurasi wajib (sudah ditegakkan sejak Phase 0, lihat `config.Load`), dan database sejak issue #2. Prinsip yang sama akan berlaku pada dependensi wajib berikutnya kecuali ada ADR baru yang mengecualikannya secara eksplisit.

## Alasan

| | |
|---|---|
| **Konsisten dengan validasi config** | `config.Load()` sudah gagal keras sejak Phase 0 (Aturan freeze — env invalid → exit sebelum server start). Membiarkan boot lanjut saat DB tidak terjangkau berarti dua kelas kegagalan wajib diperlakukan berbeda tanpa alasan. |
| **Gagal di titik yang benar** | Kegagalan saat deploy — terlihat jelas di log orchestrator, gagal sekali — lebih murah daripada kegagalan yang menyebar sebagai ratusan request individual yang gagal satu per satu setelah server "sukses" start. |
| **Tidak ada mode degraded yang aman untuk CRM** | Hampir seluruh endpoint produk ini menyentuh database (lead, membership, subscription, dst.). Server yang "hidup" tapi tidak bisa melayani permintaan apapun bukan ketersediaan — ia hanya menunda kegagalan sambil membingungkan orchestrator yang mengira proses itu sehat. |
| **Selaras dengan `docker-compose.yml`** | `depends_on: condition: service_healthy` memastikan Postgres siap sebelum `api` start pada `docker compose up` normal — jalur fail-fast ini seharusnya jarang terpicu di alur deploy yang benar, dan **memang harus** terpicu bila urutan startup rusak. |

## Yang TIDAK berubah oleh keputusan ini

**`/health/ready` tetap independen setelah boot berhasil.** Fail-fast hanya berlaku **saat startup**. Setelah server berjalan, bila Postgres mati di tengah jalan, proses **tidak** ikut mati — `/health/ready` melapor 503 sampai koneksi pulih, sementara `/health` (liveness) tetap 200 karena tidak menyentuh database. Ini dua fase yang berbeda dengan perilaku yang sengaja berbeda:

| Fase | Perilaku saat DB tidak terjangkau |
|---|---|
| **Saat boot** | Proses berhenti, HTTP server tidak pernah mulai (ADR ini) |
| **Setelah boot berhasil** | Proses tetap hidup, `/health/ready` → 503, pulih otomatis tanpa restart |

Keduanya diverifikasi manual di `docs/phases/00-foundation/notes.md` bagian #2.

## Konsekuensi

**Positif:** kegagalan infrastruktur terlihat sekali dan jelas, bukan tersebar sebagai error request · konsisten dengan pola fail-fast config yang sudah ada · orchestrator (Docker, nanti Kubernetes/systemd) mendapat sinyal exit code yang benar untuk keputusan restart/alert

**Negatif:** deployment tidak toleran terhadap Postgres yang lambat siap **tanpa** urutan startup yang benar (`depends_on`/init container/retry di level orchestrator) — tanggung jawab itu dipindah ke infrastruktur deploy, bukan diserap aplikasi

**Mitigasi:** `docker-compose.yml` sudah menegakkan urutan yang benar via healthcheck. Deployment produksi (Phase 8+) harus menerapkan pola setara — dicatat sebagai kebutuhan, bukan diimplementasikan sekarang (Aturan #27).

## Konsekuensi yang wajib ditegakkan

| # | Aturan |
|---|---|
| 1 | Setiap dependensi wajib baru (mis. kelak: cache, object storage bila menjadi wajib) mengikuti pola yang sama — gagal saat boot, bukan diam-diam berjalan dalam keadaan setengah siap |
| 2 | Endpoint liveness (`/health`) **tidak pernah** menyentuh dependensi eksternal manapun |
| 3 | Endpoint readiness (`/health/ready` dan sejenisnya) mencerminkan state **saat ini**, bukan state boot — diperiksa ulang setiap request |
| 4 | Mengecualikan sebuah dependensi dari fail-fast (mis. dependensi opsional/best-effort) memerlukan **ADR baru**, bukan penambahan diam-diam |

## Kapan dievaluasi ulang

Bila suatu saat ada dependensi yang **secara sah** boleh opsional saat boot (mis. layanan pihak ketiga yang hanya dipakai sebagian fitur, bukan seluruh produk) — itu keputusan baru yang didokumentasikan lewat ADR terpisah, bukan pengecualian diam-diam terhadap prinsip ini.
