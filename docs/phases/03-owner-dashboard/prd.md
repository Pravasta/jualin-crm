# Phase 3 — Owner Dashboard · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 4 (Phase 3), 3.2 (cakupan dashboard & metrik dasar), 2.3 ketentuan #3 (filter jaring pengaman), Aturan #25 (token di cookie) · [ADR-009](../../decisions/ADR-009-monorepo-structure.md) (letak `crm_dashboard/`)

---

## Tujuan

**Demo pertama.** Produk bisa dipakai tanpa `curl`.

Phase 0 menyiapkan lantai, Phase 1 dinding tenancy, Phase 2 seluruh siklus hidup lead. Ketiganya nyata,
tetapi **tidak seorang pun di luar terminal ini pernah melihatnya**. Phase 3 adalah yang pertama
menghasilkan sesuatu yang bisa ditunjukkan ke calon pelanggan.

> Ini juga phase pertama di luar `crm_be`. Sampai sekarang `crm_dashboard/` hanya berisi `README.md`
> yang menyatakan "belum dibuat" (ADR-009) — Phase 3 mengisinya.

**Lead list & detail adalah produknya** (freeze 3.2). Layar itu dibuka ratusan kali sehari; sisanya
sesekali. Alokasi usaha dan pembagian issue di bawah mengikuti perbandingan itu, bukan membagi rata.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Owner baru | Mendaftar, memverifikasi email, dan masuk lewat layar | Tidak perlu tahu apa itu `curl` untuk mulai memakai produk yang saya bayar |
| 2 | Owner | Melihat daftar lead dengan filter dan pencarian | Bisa menjawab "mana yang belum ditangani" dalam hitungan detik, bukan dengan membaca seluruh daftar |
| 3 | Owner | Melihat satu lead beserta seluruh riwayatnya dalam satu layar | Bisa melanjutkan pekerjaan orang lain tanpa bertanya |
| 4 | Owner | Menugaskan lead ke sales lewat beberapa klik | Distribusi kerja tidak lagi bergantung pada saya mengingat siapa memegang apa |
| 5 | Owner | Melihat lead yang **tidak punya pemilik aktif** | Lead tidak menghilang diam-diam saat seseorang resign — jaring pengaman yang freeze 2.3 wajibkan |
| 6 | Owner | Mengundang, mengubah role, dan menonaktifkan anggota tim | Tidak perlu meminta developer setiap ada karyawan masuk atau keluar |
| 7 | Owner | Melihat angka dasar: lead masuk, per status, belum ter-assign, conversion rate, performa per employee | Bisa tahu keadaan bisnis tanpa menghitung manual |
| 8 | Owner | Mengubah lead yang berhasil menjadi customer lewat satu aksi | Relasi jangka panjang tercatat, dan saya tahu kapan itu terjadi |
| 9 | Siapa pun yang login | Melihat notifikasi saat ada lead ditugaskan ke saya | Bisa merespons tanpa memantau layar terus-menerus |
| 10 | Siapa pun yang login | Melihat pesan kesalahan yang bisa dimengerti, dalam Bahasa Indonesia | Tahu apa yang salah dan apa yang harus dilakukan, bukan melihat kode HTTP |

---

## Acceptance Criteria

