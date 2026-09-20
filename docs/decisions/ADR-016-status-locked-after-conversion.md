# ADR-016 — Status Lead Terkunci Setelah Dikonversi

> **Status:** ✅ Accepted — 21 September 2026
> **Berlaku sejak:** issue #142
> **Mengamandemen:** [ADR-006](./ADR-006-lead-status-as-pipeline.md) aturan turunan #3 (transisi keluar dari `won`) · `docs/phases/02-crm-core/td.md` §5 · **menegakkan** `td.md` §10 (*"statusnya tidak berubah. Ia tetap `won`"*)
> **Errata:** ADR-006 tentang kolom `converted_*` di lead — lihat *Errata* di bawah
> **Tidak mengubah** rumus `conversion_rate`, aturan konversi (`POST /v1/leads/{id}/convert`), atau field lead selain `status`

## Konteks

Lead **Menang** yang sudah **dikonversi jadi Customer** masih bisa diubah statusnya: Menang → Konversi → **Kembali ke Penawaran**, atau **Kalah / Tidak Memenuhi Syarat / Spam**. Ditemukan pemilik produk saat uji manual. Bukan bug kode — aturan transisi berjalan sesuai teks.

`validateStatusTransition` tidak punya perlakuan khusus untuk `won`; dua aturan umum meloloskannya:

