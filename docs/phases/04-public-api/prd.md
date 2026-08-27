# Phase 4 — Public API · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 4 (Phase 4), 3.1 (API Key & Public Lead API), 5.1 (Aturan #24 — User App ≠ API Key), 8.4 (migration `0005`), Aturan #21, #23, #26 · [ADR-004](../../decisions/ADR-004-api-key-format.md) (format & hashing API key) · [ADR-005](../../decisions/ADR-005-public-form-key.md) (kenapa public form key **bukan** API key) · [`api.md`](../../architecture/api.md) bagian *Idempotency*, *Rate limiting*, *Yang menyusul di Phase 4*

---

## Tujuan

**Tesis produk terbukti.** Capture layer bekerja dari luar.

Sampai sekarang setiap lead di sistem ini lahir dari seseorang yang mengetiknya — lewat `curl` di Phase 2,
lewat form dashboard di Phase 3. Keduanya membuktikan lead bisa **dikelola**. Tidak satu pun membuktikan
lead bisa **masuk sendiri**.

Kalimat inti MVP (freeze 3, Decisions §23) menyebutnya secara eksplisit:

> Owner mendaftar → verifikasi email → login → undang employee → employee login →
> **owner membuat API key → website mengirim lead** → owner meng-assign →
> employee menerima notifikasi → follow-up dari HP → update → konversi ke Customer

Dua langkah yang ditebalkan adalah satu-satunya bagian dari kalimat itu yang belum pernah dijalankan
siapa pun. Phase 4 menutupnya.

> Ini juga phase pertama yang menghadapi **klien yang bukan kita**. Dashboard salah bisa diperbaiki dan
> di-deploy ulang tanpa memberi tahu siapa pun; bentuk request yang sudah dipakai integrator tidak bisa.
> Itulah kenapa `api.md` sengaja ditulis sejak bootstrap, dan kenapa halaman dokumentasi di phase ini
> bukan pelengkap.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Owner | Membuat kredensial untuk website saya, tanpa meminta bantuan developer Jualin | Bisa mulai mengirim lead di hari yang sama saat saya berlangganan |
| 2 | Owner | Melihat kredensial mana yang pernah dipakai, dan kapan terakhir | Bisa tahu integrasi mana yang hidup dan mana yang tinggal nama |
| 3 | Owner | Mencabut satu kredensial seketika, tanpa mengganggu yang lain | Kebocoran satu kunci tidak berarti mematikan seluruh integrasi |
| 4 | Owner | Melihat lead yang masuk lewat API tercampur dengan lead manual, dengan penanda dari mana ia datang | Tidak perlu membuka dua tempat berbeda untuk pekerjaan yang sama |
| 5 | Developer pelanggan | Satu halaman yang cukup untuk membuat integrasi bekerja | Tidak perlu menghubungi support — dan saya tidak perlu membayar orang yang menjawabnya |
| 6 | Developer pelanggan | Mengulang request yang timeout tanpa membuat lead ganda | Retry saat jaringan buruk adalah kejadian normal, bukan insiden data |
| 7 | Developer pelanggan | Tahu berapa sisa kuota saya **sebelum** ditolak | Bisa mengatur laju kirim sendiri, bukan menabrak `429` lalu menebak |
| 8 | Developer pelanggan | Pesan kesalahan yang menyebut field mana yang salah | Bisa memperbaikinya tanpa menebak-nebak |
| 9 | Pemilik sistem | Kredensial yang bocor **tidak bisa** dipakai mengelola tim, mengubah subscription, atau membuat kredensial baru | Radius ledakan kebocoran API key terbatas pada "lead palsu masuk", bukan pengambilalihan organization |

---

## Acceptance Criteria

