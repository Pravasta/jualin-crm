# ADR-009 — Monorepo dengan Empat Aplikasi

> **Status:** ✅ Accepted — 17 Agustus 2026
> **Mengubah:** struktur berkas di `phases/00-foundation/td.md` §2 dan `.claude/skills/jualin-backend/`

## Konteks

Jualin CRM terdiri dari empat aplikasi yang berbeda teknologi tetapi satu produk: backend Go, dashboard Next.js, landing page Next.js, dan mobile Flutter.

Sebelum keputusan ini, modul Go diletakkan di akar repository — yang berarti tiga aplikasi lain tidak punya tempat yang jelas, dan `go.mod` di akar akan membuat repository terlihat seolah-olah ia adalah project Go.

## Keputusan

**Satu repository, empat folder aplikasi sejajar.**

```
jualin_crm/
├── crm_be/              Go + Gin + PostgreSQL      Phase 0+
├── crm_dashboard/       Next.js                    Phase 3
├── crm_landing_page/    Next.js                    belum terjadwal
├── crm_employee/        Flutter                    Phase 5
│
├── docs/                dokumentasi seluruh produk
├── .claude/skills/      konvensi kode
├── CLAUDE.md
├── Makefile             entry point tunggal
├── docker-compose.yml   orkestrator lintas aplikasi
└── .gitignore
```

### Turunan

| Hal | Keputusan |
|---|---|
| Module path Go | `github.com/Pravasta/jualin-crm/crm_be` |
| `docker-compose.yml` | **Akar** — ia akan mengorkestrasi lebih dari satu aplikasi |
| `Makefile` | **Akar** — satu entry point, target mendelegasikan ke folder terkait |
| `.golangci.yml` | Di dalam `crm_be/` — milik satu aplikasi |
| CI workflow | Satu berkas per aplikasi, dengan `paths:` filter |
| `docs/` | **Akar** — dokumentasi produk, bukan milik satu aplikasi |
| `.gitignore` | **Akar** — satu berkas mencakup Go, Node, dan Flutter |

## Alasan

| | |
|---|---|
| **Satu produk, satu riwayat** | Perubahan yang menyentuh backend dan dashboard sekaligus bisa berada dalam satu PR dan satu review |
| **Kontrak API tidak bisa menyimpang diam-diam** | Perubahan bentuk response dan client yang memakainya terlihat berdampingan di diff yang sama |
| **Satu tempat dokumentasi** | `docs/` berlaku untuk seluruh produk; memecahnya per repo akan menduplikasi freeze dan ADR |
| **Akar tidak lagi menyamar sebagai project Go** | `go.mod` di akar menyesatkan — repository ini bukan project Go, melainkan produk dengan komponen Go |

### Kenapa bukan repository terpisah per aplikasi

Repository terpisah menuntut versioning kontrak antar-aplikasi, duplikasi dokumentasi, dan koordinasi rilis — semuanya biaya yang hanya terbayar bila aplikasi dikembangkan oleh tim yang berbeda dengan kecepatan yang berbeda. Belum ada kondisi itu.

> Ini **tidak** bertentangan dengan keputusan bahwa Jualin HRIS dan Jualin Invoice akan menjadi project terpisah (decisions §2, §57). Yang satu repository di sini adalah **empat permukaan dari satu produk**; yang terpisah nanti adalah **produk yang berbeda**.

## Konsekuensi

**Positif:** perubahan lintas aplikasi dalam satu PR · dokumentasi tidak terduplikasi · setup developer sekali jalan

**Negatif:**

| Konsekuensi | Mitigasi |
|---|---|
| CI berisiko menjalankan semua job untuk perubahan di satu aplikasi | `paths:` filter per workflow. **Wajib sejak workflow pertama** — menambahkannya belakangan berarti sudah membakar waktu CI berbulan-bulan. |
| Perintah harus dijalankan dari akar atau dengan `cd` | `Makefile` di akar sebagai entry point tunggal |
| Docker build context perlu ditunjuk eksplisit | `context: ./crm_be` di compose |
| Repository terlihat besar bagi pendatang baru | `README.md` di setiap folder aplikasi menjelaskan isinya dan kapan dikerjakan |

## Catatan implementasi

Folder `crm_dashboard/`, `crm_landing_page/`, dan `crm_employee/` dibuat sekarang hanya berisi `README.md` yang menyatakan **apa isinya dan phase berapa ia dimulai**.

Ini bukan pelanggaran Aturan #28 (*jangan buat folder untuk kebutuhan yang belum ada*): folder-folder itu bukan scaffolding kosong yang menyerupai kemajuan, melainkan penunjuk arah yang menjawab pertanyaan "di mana dashboard nanti diletakkan" tanpa seseorang harus menebak.

**Tidak ada berkas konfigurasi, dependensi, atau kode** yang dibuat di sana sampai phase-nya tiba.
