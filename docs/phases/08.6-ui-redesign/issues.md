# Phase 8.6 — Redesain UI · Issues

> Indeks. **Tanpa kolom status** — status hidup di [GitHub Issues](https://github.com/Pravasta/jualin-crm/milestone/13) (ADR-008).
> Apa & kenapa di [`prd.md`](./prd.md) · bagaimana di [`td.md`](./td.md) · brief desain di [`design-brief.md`](./design-brief.md).

**Milestone:** [Phase 8.6 — Redesain UI](https://github.com/Pravasta/jualin-crm/milestone/13)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [159](https://github.com/Pravasta/jualin-crm/issues/159) | Token desain & fondasi visual dashboard | `crm_dashboard` | `globals.css`, Plus Jakarta Sans + JetBrains Mono, radius, `status-badge.tsx` (Opsi B), tabel kontras dihitung ulang | §3, §3.1 |
| [160](https://github.com/Pravasta/jualin-crm/issues/160) | Kerangka aplikasi responsif | `crm_dashboard` | `app-shell.tsx`, sidebar 232/68, header HP + bilah bawah + sheet "Lainnya" | §4.1 |
| [161](https://github.com/Pravasta/jualin-crm/issues/161) | Daftar Lead | `crm_dashboard` | Tabel→kartu, chip status + "tanpa pemilik aktif", sheet filter, dialog→sheet, 4 keadaan | §4.2, §4.3 |
| [162](https://github.com/Pravasta/jualin-crm/issues/162) | Detail Lead | `crm_dashboard` | Stepper 5 tahap, tombol berkelompok, timeline, tugas, konversi, konflik 409 | §4.2, §4.3 |
| [163](https://github.com/Pravasta/jualin-crm/issues/163) | Beranda | `crm_dashboard` | Kartu metrik, chip per status, performa anggota, "belum ada data" ≠ 0% | §4 |
| [164](https://github.com/Pravasta/jualin-crm/issues/164) | Customer — daftar & detail | `crm_dashboard` | Tanpa nilai kontrak, tanpa riwayat pembelian | §4.2 |
| [165](https://github.com/Pravasta/jualin-crm/issues/165) | Tugas & Tim | `crm_dashboard` | Selesai satu arah, 4 role, dialog nonaktif tiga cabang | §4.2 |
| [166](https://github.com/Pravasta/jualin-crm/issues/166) | Connect + sub-layar | `crm_dashboard` | 3 kartu kanal, kredensial satu kali, riwayat pengiriman | §4.2, §4.3 |
| [167](https://github.com/Pravasta/jualin-crm/issues/167) | Langganan & Pengaturan | `crm_dashboard` | Free/Pro/Enterprise dari API, pemakaian, profil baca-saja | §4 |
| [168](https://github.com/Pravasta/jualin-crm/issues/168) | Layar auth | `crm_dashboard` | Masuk, pemilih organization, daftar, verifikasi, undangan | §3, §4.3 |
| [169](https://github.com/Pravasta/jualin-crm/issues/169) | Laporan API — tren, sumber, alasan kalah | `crm_be` | `Filter.Assignee`/`Source`, 3 endpoint, timezone organization | §2.1–2.4 |
| [170](https://github.com/Pravasta/jualin-crm/issues/170) | Laporan API — waktu respons & tugas | `crm_be` | Bucket waktu respons + `never_touched`, tugas per anggota | §2.5, §2.6 |
| [171](https://github.com/Pravasta/jualin-crm/issues/171) | Layar Laporan | `crm_dashboard` | 8 blok, filter global, grafik tanpa dependensi baru, 4 keadaan | §4.4 |
| [172](https://github.com/Pravasta/jualin-crm/issues/172) | Tema mobile ≥7:1 | `crm_employee` | `theme.dart`, badge Opsi A, keenam layar | §5 |
| [173](https://github.com/Pravasta/jualin-crm/issues/173) | Mobile — Lead Saya & Detail Lead | `crm_employee` | Tata letak handoff, bilah aksi Telepon/WhatsApp, pemilih transisi sah | §5 |
| [174](https://github.com/Pravasta/jualin-crm/issues/174) | Dokumentasi + penutup phase | `crm_be` + docs | `api.md`, `authorization.md`, `testing/flow/`, review 10 AC. **Penutup phase** | §7 |

---

## Urutan

```
#159 ──► #160 ──┬──► #161 ──► #162 ──► #163 ──► #164 ──► #165 ──► #166 ──► #167 ──► #168 ──┐
                │                                                                          │
                └──────────────────────────► #171 ◄── #169 ──► #170 ────────────────────────┤
                                                                                           │
#172 ──► #173 ──────────────────────────────────────────────────────────────────────────────┤
                                                                                           ▼
                                                                                         #174
```

| Dependensi | Sifat |
|---|---|
| Semua layar dashboard → #159 | **Keras.** Mengganti font setelah layar dibangun berarti setiap lebar tabel dan pembungkusan label dihitung ulang (TD §8) |
| Layar dashboard → #160 | **Keras.** Tiap layar dibangun di dalam kerangka responsif, bukan membawa aturan breakpoint sendiri-sendiri |
| #161 → #162 → … → #168 | **Lunak.** Berurutan karena satu issue = satu session, bukan karena saling bergantung. Urutannya mengikuti prioritas brief: *"daftar lead & detail lead adalah produknya"* |
| #171 → #169, #170 | **Keras.** Lima blok Laporan tidak punya data sampai endpoint-nya ada |
| #169 ‖ #170 ‖ (layar dashboard) | **Paralel.** Dua issue backend hanya menyentuh `internal/metrics`; layar dashboard tidak menyentuh Go sama sekali |
| #173 → #172 | **Keras.** Tata letak baru dibangun di atas token yang sudah diverifikasi kontrasnya |
| (mobile) ‖ (dashboard) | **Paralel.** Aplikasi berbeda, tidak ada berkas yang beririsan |
| #174 → semuanya | **Keras.** Prosedur uji responsif hanya bisa ditulis setelah layarnya ada |

**Item navigasi "Laporan" dipasang di #171**, bukan di #160 — menu yang menuju rute yang belum ada adalah
tautan rusak yang terlihat seperti kemajuan. #160 memasang delapan item yang halamannya sudah ada.

---

## Batas per issue

| Issue | Setelah selesai, yang **belum** ada |
|---|---|
| #159 | Token dan badge baru terpasang, tapi **tidak satu layar pun berubah** — perubahannya baru terlihat sebagai pergantian font dan warna netral |
| #160 | Dashboard bisa dibuka di HP tanpa geser samping, tapi **isi halamannya masih tata letak lama** |
| #161–#168 | Layar demi layar pindah ke sistem baru; sampai #168 selesai, dashboard **campur dua gaya** — konsekuensi yang diterima dari satu issue satu PR |
| #169 | Tiga endpoint bisa dipanggil lewat `curl`, tapi **tidak ada layar yang memanggilnya** |
| #170 | Kelima endpoint lengkap, masih **tanpa satu pun grafik** |
| #171 | Laporan hidup, tapi **belum ada prosedur uji responsif tertulis** dan `api.md` belum memuat kelima endpoint |
| #172 | Aplikasi mobile berganti warna dan font, **tata letaknya belum berubah** |
| #173 | Dua layar terpenting mobile selesai; **Tugas Saya, Notifikasi, Masuk, dan biometrik hanya berganti token** — tata letak barunya menunggu desain |
| #174 | Phase 8.6 tutup. Laporan lanjutan, ekspor, dark mode, dan tata letak baru empat layar mobile **belum disentuh** — dan memang tidak boleh |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan* dan bersifat mengikat.

---

## Yang harus dijaga di setiap PR

| Hal | Kenapa |
|---|---|
| **Data dummy handoff bukan data sistem** | Tabel selisihnya di [`prd.md`](./prd.md) §*Handoff vs sistem*. Setiap kanal, paket, role, dan kolom yang tidak ada di API **tidak dibangun**, meski ada gambarnya |
| **Angka kontras dihitung, bukan disalin** | Sudah meleset dua kali: #40 (aksen 4.45:1) dan #70 (tiga angka desainer) |
| **Perilaku tidak boleh hilang diam-diam** | Test yang ada tidak dilonggarkan. Bila sebuah test harus berubah, alasannya ditulis di PR |
| **Tanpa guliran horizontal halaman** | Diuji di 360/390/820/1440 px, bukan diperkirakan |