- **Mundur satu langkah** (ADR-006 #3) → `won → proposal`
- **Terminal samping dapat dicapai dari status mana pun di jalur utama** (`td.md` §5) → `won → lost / unqualified / spam`

Sama seperti `new` di [ADR-015](./ADR-015-nothing-leads-back-to-new.md): aturan ditulis umum, diilustrasikan dengan kasus yang masuk akal, dan tidak pernah diperiksa untuk `won`. Ditambah, `UpdateStatus` tidak punya penjaga apa pun untuk lead yang sudah dikonversi.

**Dampak:**

1. Customer tetap ada, tetapi lead-nya berkata "Penawaran" atau "Kalah" — dua fakta yang saling bertentangan.
2. **`conversion_rate` ikut turun.** Rumusnya `count(status = 'won') / (total − spam − unqualified)` — dihitung dari **status lead**, bukan dari Customer. ADR-006 menyebut angka ini *"alasan owner membayar"*.
3. Bertentangan dengan `td.md` §10, yang sudah menyatakan lead tetap `won` setelah konversi. Kalimat itu menggambarkan yang dilakukan **konversi**; tak ada yang menegakkannya sesudahnya.

## Keputusan

**Status lead terkunci begitu ia dikonversi.** `PATCH /v1/leads/{id}/status` pada lead yang sudah punya Customer → **`422 lead_converted_locked`**, untuk **setiap** status tujuan, tanpa mengubah apa pun.

**Sebelum dikonversi, `won` tetap bisa mundur atau ditutup.** Salah klik "Menang" masih bisa dikoreksi. Kuncinya ditaruh tepat di tindakan yang benar-benar tidak bisa dibatalkan — konversi — bukan pada `won` itu sendiri.

**Hanya `status` yang terkunci.** Mengubah nama/kontak, penugasan, task, dan catatan pada lead terkonversi tidak disentuh.

**Kode error `422 lead_converted_locked`, bukan `invalid_status_transition` dan bukan `409`.** 422 karena permintaannya sah bentuknya dan keadaan lead yang melarangnya (sama dengan `invalid_status_transition`); kode sendiri karena UI harus menjelaskan *kenapa* — pesan generik "transisi tidak diizinkan" mengirim pengguna mencari tombol yang tidak ada. Bukan 409: dashboard membaca `409 version_conflict` sebagai "muat ulang dan coba lagi", persis salah di sini. Pemeriksaannya mendahului validasi transisi, karena "lead ini sudah dikonversi" adalah jawaban yang dibutuhkan untuk tujuan apa pun, termasuk yang juga ditolak aturan transisi.

UI (dashboard dan mobile) **tidak menawarkan** tombol status untuk lead terkonversi dan menjelaskan kenapa dengan satu kalimat — bukan ditawarkan lalu ditolak. Sinyalnya entri `lead_converted` di timeline, karena lead tidak punya penanda apa pun (lihat *Errata*). Backend tetap otoritasnya: layar yang basi tetap mendapat `lead_converted_locked`.

## Mekanisme — dan kenapa satu `UPDATE … WHERE NOT EXISTS` tidak cukup

Bagian ini didokumentasikan karena ia yang paling mudah salah, dan alasannya tidak terlihat dari kode.

`customer.Convert` adalah `INSERT … SELECT FROM leads WHERE status = 'won'`, dan konversi **tidak menaikkan `leads.version`** (lead "tidak pernah berubah oleh konversi", #23). Maka optimistic locking `version` **tidak** melindungi konversi dari perubahan status yang bersamaan. Sebelum ADR ini, `Convert` juga tidak mengunci baris lead. Dua request yang berpapasan bisa menghasilkan Customer **dan** lead ber-status non-`won` — persis keadaan yang hendak dicegah, dan terbukti bisa terjadi (lihat *Verifikasi*).

Pemeriksaan "sudah dikonversi?" yang hanya berupa `SELECT` di awal transaksi tidak menutupnya. Yang benar:

1. **Kedua sisi mengunci baris lead yang sama.** `UpdateStatus` memakai `SELECT … FOR UPDATE` (`FindByIDForUpdate`), `Convert` memakai `FOR UPDATE` di `SELECT`-nya. Salah satu selalu menunggu yang lain.
2. **Pemeriksaan terkonversi adalah statement terpisah, sesudah kunci.** Di `READ COMMITTED` tiap statement mengambil snapshot baru, sehingga customer yang di-commit `Convert` selama kita menunggu terlihat. **Melipatnya ke dalam `UPDATE … WHERE NOT EXISTS (SELECT 1 FROM customers …)` tidak cukup**: setelah menunggu kunci baris, kondisi baris itu dievaluasi ulang, tetapi subquery ke tabel lain tetap membaca snapshot **awal statement**.
3. Urutan sebaliknya (perubahan status lebih dulu): `Convert` yang menunggu mengevaluasi ulang `status = 'won'` pada baris yang sudah berubah, tidak menemukan apa-apa, dan mengembalikan `invalid_status_transition`.

Tidak ada siklus kunci: `Convert` mengunci baris lead lalu menyisipkan customer (FK komposit hanya mengambil `FOR KEY SHARE` atas baris lead yang sama); `UpdateStatus` mengunci baris lead lalu meng-`UPDATE`-nya. Tak ada pihak yang mengunci customer lalu meminta lead.

`IsConverted` **membaca tabel `customers` langsung** dari repository lead — bukan lewat interface baru yang dijembatani di composition root. Repository customer sudah membaca `leads` dengan cara yang sama di `Convert`; menambah interface, implementasi, dan wiring untuk satu `EXISTS` adalah ceremony (Aturan #27–29). `internal/lead` tetap tidak mengimpor `internal/customer`.

## Errata — ADR-006 berselisih dengan skema

ADR-006 menulis bahwa `converted_customer_id` dan `converted_at` "sudah ada sejak awal" di lead. **Tabel `leads` tidak punya kolom `converted` apa pun** (`migrations/0003_crm_core.sql`); `converted_at` hanya ada di `customers`. Satu-satunya penanda adalah `customers.converted_from_lead_id` (dengan `uq_customers_org_lead`) dan activity `lead_converted`. Kode yang menang (Aturan #30). Konsekuensi untuk rencana perpindahan ke Deal (ADR-006): backfill `won → converted` harus mengambil sumbernya dari `customers`, bukan dari lead.

## Alternatif yang ditolak

1. **`won` final** (seperti `unqualified` dan `spam`). Paling sederhana dan sesuai niat `td.md` §10, tetapi salah klik "Menang" tidak bisa dibatalkan dan angka konversi tercemar selamanya — kerugian yang lebih besar dari kasus `new`, karena `won` adalah keadaan yang paling berpengaruh pada metrik dan membuka tombol Konversi.
2. **Hitung `conversion_rate` dari tabel `customers`.** Menyelesaikan metriknya, tetapi kontradiksi antara Customer dan status lead tetap ada, dan lead "Kalah" yang punya Customer tetap muncul.
3. **Kunci lewat `UPDATE … WHERE NOT EXISTS`** — ditolak karena tidak benar di bawah konkurensi (lihat *Mekanisme*).

## Yang tidak dilindungi — terus terang

- **Customer yang dihapus (soft delete) tidak membuka kunci.** `uq_customers_org_lead` bukan indeks parsial, sehingga customer terhapus tetap menghalangi konversi ulang; kunci status ikut konsisten dengan itu. Kalau tidak, menghapus customer diam-diam membuka lead yang tak bisa dikonversi lagi.
- **`lead.Delete` tidak menjaga lead terkonversi** (`UPDATE leads SET deleted_at = now()` tanpa pemeriksaan), padahal `td.md` §10 menyatakan lead itu tidak dihapus dan timeline-nya adalah jejak bagaimana pelanggan itu didapat. Kelas masalah yang sama, keputusan yang berbeda — **tidak dicakup ADR ini**.
- **Lead yang sudah tidak konsisten sebelum kunci ini ada** (dikonversi lalu dimundurkan) tidak diperbaiki otomatis; memilih status yang benar untuk tiap lead bukan keputusan kode. Pada database dev saat ADR ini ditulis: 1 lead terkonversi, 0 yang tidak konsisten.

## Verifikasi

Balapan diuji di bawah konkurensi asli terhadap Postgres sungguhan lewat router produksi (`cmd/api/lead_conversion_lock_test.go`), dua lapis: **deterministik** (satu transaksi dibiarkan terbuka sebagai `Convert` atau perubahan status yang belum commit, lalu dibuktikan lewat `pg_stat_activity` bahwa sisi lain *benar-benar menunggu kunci*, dan diuji **kedua urutan**) dan **stres** (40 pasangan dilepas bersamaan). Setiap sisi dibuktikan bisa gagal: `FOR UPDATE` dicabut dari `Convert` → tes urutan kedua dan tes stres merah; kunci dicabut dari `UpdateStatus` → tes urutan pertama dan tes stres merah. Bahwa tes stres ikut merah dan gagal cepat (0,06 detik dari 40 pasangan yang dijadwalkan) menunjukkan balapannya nyata dan mudah dipancing, bukan hanya teori.

## Konsekuensi

**Positif:** Customer dan status lead tidak lagi saling bertentangan · `conversion_rate` berhenti bergeser karena lead terkonversi dimundurkan · `td.md` §10 kini benar-benar ditegakkan · salah klik "Menang" sebelum konversi tetap bisa dikoreksi.

**Negatif:** lead terkonversi yang statusnya keliru tidak bisa dikoreksi lewat aplikasi (perlu intervensi data) · `UpdateStatus` kini memegang kunci baris dan satu query tambahan, dan `Convert` ikut mengunci — dua operasi pada lead yang sama kini berurutan, bukan berpapasan.

**Kapan dievaluasi ulang:** bila pengguna nyata memerlukan mengoreksi status lead terkonversi — jawabannya jalur koreksi yang eksplisit (mis. membatalkan konversi dan menuliskannya di timeline), bukan mencabut kunci ini.
