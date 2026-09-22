# Phase 8.6 — Redesain UI: Dashboard Responsif, Laporan, Mobile · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`design-brief.md`](./design-brief.md) (brief untuk Claude Design, 21 Sep 2026) ·
> handoff Claude Design di [`../../design_handoff_jualin_crm/`](../../design_handoff_jualin_crm/README.md) ·
> [`scope.md`](../../product/scope.md) · [`freeze.md`](../../architecture/freeze.md)

---

## Dua hal yang harus dilaporkan sebelum apa pun (Aturan #30)

**1. Nomor phase ini menyimpang dari `freeze.md`.** Freeze memesan Phase 9 untuk *"Deal, Pipeline entity,
reports lanjutan, automation"*. Phase ini bukan itu: isinya perombakan tampilan untuk layar yang **sudah
ada**, ditambah **laporan dasar** (`scope.md` → *Operational: reports dasar*). Karena itu ia disisipkan
sebagai **8.6**, mengikuti 04.5, 04.6, dan 8.5. Phase 9 tetap milik Deal & Pipeline. "Reports lanjutan"
tetap di Phase 9 dan **tidak** disentuh di sini (lihat *Di luar cakupan*).

**2. Handoff desain memuat data dan fitur yang tidak ada di sistem.** Prototipe Claude Design dibuat
dengan **data dummy**, dan sebagian dummy itu melewati batas scope: 6 kanal Connect (WhatsApp Business
API, Instagram DM, Marketplace, Google Sheets), riwayat pembayaran + unduh invoice, nilai kontrak dan
riwayat pembelian customer, paket Starter/Tim/Bisnis dengan harga lain, role "Sales", dan preferensi
notifikasi. **Keputusan pemilik produk (23 Sep 2026): ambil gaya visualnya saja. Kontennya selalu data
yang sudah ada.** Daftar lengkap selisihnya ada di §*Handoff vs sistem*. Tidak ada satu pun yang
dibangun di phase ini.

---

## Tujuan

**Jualin CRM terlihat seperti produk yang layak dibayar dan bisa dipakai dari HP.** Setelah phase ini:
dashboard memakai satu sistem visual baru dan **responsif sampai 360 px**, sehingga Owner/Admin/Manager
bisa memeriksa lead dan tim dari browser HP. Ada bagian **Laporan** yang menjawab *"bagaimana kinerja
penanganan lead kami?"*, lengkap dengan API untuk kedelapan bloknya. Aplikasi mobile Employee memakai
keluarga visual yang sama, dengan kontras yang dinaikkan untuk dibaca di bawah matahari.

Perilaku tidak berubah. Aturan transisi status, otorisasi per role, konflik 409, kredensial satu kali,
dan dialog nonaktif tiga cabang semuanya **dipertahankan persis**. Yang berubah hanya tampilannya, dan
pada Laporan, datanya.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Owner/Admin/Manager | Membuka dashboard di **HP** tanpa menggeser layar ke samping | Saya bisa memeriksa lead dan tim saat di luar kantor |
| 2 | Owner | Tampilan yang rapi dan konsisten | Produk terasa layak dibayar, bukan purwarupa |
| 3 | Siapa pun | Delapan status lead bisa dibedakan **tanpa mengandalkan warna** | Badge tetap terbaca oleh buta warna dan di layar redup |
| 4 | Owner/Admin/Manager | Halaman **Laporan**: tren, sumber, alasan kalah, waktu respons, tugas per anggota | Saya tahu di mana penanganan lead bocor, bukan cuma berapa jumlahnya |
| 5 | Owner/Admin/Manager | Setiap angka di Laporan **bisa diklik** ke daftar lead terfilter bila memungkinkan | Angka bisa diperiksa, bukan hanya dipercaya |
| 6 | Employee | Teks status dan tombol di aplikasi mobile terbaca **di bawah matahari** | Saya bisa bekerja di lapangan |
| 7 | Pemilik produk | Tidak ada fitur baru yang menyusup lewat desain | Batas scope tetap dijaga (§*Handoff vs sistem*) |

---

## Acceptance Criteria

