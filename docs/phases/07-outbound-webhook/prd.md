# Phase 7 — Outbound Webhook · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 8.4 (migration), 5 ketentuan #4 (`jobs` + worker), 3.4 (Webhook → Phase 7) · [`product/decisions.md`](../../product/decisions.md) §15 (syarat keamanan webhook), §25 (retry detail sengaja belum diputuskan) · [ADR-012](../../decisions/ADR-012-connect-surface-and-subscription-gating.md) (`/connect/webhook` sebagai kanal ketiga)

---

## Tujuan

**Data berhenti menjadi jalan satu arah.**

Phase 4 dan Phase 6 membuat lead bisa **masuk** tanpa developer. Tapi begitu masuk, ia berhenti di
sini. Pemilik toko yang juga memakai spreadsheet stok, aplikasi akuntansi, atau grup WhatsApp tim
harus membuka Jualin dan menyalin ulang — setiap kali. CRM menjadi tempat data mengendap, bukan
tempat data mengalir.

Phase 7 membalik arahnya: **saat sesuatu terjadi di Jualin, sistem lain diberi tahu sendiri.**

> Ini phase pertama di mana **kita yang memanggil**, bukan yang dipanggil. Seluruh permukaan jaringan
> produk ini sampai sekarang bersifat masuk — kita menerima request dan memutuskan menolaknya. Di sini
> pelanggan memberi kita URL, dan server kita meneleponnya. Konsekuensi keamanannya terbalik arah dan
> lebih berat: lihat *Kebutuhan #4* dan keputusan **D6**.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Owner | Sistem lain saya tahu ada lead baru **tanpa saya memberitahunya** | Saya sudah punya alat yang dipakai tim; menyalin ulang lead ke sana setiap hari adalah pekerjaan yang seharusnya tidak ada |
| 2 | Owner | Tahu kalau pengiriman **gagal** | Integrasi yang diam-diam mati lebih berbahaya daripada integrasi yang tidak pernah ada — saya berhenti mengecek Jualin karena mengira sistem lain sudah tahu |
| 3 | Owner | Mengirim ulang pengiriman yang gagal | Endpoint saya sempat mati satu jam; saya tidak mau kehilangan lead hari itu selamanya |
| 4 | Pemilik sistem | URL yang diketik pelanggan **tidak bisa** dipakai menyerang infrastruktur kita | Pelanggan mengendalikan tujuan panggilan yang **server kita** lakukan. Ini SSRF, dan ia bukan risiko teoretis — `169.254.169.254` mengembalikan kredensial cloud di hampir setiap penyedia |
| 5 | Penerima webhook | Memastikan request benar-benar dari Jualin | URL webhook pada akhirnya terekspos; tanpa tanda tangan, siapa pun yang menebaknya bisa menyuntik lead palsu ke sistem pelanggan |
| 6 | Pemilik sistem | Endpoint pelanggan yang mati **berhenti dicoba** | Di harga terjangkau, endpoint mati yang dicoba selamanya adalah biaya yang ditanggung Jualin — sama persis alasan anti-spam Phase 6 |

---

## Acceptance Criteria