Phase 4 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | `curl` dari mesin di luar jaringan kita → lead muncul di dashboard, dengan `source = api` dan penanda kredensial mana yang mengirimnya |
| 2 | Raw secret ditampilkan **tepat sekali** saat dibuat. Tidak ada endpoint, layar, atau query yang bisa menampilkannya lagi (Aturan #21) |
| 3 | Kredensial yang direvoke ditolak **seketika** — tanpa jeda cache, tanpa masa tenggang |
| 4 | Request berulang dengan `Idempotency-Key` sama mengembalikan **lead yang sama**, bukan duplikat dan bukan error |
| 5 | API key **tidak bisa** memanggil satu pun endpoint aplikasi pengguna — bukan karena setiap handler mengeceknya, melainkan karena otorisasi memang tidak punya jalan untuk mengizinkannya (Aturan #24) |
| 6 | Setiap response API publik membawa `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`; `429` membawa `Retry-After` |
| 7 | Lookup kredensial **tidak** memindai tabel — `key_id` di-index, secret dibandingkan `subtle.ConstantTimeCompare` (ADR-004) |
| 8 | Payload yang dikirim integrator tersimpan apa adanya di `raw_payload`, termasuk field yang tidak dikenal |
| 9 | Owner bisa membuat, melihat, dan mencabut kredensial **dari dashboard**, tanpa `curl` |
| 10 | Halaman dokumentasi integrasi cukup untuk membuat integrasi bekerja **tanpa bertanya kepada siapa pun** — diverifikasi dengan mengikutinya dari nol, bukan dengan membacanya |
| 11 | Raw API key **tidak pernah** muncul di log, termasuk saat request-nya gagal (Aturan #26) |
| 12 | Harness isolasi tenant bertambah kasus untuk jalur kredensial baru, dan **tetap terbukti bisa gagal** |
| 13 | `idempotency_key` punya retensi — key kedaluwarsa tidak lagi tersimpan selamanya (utang yang diwarisi Phase 2) |

---

## Keputusan yang ditutup di phase ini

Lima keputusan yang jatuh tempo di sini, ditutup di muka daripada di tengah implementasi. Alasan
teknis lengkap ada di [`td.md`](./td.md); yang di bawah adalah **apa** dan **kenapa** dalam kalimat produk.

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **D1** | Bagaimana otorisasi bekerja untuk principal yang **tidak punya role**? | `tenant.Context` bertambah `Scopes`; `authz.Require` bercabang pada `PrincipalType`. Untuk API key ada **satu** peta `Action → scope`, berisi tepat satu baris: `lead.create → leads:write`. | Aturan #24 menuntut "ditegakkan di middleware, bukan per handler". Peta yang hanya berisi satu baris membuat setiap action lain ditolak **karena tidak ada di daftar** — bukan karena seseorang ingat menuliskan pengecualiannya. Menambah kemampuan API key di masa depan berarti menambah baris ke satu tempat yang terlihat, bukan mengaudit ulang seluruh handler. |
| **D2** | `POST /v1/leads` satu path untuk dua jalur kredensial, atau path terpisah? | **Satu path.** Middleware memilah dari prefix `jln_live_`/`jln_test_` pada `Authorization: Bearer`. | Freeze 3.1 dan `api.md` sama-sama menyebut endpoint publiknya `POST /v1/leads`. `api.md` melarang endpoint menerima dua principal *tanpa alasan tertulis* — ini alasan tertulisnya, dan bentuk kredensialnya bisa dibedakan tanpa menebak. Path terpisah (`/v1/public/leads`) berarti dua handler yang harus tetap sama selamanya. |
| **D3** | Retensi `idempotency_key` (utang Phase 2) | **48 jam**, dihapus malas saat `POST /v1/leads` berjalan — tanpa scheduler, tanpa tabel `jobs`. | `api.md` menyebut 24–48 jam; freeze melarang worker sampai ada kebutuhan async nyata (Phase 5/7). Penghapusan malas menumpang pada satu-satunya jalur yang pasti berjalan bila keynya dipakai sama sekali. Bila tidak ada traffic, tidak ada yang perlu dibersihkan. |
| **D4** | Angka rate limit final (terbuka sejak Phase 1) | **60 request/menit per API key** untuk `POST /v1/leads`, jendela tetap. Bisa diubah lewat konfigurasi tanpa deploy ulang kode. | Website SMB yang normal mengirim beberapa lead per jam, bukan per detik. 60/menit memberi ruang dua orde besaran di atas pemakaian nyata sambil tetap membuat penyalahgunaan mahal. Angkanya **bukan** hasil pengukuran — ia baru bisa di-tuning setelah ada integrator sungguhan, dan headernya (kriteria #6) memang ada supaya perubahan itu tidak mengejutkan siapa pun. |
| **D5** | Apakah API publik boleh dipanggil dari browser? | **Tidak.** Origin website pelanggan tidak pernah masuk `CORS_ALLOWED_ORIGINS`, jadi request browser dari sana tidak pernah mendapat header CORS. | Aturan #23: API key tidak pernah hadir di sisi klien. Bila browser bisa memanggilnya, seseorang **akan** menempelkan key di JavaScript website-nya, dan itu berarti kredensial organization tersebar ke setiap pengunjung. Ternyata ini **sudah** terjaga oleh allowlist yang dibuat di #30 — yang belum ada hanyalah pernyataan tertulisnya, test yang menguncinya, dan peringatan di halaman dokumentasi (TD §8). Kebutuhan "form di website mengirim lead langsung dari browser" adalah **Phase 6** dan punya kredensialnya sendiri (ADR-005). |

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| Embedded form, `forms`, `public_key`, `source_form_id` | Phase 6 — kredensial berbeda dengan aturan berbeda (ADR-005) |
| Mobile app | Phase 5 — memakai user session, **tidak menyentuh API key sama sekali** (Aturan #24) |
| Webhook keluar (Jualin memberi tahu sistem pelanggan) | Phase 7 |
| Endpoint publik selain `POST /v1/leads` — membaca lead, mengubah status, mengelola tim lewat API | Belum dijadwalkan — lihat catatan di bawah |
| Scope selain `leads:write` | Kolomnya ada sejak awal (ADR-004 aturan #4), nilainya menyusul saat ada endpoint keduanya |
| Penegakan kuota per plan (limit lead per bulan) | Phase 8 — rate limit ≠ quota, lihat catatan di bawah |
| SDK / library klien resmi | Belum dijadwalkan — `curl` dan contoh dalam bahasa umum lebih dulu |
| Rotasi otomatis & masa berlaku key (`expires_at` terisi) | Kolomnya dibuat, pengisiannya belum — lihat catatan di bawah |
| Dashboard analytics pemakaian API (grafik request per hari) | Phase 8 bersama usage counter |

### Tiga hal yang sengaja tertahan

**API publik hanya menulis, belum membaca.** Freeze 3.1 menyebut cakupannya `POST /v1/leads` — satu
endpoint, bukan "API publik" dalam arti seluruh CRUD. Alasannya bukan kemalasan: endpoint baca publik
mengharuskan kita membekukan bentuk respons lead untuk pihak luar, dan bentuk itu masih berubah setiap
phase (`source_form_id` datang di Phase 6, Deal mengubah daftar status pasca-Phase 5). Menulis lebih
aman dibekukan lebih dulu karena payload masuk memang sudah stabil sejak Phase 2.

**Rate limit bukan quota.** Rate limit menjawab *"seberapa cepat"* dan melindungi sistem; quota menjawab
*"seberapa banyak per bulan"* dan menegakkan harga. Keduanya sering dikira satu hal. Phase 4 hanya
mengerjakan yang pertama. Yang kedua butuh `usage_counters` dan definisi plan yang memang dijadwalkan
Phase 8 — dan menebaknya sekarang berarti membangun penegakan untuk harga yang belum ditetapkan.

**`expires_at` dibuat kosong.** Kolomnya ada di ADR-004 dan dibuat di `0005`, tetapi tidak ada UI dan
tidak ada perilaku yang mengisinya. Menambahkan kolom nullable belakangan ke tabel yang sudah beredar
bersifat instan; yang tidak instan adalah menjelaskan kepada integrator kenapa kunci mereka tiba-tiba
mati. Masa berlaku diaktifkan saat ada yang memintanya, bukan karena tabelnya terlihat kurang lengkap.

---

## Dependensi

Phase 1, 2, dan 3 selesai. Yang dipakai phase ini sudah ada seluruhnya:

| Yang dipakai | Dari | Catatan |
|---|---|---|
| `tenant.Context` dengan `PrincipalAPIKey` dan `APIKeyID` | Phase 1 | **Sudah ada, belum pernah dipakai.** Field `APIKeyID` ditulis di Phase 1 dengan komentar eksplisit: *"exists now so this struct's shape doesn't change when Phase 4 introduces API keys"*. Phase 4 adalah momen itu. |
| `internal/shared/token` (SHA-256 + hex) | Phase 1 | Doc comment-nya sudah menyebut ADR-004 sebagai prinsip yang sama |
| `internal/shared/ratelimit` | Phase 1 | **Perlu diperluas** — `Allow(key) bool` tidak cukup untuk header `X-RateLimit-Remaining`/`Reset` (TD §6) |
| `leads.source` menerima `api`, `raw_payload jsonb`, `idempotency_key` + `uq_leads_org_idempotency` | Phase 2 | Ketiganya dibuat di `0003` **sebelum** ada yang memakainya, persis untuk phase ini |
| `lead.Usecase.Create` dengan jalur idempotent replay | Phase 2 | Sudah mengembalikan `isNew=false` + 200 pada replay; jalur API memakai ulang, tidak menulis ulang |
| `auditlog` dengan `actor_type = 'api_key'` | Phase 1 | Nilai `api_key` sudah ada di `ck_audit_logs_actor_type` sejak `0002` |
| `crm_dashboard` — sesi, klien API, app shell, komponen | Phase 3 | Dua layar baru menempel pada kerangka yang sudah ada |

**Satu kewajiban diwarisi Phase 2** ([`02-crm-core/td.md`](../02-crm-core/td.md) §19): retensi
`idempotency_key`. Phase 2 mencatatnya sebagai utang dengan kalimat *"baru benar-benar relevan di Phase 4,
saat integrator sungguhan mulai mengirim key"*. Itu terjadi sekarang — ia masuk sebagai acceptance
criterion #13, bukan sebagai catatan.

**Tidak ada keputusan yang memblokir.** D1–D5 ditutup di atas.

### Yang **tidak** dibutuhkan phase ini

Demo ke calon pengguna (tujuan Phase 3) dan pemilihan Phase 4-vs-5 sudah dicatat di `STATUS.md` sebagai
keputusan manusia. Phase 4 dipilih lebih dulu **bukan** karena lebih penting dari Phase 5, melainkan
karena Phase 5 menunggu pihak ketiga: pendaftaran Apple Developer Program dan Firebase belum dimulai
(`STATUS.md` bagian *Punya Lead Time*), sementara Phase 4 tidak menunggu siapa pun. Freeze menempatkan
keduanya tidak saling bergantung, jadi urutan ini tidak melanggar apa pun.

---

## Pembagian issue

Freeze memetakan Phase 4 sebagai **dua** session (13: API Key · 14: Public Lead API + rate limit +
dokumentasi). Saya pecah menjadi **empat**, dengan memisahkan aplikasi:

| Urut | Issue | Kenapa dipisah |
|---|---|---|
| 1 | Migration `0005` + domain `api_key` + CRUD kredensial | Kredensialnya harus ada sebelum ada yang bisa mengautentikasi dengannya. Seluruhnya `crm_be`, dan seluruhnya bisa diuji tanpa menyentuh jalur lead. |
| 2 | Autentikasi API key + `POST /v1/leads` publik + rate limit + idempotency | Bagian dengan risiko keamanan tertinggi di seluruh phase: satu peta otorisasi salah dan API key bisa mengelola tim. Layak direview sendiri, bukan tertimbun di antara layar. |
| 3 | Dashboard — manajemen API key | Aplikasi berbeda, CI berbeda (ADR-009). Bergantung pada #1, **tidak** pada #2. |
| 4 | Halaman dokumentasi integrasi — penutup phase | Freeze: *"Halaman dokumentasi integrasi bukan pelengkap."* Ia hanya bisa ditulis jujur setelah #2 selesai, karena isinya adalah perilaku nyata endpoint itu — bukan rencana perilakunya. |

Ini **penyimpangan dari peta session di freeze**, dicatat di sini agar terlihat — pola yang sama dipakai
Phase 1 (3 → 4) dan Phase 2 (4 → 5). Yang menambah dua bukan kerumitan backend, melainkan bahwa Phase 4
menyentuh **dua aplikasi**, dan freeze memetakan session sebelum ADR-009 memisahkan repo-nya.

Rincian: [`issues.md`](./issues.md)

---

## Bukan tujuan phase ini

- **Bukan** membangun "API publik" yang lengkap — satu endpoint tulis, sesuai freeze 3.1
- **Bukan** menetapkan harga atau limit plan — rate limit melindungi sistem, bukan menegakkan tagihan
- **Bukan** membuat kredensial kedua untuk browser — itu Phase 6, dan ADR-005 sudah menjelaskan kenapa ia tidak boleh dijadikan satu dengan API key
- **Bukan** menyiapkan struktur untuk webhook yang "sudah pasti datang" (Aturan #27, #28)