Phase 8.6 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Token desain handoff (warna oklch, Plus Jakarta Sans + JetBrains Mono, radius `0.5rem`) terpasang di `globals.css`. **Setiap pasangan teks/latar dihitung ulang** dan tercatat di `notes.md` (preseden #40, #70), **bukan disalin** dari angka desainer |
| 2 | Badge status di dashboard memakai **Opsi B** (tepi + bentuk: pil / kotak tebal / putus-putus) dan di mobile **Opsi A** (ikon + tint ≥7:1), sesuai keputusan di Token Desain §03 |
| 3 | **Tidak ada guliran horizontal halaman** di lebar 360, 390, 820, dan 1440 px di seluruh layar dashboard, **diuji**, bukan diperkirakan |
| 4 | Di bawah 768 px: sidebar diganti header + bilah bawah (**Beranda · Lead · Tugas · Laporan · Lainnya**), tabel diganti kartu, dialog menjadi *sheet* dari bawah, target sentuh ≥44 px |
| 5 | Seluruh layar dashboard yang ada (auth, Beranda, Lead, Detail Lead, Customer, Tugas, Tim, Connect + sub-layarnya, Langganan, Pengaturan, notifikasi) memakai sistem visual baru, **tanpa satu pun perilaku yang hilang**, dibuktikan oleh test yang sudah ada tetap lolos |
| 6 | **Tidak ada angka uang** di mana pun kecuali harga paket di Langganan. Tidak ada kanal, paket, role, atau kolom dari data dummy handoff yang muncul di UI |
| 7 | Laporan menampilkan **8 blok** dari API sungguhan. Lima endpoint baru di `/v1/metrics/*`, **Owner/Admin/Manager saja**, masing-masing punya **test isolasi tenant** (Aturan #7) |
| 8 | Conversion rate di Laporan mengecualikan Spam + Tidak Memenuhi Syarat **dan menjelaskannya di layar**. "Belum ada data" tampil berbeda dari 0% |
| 9 | Pengelompokan tren per hari/minggu memakai **`organizations.timezone`** (Aturan #13), bukan UTC |
| 10 | Aplikasi mobile: keenam layar memakai tema baru. Lead Saya dan Detail Lead mengikuti tata letak handoff. Pemilih status **hanya menawarkan transisi yang sah** (ADR-015/016), tidak delapan pilihan seperti di prototipe |

---

## Handoff vs sistem — yang **tidak** dibangun

Handoff adalah acuan **visual**. Baris di bawah adalah konten dummy yang bertentangan dengan sistem atau
scope. Implementasi memakai kolom **Yang dibangun**.

| Layar | Di handoff (dummy) | Yang dibangun | Alasan |
|---|---|---|---|
| Connect | 6 kartu: WhatsApp Business, Formulir, API & Webhook, Instagram DM, Marketplace, Google Sheets, dengan tombol Hubungkan/Putuskan | **3 kartu: API · Formulir · Webhook**, masing-masing membuka sub-layar yang sudah ada | ⛔ chat inbox WA/IG (`scope.md`). Marketplace dan Sheets tidak ada. Import/export ditunda |
| Connect | Kartu terkunci berlabel *"Butuh paket Tim"* | Kartu terkunci redup **tanpa nama paket, tanpa tombol upgrade** | Brief §8.9 |
| Langganan | Starter/Tim/Bisnis, Rp149rb/Rp399rb, 1/5/20 anggota | **Free/Pro/Enterprise** dari `GET /v1/plans` (Rp99.000, Enterprise "Hubungi kami") | Data sungguhan (Phase 8.5) |
| Langganan | Tombol Upgrade/Downgrade langsung di kartu paket | Tombol **coba Pro** hanya untuk Owner dan hanya bila test checkout aktif. Tanpa downgrade | Phase 8 D4: jalur downgrade tidak ada |
| Langganan | Riwayat pembayaran + *Unduh invoice* | — | ⛔ invoice generation, payment service terpisah |
| Langganan | Bar "Penyimpanan file 1.2 GB / 5 GB" | Hanya **lead bulan ini** dan **anggota** | Penyimpanan file tidak ada. Media storage adalah kelas biaya baru |
| Langganan | Fitur *"Peran & izin kustom"*, *"Laporan lengkap + tren"* | Daftar fitur dari data paket yang ada | RBAC dinamis ditunda. Laporan untuk semua paket (§*Keputusan*) |
| Customer | Nilai kontrak (Rp), riwayat pembelian/order | Profil, *"Berasal dari lead #…"*, catatan, dan data yang ada di `customers` | ⛔ angka uang. Order termasuk ERP |
| Tugas | Tugas yang tertaut ke **Customer** ("Kirim invoice bulanan") | Tugas **selalu** tertaut ke lead | `tasks.lead_id NOT NULL`, tanpa `customer_id` |
| Tugas | Checkbox bisa dicentang lalu dibuka lagi. Tab Semua/Tugas saya/Terlambat | Tandai selesai **satu arah**. Filter yang ada: penanggung jawab, Belum selesai/Selesai, jatuh tempo | Perilaku yang berjalan (brief §8.7) |
| Tim | Role Owner/Admin/**Sales**. Tidak ada dialog nonaktif | Empat role: **Owner/Admin/Manager/Employee**. Dialog tiga cabang **dipertahankan** | Kosakata wajib (brief §6). Pola §12.1 |
| Pengaturan | Tab Notifikasi (toggle, *ringkasan mingguan via email*), tab API & Webhook, tab Keamanan. Zona waktu dan email bisnis bisa diubah | Profil organization + pengguna, **baca saja** | Preferensi notifikasi tidak ada. Email berkala adalah kelas biaya baru. API sudah ada di Connect |
| Mobile | Pemilih status menawarkan 8 status | Hanya transisi sah + langkah alasan Kalah | ADR-015, ADR-016 |
| Semua | Bilah bawah HP berbeda-beda per halaman (Connect/Paket/Atur di slot ke-5) | **Tetap**: Beranda · Lead · Tugas · Laporan · Lainnya | Navigasi yang berpindah-pindah bukan navigasi |

Bila di tengah implementasi ditemukan selisih lain, perlakuannya sama: **data sungguhan menang, catat di
`notes.md`**.

---

## Keputusan yang sudah diambil

| # | Pertanyaan (brief §17) | Keputusan | Oleh |
|---|---|---|---|
| K1 | Blok Laporan [API BARU] sebelum API-nya ada | **API-nya dibangun di phase ini.** Kedelapan blok dirilis bersama, tidak ada blok "segera hadir" | Pemilik produk, 23 Sep 2026 |
| K2 | Satu phase atau tiga | **Satu phase**, satu milestone | Pemilik produk, 23 Sep 2026 |
| K3 | Mobile: handoff baru mencakup 2 dari 6 layar | Tema diterapkan ke **keenam** layar. Lead Saya dan Detail mengikuti tata letak handoff, empat layar lain di-*restyle* tanpa mengubah tata letak | Pemilik produk, 23 Sep 2026 |
| K4 | Navigasi dashboard di HP: bilah bawah atau drawer | **Bilah bawah + "Lainnya"** (pilihan desainer di handoff) | Desain |
| K5 | Stepper tahapan di mobile | **Tidak dibawa.** Handoff tidak memuatnya. Badge status + pemilih transisi sudah menjawab "di mana lead ini", sesuai target sesi 10–40 detik | Desain |
| K6 | Laporan untuk semua paket atau sebagian untuk Pro | **Semua paket** di phase ini. Menggerbangi Laporan adalah keputusan harga, bukan implementasi: bila diputuskan nanti, cukup tambah satu kanal di `planChannels` | Default, bisa dibuka kembali oleh pemilik produk |

---

## Di luar cakupan

| Yang tidak dikerjakan | Ke mana |
|---|---|
| Semua isi kolom *"Di handoff (dummy)"* di atas | Tidak ke mana pun. Bila diminta lagi → *scope discussion* |
| Dark mode | Tidak dijadwalkan (brief §15) |
| Laporan lanjutan: builder kustom, pivot, pembanding antar-periode | Phase 9 |
| Ekspor CSV/PDF laporan | Tidak dijadwalkan. Import/export CSV punya catatan prioritas sendiri di `scope.md` |
| Library grafik | Tidak ditambahkan. Grafik batang dan garis sederhana dibangun dengan elemen HTML/SVG biasa (Aturan #27, ringan) |
| Tata letak baru untuk Tugas Saya, Notifikasi, dan layar masuk mobile | Setelah Claude Design menyerahkan layarnya. Phase ini hanya me-*restyle* |
| Landing page | `crm_landing_page/`, belum dijadwalkan |

---

## Dependensi

| Butuh | Status |
|---|---|
| Handoff desain di `docs/design_handoff_jualin_crm/` | Ada, masuk repo lewat PR pembuka phase ini |
| `GET /v1/metrics/summary` + `/employees` (Phase 3) | Ada. Menjadi 3 blok [TERSEDIA] |
| `GET /v1/plans`, `limits`/`usage` di `/v1/me` (Phase 8.5) | Ada. Sumber layar Langganan |
| Skema `leads.source`, `leads.lost_reason`, `activities`, `tasks.due_at`/`completed_at` | Ada. **Laporan [API BARU] tidak butuh migration** (TD §1) |