Phase 3 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Owner menyelesaikan **seluruh core loop tanpa menyentuh terminal**: daftar → verifikasi → masuk → buat lead → assign → catat activity → buat task → ubah status → konversi ke customer |
| 2 | Browser di origin dashboard bisa memanggil API — CORS dikonfigurasi eksplisit per origin, **tidak pernah** `*`, dan **gagal saat boot** bila kosong di production |
| 3 | Cookie sesi tetap `HttpOnly` — JavaScript dashboard **tidak pernah** menyentuh access/refresh token (Aturan #25), dan setiap request non-GET membawa `X-CSRF-Token` |
| 4 | Beberapa request yang bersamaan menerima `401` menghasilkan **tepat satu** panggilan refresh, bukan satu per request |
| 5 | Daftar lead bisa disaring per status, pemilik, sumber, periode, dan kata kunci, dengan pagination yang menampilkan jumlah total |
| 6 | Filter **"lead tanpa pemilik aktif"** tersedia permanen di daftar lead — kewajiban yang diteruskan Phase 2 |
| 7 | Layar detail lead menampilkan timeline activity terbaru-dulu, task pada lead itu, dan seluruh aksi tulis (status, assignment, catatan, konversi) |
| 8 | Menyimpan perubahan pada data yang sudah berubah di tempat lain → layar menampilkan **konflik**, memuat ulang keadaan terkini, dan **tidak pernah menimpa otomatis** |
| 9 | Menonaktifkan anggota yang masih memegang lead terbuka → layar menampilkan jumlahnya dan **memaksa memilih** lepas assignment / pindahkan / batal |
| 10 | Metrik dasar freeze 3.2 tampil dari endpoint agregat backend — **bukan** dihitung di browser dari halaman yang sedang terlihat |
| 11 | Conversion rate **mengecualikan** `spam` dan `unqualified` dari penyebut |
| 12 | Seluruh teks antarmuka dan pesan kesalahan dalam **Bahasa Indonesia** |
| 13 | CI punya workflow tersendiri untuk `crm_dashboard/` dengan `paths:` filter — backend tidak ikut jalan saat hanya UI berubah, dan sebaliknya |

---

## Keputusan yang ditutup di phase ini

`STATUS.md` menandai **Bahasa UI** sebagai keputusan yang jatuh tempo di Phase 3. Dua lagi ternyata
belum pernah diputuskan di dokumen manapun dan ditutup di sini juga, sebelum implementasi — bukan di
tengah jalan:

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **C1** | Bahasa UI: Indonesia / Inggris / dwibahasa | **Indonesia saja, tanpa i18n** | Backend sudah mengunci `error.message` dalam Bahasa Indonesia sejak #9 (*"Role Anda tidak mengizinkan aksi ini"*). Memasang i18n berarti antarmuka dwibahasa di atas lapisan error yang tetap satu bahasa — campur aduk di layar yang sama, dengan biaya setiap string lewat kunci terjemahan. Retrofit i18n memang menyakitkan; tetapi memasangnya sekarang untuk bahasa kedua yang belum ada pelanggannya adalah persis yang Aturan #28 larang. |
| **C2** | Bagaimana browser bicara ke Go API | **Browser → Go langsung, dengan CORS.** Bukan BFF. | Cookie `HttpOnly SameSite=Lax` tetap terkirim antar-subdomain (`app.` → `api.` adalah same-site), dan CSRF double-submit `httpx.VerifyCSRF` sudah dibangun dan diuji sejak #10 **persis untuk kasus ini**. BFF lewat Next.js route handler menghilangkan CORS, tetapi menambah satu hop di setiap request dan mewajibkan runtime Node permanen — menaikkan biaya infrastruktur per tenant untuk memecahkan masalah yang sudah dipecahkan. |
| **C3** | Komponen UI | **shadcn/ui + Tailwind** | Komponen di-*copy* ke repo, bukan dependensi runtime — tidak ada versi library yang mengunci, dan setiap komponen bisa diubah tanpa melawan library. Bundle kecil, relevan untuk sales yang membuka dashboard dari koneksi lambat. Alternatif (Mantine/MUI) memberi lebih banyak komponen jadi, dengan harga gaya visual yang sulit dilepas. |

C1 dicoret dari `STATUS.md` bagian *Keputusan Belum Diambil* dan diperbarui di
[`product/glossary.md`](../../product/glossary.md) — dua tempat yang sebelumnya menyatakan "belum
diputuskan".

---

## Phase 3 bukan phase frontend murni

Dua pekerjaan Go wajib mendahului layar manapun. Keduanya bukan temuan kejutan — keduanya tercatat
sebagai utang Phase 2 — tetapi keduanya mudah terlewat kalau phase ini dibaca sebagai "phase UI":

**1. CORS belum ada sama sekali.** Tidak ada middleware CORS di `crm_be`, dan tidak ada satu pun
keputusan tentang origin di `docs/architecture/`. Tanpa itu, keputusan C2 tidak bisa berjalan: browser
di origin dashboard akan ditolak sebelum request pertama sampai ke handler.

**2. Endpoint metrik belum ada.** [`02-crm-core/td.md`](../02-crm-core/td.md) §5 menyatakannya
eksplisit: *"Di Phase 2 belum ada endpoint metrik (itu Phase 3)"*. Freeze 3.2 mewajibkan lima metrik.
Menghitungnya di browser dari endpoint list yang berpaginasi **salah** — angkanya akan mengikuti halaman
yang sedang terlihat, bukan keseluruhan data, dan salahnya tidak akan terlihat sampai ada organization
dengan lebih dari satu halaman lead.

Keduanya digabung menjadi satu issue Go yang mendahului seluruh pekerjaan UI (issue #30).

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| Landing page (`crm_landing_page/`) | Belum dijadwalkan — bukan bagian Phase 3 |
| Mobile app, push notification, `device_tokens` | Phase 5 |
| API Key, `POST /v1/leads` publik, halaman dokumentasi integrator | Phase 4 |
| Angka revenue / nilai deal di metrik | Menunggu Deal, pasca-Phase 5 — freeze 3.2 mencatatnya sebagai ekspektasi yang disepakati, bukan kekurangan |
| Grafik tren & laporan lanjutan | Phase 9 — Phase 3 menampilkan angka, bukan visualisasi deret waktu |
| Export CSV dari daftar lead | Belum dijadwalkan — sama seperti import, naikkan prioritas bila calon pelanggan memintanya |
| Realtime / websocket (notifikasi muncul tanpa refresh) | Belum dijadwalkan — freeze melarang message broker; polling sederhana cukup untuk MVP |
| Dark mode, tema, kustomisasi tampilan | Saat diminta |
| Layar untuk Employee | Employee memakai mobile (Phase 5). Dashboard adalah alat Owner/Admin/Manager. |

### Dua hal yang sengaja tertahan

**Realtime notification.** Freeze melarang message broker, dan websocket untuk memunculkan lonceng
notifikasi bukan alasan yang cukup untuk melanggarnya. Dashboard mengambil notifikasi saat halaman
dimuat dan saat pengguna membukanya. Bila nanti terasa kurang, jawabannya polling interval — bukan
infrastruktur baru.

**Layar Employee.** Dashboard ini untuk Owner/Admin/Manager. Employee **bisa** login (backend tidak
melarangnya) dan akan melihat data yang di-scope repository ke lead miliknya sendiri — tetapi tidak ada
layar yang dirancang untuk alur kerja mereka. Itu Phase 5, dan mencampurnya ke sini berarti membangun
dua produk dalam satu phase.

---

## Dependensi

Phase 2 selesai — seluruh siklus lead, activity, task, assignment, notification, dan customer sudah ada
di API internal, tertutup harness isolasi tenant yang terbukti bisa gagal.

**Satu kewajiban diwarisi Phase 2** ([`02-crm-core/td.md`](../02-crm-core/td.md) §19): dashboard wajib
menyediakan filter permanen **"lead tanpa pemilik aktif"** sebagai jaring pengaman terhadap membership
yang dinonaktifkan (freeze 2.3 ketentuan #3). Query-nya sudah didukung `assigned_to=none` sejak #20 —
yang belum ada hanya layarnya. Ia masuk sebagai acceptance criterion #6, bukan sebagai catatan.

**Keputusan yang masih menunggu tetapi tidak memblokir:** domain final & hosting. Keduanya menentukan
nilai konkret `CORS_ALLOWED_ORIGINS` dan `COOKIE_DOMAIN` saat deploy, bukan bentuk kodenya —
pengembangan lokal berjalan dengan `localhost`.

---

## Pembagian issue

Freeze memetakan Phase 3 sebagai session 9–12 (**empat**). Setelah menghitung isinya — ±15 layar, dua
endpoint Go baru, middleware CORS, plus setup aplikasi yang belum pernah ada sama sekali — saya pecah
menjadi **enam**:

| Urut | Issue | Kenapa dipisah |
|---|---|---|
| 1 | Backend: CORS + endpoint metrik | Murni Go, bisa diuji penuh tanpa satu baris UI, dan **memblokir semua issue lain** — tanpa CORS tidak ada request browser yang berhasil. Mereviewnya bersama layar berarti mereview dua hal yang gagal dengan cara sangat berbeda. |
| 2 | Setup Next.js + auth UI + sesi | Fondasi: proyek, Tailwind/shadcn, klien API + CSRF + refresh single-flight, proteksi route, lima layar auth. Semua issue berikutnya butuh "bisa login". |
| 3 | Lead list + filter + pagination | Layar dengan traffic tertinggi (freeze 3.2). Termasuk filter "lead tanpa pemilik aktif". |
| 4 | Lead detail + timeline + activity + task + status + assignment + konversi | Traffic tertinggi kedua, dan tempat hampir seluruh aksi tulis berada. Digabung dengan #3 akan menghasilkan satu PR yang tidak bisa direview dengan jujur. |
| 5 | Tim: employee, undangan, penonaktifan, notification | Area admin. Penonaktifan membawa alur `on_open_leads` tiga cabang dari #22 — satu-satunya layar dengan percabangan keputusan sungguhan. |
| 6 | Home metrik + customer + daftar task + settings | Layar top-level yang tersisa, mengonsumsi endpoint dari issue #1. Penutup phase. |

Ini **penyimpangan dari peta session di freeze**, dicatat agar terlihat — pola yang sama dipakai Phase 1
(3 → 4) dan Phase 2 (4 → 5). Bila Anda lebih memilih lebih sedikit PR, katakan saat review: #5 dan #6
paling mudah digabung.

Rincian: [`issues.md`](./issues.md)

---

## Bukan tujuan phase ini

- **Bukan** membangun landing page atau apa pun yang menghadap publik selain dashboard itu sendiri
- **Bukan** mengubah kontrak API yang sudah ada — bila sebuah layar terasa butuh endpoint baru, itu
  sinyal untuk memeriksa apakah endpoint yang ada sudah cukup sebelum menambah (dua endpoint metrik
  adalah satu-satunya penambahan yang sudah disetujui di muka)
- **Bukan** mengejar kelengkapan visual — layar yang benar dan bisa dipakai lebih penting daripada layar
  yang cantik, dan seluruh Phase 3 ada untuk dibuktikan di depan calon pengguna
- **Bukan** menyiapkan struktur untuk Deal, grafik, atau realtime yang "sudah pasti datang"
  (Aturan #27, #28)