Phase 7 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Owner mendaftarkan URL dari dashboard → lead baru dibuat → **request sungguhan sampai** ke URL itu, diverifikasi dari server penerima nyata, bukan dari membaca log |
| 2 | Payload membawa signature yang **bisa diverifikasi** penerima dengan secret yang ditampilkan sekali saat endpoint dibuat |
| 3 | Signature menolak payload yang diubah satu byte, dan menolak request yang diputar ulang di luar toleransi waktu |
| 4 | URL yang menunjuk ke alamat privat, loopback, atau link-local **ditolak saat disimpan** — dan tetap ditolak saat dikirim, bila DNS-nya berubah setelah tersimpan |
| 5 | Redirect **tidak pernah diikuti** |
| 6 | Endpoint yang mengembalikan `5xx` dicoba ulang dengan jeda menaik; setelah percobaan terakhir ia berhenti dan berstatus gagal permanen |
| 7 | Endpoint yang mengembalikan `4xx` **tidak** dicoba ulang — kecuali `429` |
| 8 | Pengiriman **tidak pernah** terjadi di dalam transaksi database yang memicunya (Aturan #32) |
| 9 | Membuat lead **tetap berhasil** walau endpoint webhook mati total — pengiriman gagal tidak pernah membatalkan operasi yang memicunya |
| 10 | Owner melihat riwayat pengiriman per endpoint: waktu, status, kode respons, percobaan ke berapa |
| 11 | Owner bisa memicu kirim ulang manual untuk pengiriman yang gagal, dan hasilnya terlihat di riwayat yang sama |
| 12 | Dua instance aplikasi berjalan bersamaan **tidak** mengirim pengiriman yang sama dua kali — dibuktikan di bawah konkurensi nyata, bukan diasumsikan dari `SKIP LOCKED` |
| 13 | Endpoint webhook milik organization lain **tidak** terlihat maupun bisa disunting — harness isolasi tenant bertambah kasus, dan **tetap terbukti bisa gagal** |
| 14 | `webhook_deliveries` punya retensi tertulis — ia tidak tumbuh selamanya |

---

## Keputusan yang ditutup di phase ini

Delapan keputusan yang jatuh tempo di sini, ditutup di muka daripada di tengah implementasi. Alasan
teknis lengkap di [`td.md`](./td.md); yang di bawah adalah **apa** dan **kenapa** dalam kalimat produk.

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **D1** | Antriannya tabel `jobs` generik atau `webhook_deliveries` sendiri? | **`webhook_deliveries` adalah antriannya.** Tidak ada tabel `jobs`. | Lihat catatan *"Penyimpangan tertulis dari freeze"* di bawah — satu-satunya keputusan phase ini yang menyimpang dari freeze, dan karena itu ditulis terpisah. |
| **D2** | Worker jalan di mana? | **Goroutine di dalam binary `api` yang sama**, mengambil kerja lewat `SELECT … FOR UPDATE SKIP LOCKED`. Bukan proses, bukan deployable, bukan broker. | Batasan CLAUDE.md mengikat: monolith, tanpa message broker, biaya infrastruktur per tenant harus tetap rendah. `SKIP LOCKED` membuat banyak instance aman **tanpa** leader election — Postgres yang menjamin, bukan janji kita. Preseden `FOR UPDATE` sudah ada sejak rotasi refresh token (#10). |
| **D3** | Event apa yang memicu? | **Dua**: `lead.created` dan `lead.status_changed`. | Cukup membuktikan seluruh mekanisme (antrian, retry, signature, SSRF) sekaligus benar-benar berguna — sinkronisasi pipeline, bukan sekadar pemberitahuan. Event ketiga kelak = satu pemanggil baru, bukan mekanisme baru. |
| **D4** | Bentuk signature | `X-Jualin-Signature: t=<unix>,v1=<hmac-sha256>`, di-HMAC atas `<timestamp>.<body mentah>`. Toleransi waktu 5 menit. | Bentuk yang sudah dikenal integrator (pola yang sama dipakai Stripe dan GitHub) — pilihan yang membuat dokumentasi kita lebih pendek, bukan lebih panjang. Timestamp **ikut ditandatangani**, kalau tidak ia bisa diganti dan replay protection-nya hilang. |
| **D5** | Kebijakan retry | **5 percobaan**, jeda 1m → 5m → 30m → 2j → 6j, lalu berhenti permanen. `4xx` tidak diulang, kecuali `429`. | `decisions.md` §25 sengaja menyisakan angka ini untuk diputuskan di phase-nya. Angkanya **bukan hasil pengukuran** — masuk daftar yang sama dengan angka rate limit (`api.md`). Yang **bukan** tebakan adalah bentuknya: `4xx` berarti "permintaanmu salah", mengulanginya tidak akan mengubah apa pun. |
| **D6** | Pertahanan SSRF | Validasi **dua kali** — saat disimpan dan saat dikirim — terhadap **IP hasil resolusi**, bukan hostname. Redirect **tidak pernah** diikuti. | Validasi sekali saat disimpan bisa dilewati DNS rebinding: hostname yang tadinya publik diarahkan ulang ke `127.0.0.1` setelah tersimpan. Menolak redirect sepenuhnya menghapus seluruh kelas bypass tanpa kerja tambahan (Aturan #27) — pelanggan yang butuh redirect bisa mendaftarkan URL tujuannya langsung. |
| **D7** | Kredensial keempat | **Signing secret** per endpoint, ditampilkan **sekali** saat dibuat (Aturan #21), disimpan sebagai hash. | Ini kredensial keempat produk ini, dan yang pertama yang **kita** pakai untuk membuktikan diri ke pihak lain — tiga sebelumnya untuk pihak lain membuktikan diri ke kita. Arahnya terbalik; formatnya sengaja tidak menyerupai `jln_live_` maupun `pk_`. |
| **D8** | Retensi `webhook_deliveries` | **30 hari**, dibersihkan malas tanpa scheduler — pola persis retensi `idempotency_key` (Phase 4). | Riwayat pengiriman adalah alat diagnosis, bukan catatan audit — Aturan #18 mengikat `activities` dan `audit_log`, bukan ini. Tanpa retensi ia tabel dengan pertumbuhan tercepat di produk (satu baris per lead per endpoint per percobaan). |

### Penyimpangan tertulis dari freeze — D1

`freeze.md` bagian 5 ketentuan #4 menulis: *"Tabel `jobs` + worker diperkenalkan saat ada kebutuhan
async yang **nyata**: push notification (Phase 5) atau outbound webhook (Phase 7)."*

Phase 5 **tidak** memperkenalkannya — push dibangun fire-and-forget, best-effort, tanpa antrian. Jadi
Phase 7 adalah momen yang dimaksud ketentuan itu. Tetapi phase ini tetap **tidak** membuat tabel
`jobs`, dan alasannya perlu berdiri terbuka:

| | |
|---|---|
| **Maksud ketentuan tetap dipenuhi** | Yang ketentuan #4 lindungi adalah *"jangan bangun infrastruktur async sebelum ada kebutuhan nyata"*. Phase 7 memang membangunnya — antrian tahan-crash, worker, retry berjeda, dead-letter. Yang berbeda hanya **bentuk tabelnya**. |
| **Hanya ada satu konsumen** | Email, push, dan audit semuanya fire-and-forget hari ini. Outbound webhook adalah satu-satunya kerja async di seluruh produk. Tabel `jobs` generik dengan `payload jsonb` + `job_type` untuk **satu** tipe berarti membangun abstraksi sebelum implementasi kedua yang nyata — Aturan #28, dilanggar telak. |
| **Bentuk generik justru kehilangan sesuatu** | `webhook_deliveries` butuh kolom yang `jobs` generik tidak punya tempatnya: `endpoint_id` (FK komposit tenant-scoped), `response_status`, `attempt`, `event_type`. Menyimpannya di dalam `payload jsonb` berarti riwayat pengiriman — yang **kriteria #10 wajibkan tampil di dashboard** — hanya bisa dibaca dengan menggali JSON, bukan di-query. |

**Kewajiban yang ikut lahir dari keputusan ini** — dicatat supaya tidak hilang: bila kelak lahir
konsumen async **kedua** yang nyata (laporan terjadwal, ekspor besar, pembersihan massal), saat itulah
tabel `jobs` generik dievaluasi ulang — dengan dua implementasi nyata di tangan, sebagaimana Aturan #28
memang mensyaratkan. Phase 7 tidak menutup pintu itu; ia hanya menolak membukanya lebih awal.

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| **Inbound webhook** | **Phase 7.5** — dipecah secara sadar. Inbound adalah pengulangan bentuk Phase 6 (kredensial publik, satu endpoint, verifikasi signature menggantikan honeypot); outbound adalah kelas yang sama sekali berbeda (antrian, worker, SSRF). Menggabungkannya berarti satu PRD dengan dua profil risiko yang tidak berhubungan. Bentuk payload inbound sudah diputuskan: **tetap, sama seperti API Phase 4** — pengirim menyesuaikan ke bentuk kita |
| Pemetaan payload dari platform lain | Belum terjadwal — konsekuensi langsung dari keputusan inbound di atas. Ia fitur besar tersendiri (UI pemetaan, penyimpanan aturan, mesin evaluasi), dan tidak diperlukan untuk membuktikan webhook bekerja |
| Event di luar `lead.created` & `lead.status_changed` (D3) | Kapan pun diminta — menambahnya satu pemanggil baru, bukan mekanisme baru |
| Penegakan paket, keadaan "terkunci" pada kartu Webhook | Phase 8 — ADR-012 §4. Kartu Webhook berhenti berbunyi *"belum tersedia"* di phase ini, tapi **tidak** berganti jadi *"terkunci"* |
| Tabel `jobs` generik | Saat ada konsumen async **kedua** yang nyata (D1) |
| Transformasi/penyaringan payload per endpoint | Belum ada yang memintanya. Endpoint menerima seluruh event yang ia langgan, apa adanya |
| Staging / deployment untuk QA | Terpisah dari phase manapun — tetap prasyarat sebelum Tim QA bisa bekerja |

---

## Dependensi

Phase 1–6 selesai. Yang dipakai phase ini sudah ada seluruhnya:

| Yang dipakai | Dari | Catatan |
|---|---|---|
| `leads.source` menerima `'webhook'` | Phase 2 | Dibuat di `0003` **sebelum** ada yang memakainya — hadiah yang sama seperti `'form'` di Phase 6. Tidak ada `ALTER` pada enum. Dipakai Phase 7.5 (inbound), bukan phase ini |
| `db.InTx` / `Store.InTx` per-domain | Phase 1, ADR-011 | Antrian ditulis **di dalam** transaksi yang memicunya; pengiriman terjadi **di luar** (Aturan #32) |
| `SELECT … FOR UPDATE` | Phase 1 (#10) | Rotasi refresh token sudah memakainya. Phase 7 menambah `SKIP LOCKED`, bukan mekanisme kunci baru |
| Pola bridge konsumen (`ActivityRecorder`, `LeadCreator`) | Phase 2, 6 | `lead` memicu webhook lewat interface yang **ia sendiri** deklarasikan, bukan dengan mengimpor `internal/webhook` |
| `apikey` — kredensial ditampilkan sekali, hash tersimpan | Phase 4 | Signing secret (D7) mengikuti bentuknya, dengan arah kepercayaan terbalik |
| Retensi malas tanpa scheduler | Phase 4 (`idempotency_key`) | D8 memakai pola yang sama persis |
| `crm_dashboard` — Connect, `canManageForms`/`canManageAPIKeys` | Phase 4, 6 | `/connect/webhook` menempel pada kerangka yang sudah ada; `canManageWebhooks` lahir di sebelahnya |

**Satu poin terbuka yang wajib dibaca sebelum TD final**:
`docs/issues/047-public-lead-api.md` — retensi `idempotency_key` belum pernah diuji di volume
produksi, dan **D8 memakai pola yang sama**. Bukan alasan menghindarinya, tapi jangan mewarisi
keraguannya diam-diam: kalau polanya ternyata bermasalah, ia bermasalah di dua tempat.
