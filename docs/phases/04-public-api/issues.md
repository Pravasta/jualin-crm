# Phase 4 — Public API · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 4 — Public API"`

**Milestone:** [Phase 4 — Public API](https://github.com/Pravasta/jualin-crm/milestone/5)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [46](https://github.com/Pravasta/jualin-crm/issues/46) | Migration `0005`, domain `api_key`, CRUD kredensial | `crm_be` | `0005_api_keys` + `leads.source_api_key_id`, `internal/apikey`, format `jln_live_` (ADR-004), 3 endpoint pengelolaan, audit log | §1, §2, §9, §11, §15 |
| [47](https://github.com/Pravasta/jualin-crm/issues/47) | Autentikasi API key, `POST /v1/leads` publik, rate limit, idempotency | `crm_be` | Pemilahan kredensial, otorisasi berbasis scope, jalur API pada `POST /v1/leads`, header `X-RateLimit-*`, retensi `idempotency_key`. **Risiko keamanan tertinggi phase ini** | §3–§8, §10, §14 |
| [48](https://github.com/Pravasta/jualin-crm/issues/48) | Dashboard: manajemen API key | `crm_dashboard` | Layar buat/daftar/cabut, raw secret sekali dengan tombol salin, Owner/Admin saja | §9, §12 |
| [49](https://github.com/Pravasta/jualin-crm/issues/49) | Halaman dokumentasi integrasi | `crm_dashboard` | Halaman menghadap pelanggan + bab **API Publik** di `api.md`. **Penutup phase** | §13, §14, §18 |

---

## Urutan

```
#46 kredensial ──┬──► #47 autentikasi + endpoint publik ──► #49 dokumentasi (penutup)
                 │
                 └──► #48 layar dashboard   (paralel, bukan prasyarat #47)
```

| Dependensi | Sifat |
|---|---|
| #47 → #46 | **Keras.** Tidak ada yang bisa diautentikasi sebelum kredensialnya ada. |
| #48 → #46 | **Keras** ke #46 saja. Layar hanya memakai tiga endpoint pengelolaan; ia tidak peduli apakah kunci sudah bisa dipakai. |
| #48 → #47 | **Bukan prasyarat.** Boleh dikerjakan paralel. |
| #49 → #47 | **Keras.** Halaman ini mendokumentasikan perilaku nyata endpoint publik. Ditulis sebelum #47 selesai, ia hanya mendokumentasikan rencana. |

**#47 dan #48 boleh ditukar atau dikerjakan paralel.** Sisanya berurutan.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #46 | Kunci bisa dibuat, dilihat, dan dicabut lewat `curl` **sebagai user**. Kunci itu **belum bisa dipakai untuk apa pun** — tidak ada jalur autentikasi yang menerimanya. |
| #47 | `curl` dari mesin luar membuat lead. **Belum ada satu baris UI**; seluruh verifikasi lewat test dan `curl`. |
| #48 | Owner mengelola kunci tanpa terminal. **Belum ada halaman dokumentasi.** |
| #49 | Phase 4 tutup. Mobile (Phase 5), embedded form (Phase 6), dan webhook (Phase 7) **tidak** disentuh. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Dua issue yang perlu perhatian ekstra saat review

**#47 — satu peta otorisasi salah, dan kredensial yang bocor menjadi pengambilalihan organization.**
Ini bukan hiperbola: bila `authz` mengizinkan lebih dari `lead.create` untuk `PrincipalAPIKey`, sebuah
kunci yang tertempel di repositori publik pelanggan bisa mengundang anggota, mengubah role, dan membuat
kunci baru. Freeze menamai kelas kegagalan ini secara eksplisit (bagian 5.1, Aturan #24) — itu sebabnya
ia punya ADR sendiri.

Yang layak direview di PR itu **bukan** jumlah test yang lolos, melainkan **bentuk** dua hal:

1. Apakah penolakan datang dari *"action ini tidak ada di peta"* (gagal aman — endpoint baru yang lupa
   memikirkan API key otomatis tertutup) atau dari *"handler ini mengecek"* (gagal terbuka — endpoint
   yang lupa otomatis terbuka).
2. Apakah test-nya berupa **tabel atas seluruh `authz.Action`**, atau daftar action yang ditulis tangan.
   Yang kedua hijau hari ini dan bocor di phase berikutnya, saat seseorang menambah action baru dan
   tidak ada yang mengingatkannya.

**#49 — dokumentasi adalah fitur, dan kriterianya bukan "halaman ada".** Kriterianya *"integrasi bekerja
tanpa bertanya kepada siapa pun"*, dan satu-satunya cara memverifikasinya adalah **mengikuti halaman itu
dari nol**, bukan membacanya ulang. Membaca ulang dokumen yang kita tulis sendiri selalu terasa jelas —
justru karena kita sudah tahu bagian yang tidak tertulis di sana.

Di produk berharga murah, setiap pertanyaan yang halaman ini gagal jawab akan datang sebagai tiket
support, dan margin per pelanggan tidak menyediakan ruang untuk itu.

---

## Setelah keempatnya selesai

Phase 4 tutup bila **13 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi — terutama #1 (`curl`
dari mesin luar → lead di dashboard) dan #5 (API key tidak bisa memanggil satu pun endpoint aplikasi
pengguna). Lalu:

1. `api.md` (bab API Publik), `authentication.md`, `authorization.md`, `multi-tenancy.md` diperbarui (TD §18)
2. `docs/STATUS.md` — Phase 4 ✅, dan utang retensi `idempotency_key` **ditutup**
3. Buka **Phase 5 — Employee Mobile** — satu-satunya phase MVP yang tersisa

> **Sebelum membuka Phase 5, periksa `STATUS.md` bagian *Punya Lead Time*.** Pendaftaran Apple Developer
> Program dan pembuatan Firebase project belum dimulai, dan keduanya menunggu pihak ketiga —
> enrollment bisa berhari-hari sampai berminggu. Phase 5 bisa dimulai tanpa keduanya, tetapi akan
> berhenti tepat di bagian build iOS dan push notification. Mengurusnya **selama** Phase 4 berjalan
> adalah cara termurah menghindari itu.
