# Phase 3 — Owner Dashboard · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 3 — Owner Dashboard"`

**Milestone:** [Phase 3 — Owner Dashboard](https://github.com/Pravasta/jualin-crm/milestone/4)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [30](https://github.com/Pravasta/jualin-crm/issues/30) | CORS + endpoint metrik | `crm_be` | Middleware CORS + config fail-fast, `internal/metrics` (2 endpoint agregat), `metrics.read` | §1, §2 |
| [31](https://github.com/Pravasta/jualin-crm/issues/31) | Setup Next.js, klien API, sesi, auth UI | `crm_dashboard` | Proyek + Tailwind + shadcn/ui + CI, klien API (CSRF + refresh single-flight), proteksi route, 5 layar auth + pilih organization | §3, §4, §5 |
| [32](https://github.com/Pravasta/jualin-crm/issues/32) | Daftar lead: filter, pencarian, pagination | `crm_dashboard` | Layar traffic tertinggi. Filter status/pemilik/sumber/periode/kata kunci + **"lead tanpa pemilik aktif"** | §7.1, §8 |
| [33](https://github.com/Pravasta/jualin-crm/issues/33) | Detail lead: timeline, activity, task, status, assignment, konversi | `crm_dashboard` | Layar traffic tertinggi kedua; hampir seluruh aksi tulis | §5, §6, §8 |
| [34](https://github.com/Pravasta/jualin-crm/issues/34) | Tim: anggota, undangan, penonaktifan, notifikasi | `crm_dashboard` | Area admin. Penonaktifan dengan alur `on_open_leads` tiga cabang | §5, §8 |
| [35](https://github.com/Pravasta/jualin-crm/issues/35) | Home metrik, customer, daftar task, settings | `crm_dashboard` | Layar top-level tersisa; mengonsumsi endpoint dari #30. **Penutup phase** | §2.1, §8 |
| [40](https://github.com/Pravasta/jualin-crm/issues/40) | Fondasi desain: token warna, label Indonesia, app shell | `crm_dashboard` | **Ditambahkan setelah hasil desain masuk.** Dikerjakan setelah #31, sebelum #32 | design-brief §4, §6, §7.6 |

> **#40 tidak ada saat phase ini dibuka.** Ia muncul ketika hasil Claude Design masuk: desainnya
> mencakup seluruh layar #32–#35 sekaligus, sementara token warna, peta label, dan kerangka aplikasi
> dipakai bersama oleh semuanya dan tidak dimiliki satu pun dari mereka — design brief §7.6 memang
> sudah menandai kerangka aplikasi sebagai *"belum ada dan dibutuhkan"*. Dipisah agar empat issue
> berikutnya tinggal mengisi layar. Urutannya: **#31 → #40 → #32**.

---

## Urutan

```
#30 backend ──► #31 fondasi + auth ──► #40 fondasi desain ──► #32 daftar lead ──► #33 detail lead ──► #34 tim ──► #35 penutup
```

| Dependensi | Sifat |
|---|---|
| #31 → #30 | **Keras.** Tanpa CORS, tidak ada satu pun request browser yang sampai ke handler. |
| #32 → #31 | **Keras.** Butuh sesi, klien API, dan proteksi route. |
| #33 → #32 | **Praktis.** Detail dibuka dari daftar; komponen tampilan lead (badge status, nama pemilik) lahir di #32 dan dipakai ulang. |
| #34 → #31 | **Keras** ke #31 saja. Tidak bergantung pada #32/#33 — bisa ditukar dengan keduanya bila perlu. |
| #35 → #30 | **Keras** untuk bagian metrik (endpointnya dibuat di sana). |

**#34 dan #35 boleh ditukar.** Sisanya berurutan.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #30 | **Tidak ada satu baris UI.** Hanya Go: CORS, metrik, authz. Diverifikasi lewat test dan `curl` dengan header `Origin`. |
| #31 | Bisa mendaftar, verifikasi, masuk, keluar — **tidak ada layar lead sama sekali**. Setelah masuk, halaman utama boleh kosong. |
| #32 | Daftar lead bisa dilihat dan disaring. **Mengklik satu lead belum membuka apa pun** — itu #33. |
| #33 | Satu lead bisa dikelola sepenuhnya. **Belum ada layar tim, metrik, atau customer.** |
| #34 | Tim bisa dikelola dan notifikasi terbaca. **Belum ada angka.** |
| #35 | Phase 3 tutup. Mobile (Phase 5) dan API publik (Phase 4) **tidak** disentuh. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Tiga issue yang perlu perhatian ekstra saat review

**#30 — tidak menghasilkan layar apa pun.** Nilainya seluruhnya ada di dua hal yang gagal secara senyap:
konfigurasi CORS yang salah membuat dashboard mati total **tanpa satu error pun di sisi server**, dan
metrik yang salah menghitung akan menampilkan angka yang terlihat masuk akal tetapi bohong. Yang layak
direview di PR itu adalah **definisi `conversion_rate` dan waktu respons** (TD §2.2, §2.3) — apakah
`spam`/`unqualified` benar-benar keluar dari **penyebut**, dan apakah lead yang belum tersentuh
benar-benar dikecualikan dari rata-rata alih-alih dihitung nol.

**#31 — refresh single-flight adalah bagian yang sebenarnya sulit.** Layar auth-nya sendiri lurus. Yang
tidak lurus: enam request yang menerima `401` bersamaan harus menghasilkan **tepat satu** panggilan
refresh. Bila tidak, rotasi refresh token (#10) membaca token yang sudah dirotasi sebagai reuse attack
dan mencabut seluruh `family_id` — pengguna terlempar keluar, dan gejalanya terlihat seperti "aplikasi
mengeluarkan saya sendiri", bukan seperti bug klien. Ini kegagalan yang **hanya muncul di bawah
konkurensi**, persis seperti alokasi `lead_number` di #19: test berurutan akan hijau meski logikanya
salah total. **Yang direview adalah apakah test-nya benar-benar paralel.**

**#34 — penonaktifan anggota membawa keputusan, bukan konfirmasi.** `DELETE /v1/memberships/{id}` menolak
default bila anggota masih memegang lead terbuka, dan mengembalikan `409` beserta jumlahnya (#22). Layar
**tidak boleh** mengubahnya menjadi dialog "Yakin? [Ya]/[Batal]" — pengguna harus memilih secara sadar
antara melepas assignment dan memindahkannya ke orang lain. Menyembunyikan pilihan itu di balik satu
tombol berarti membuang seluruh alasan aturan tersebut ada (freeze 2.3): lead yang tetap ter-assign ke
orang yang tidak bisa login lagi tidak muncul di "My Leads" siapa pun dan tidak tertangkap filter "belum
ter-assign".

---

## Setelah keenamnya selesai

Phase 3 tutup bila **13 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi — terutama #1: *Owner
menyelesaikan seluruh core loop tanpa menyentuh terminal*. Lalu:

1. `api.md`, `authentication.md`, `authorization.md`, `multi-tenancy.md` diperbarui (TD §11)
2. `docs/STATUS.md` — Phase 3 ✅
3. **Demo ke calon pengguna** — ini tujuan phase-nya (freeze bagian 4), bukan langkah opsional setelahnya
4. Buka **Phase 4 — Public API** atau **Phase 5 — Employee Mobile**

> Freeze menempatkan Phase 4 dan Phase 5 sebagai **tidak saling bergantung**; Phase 5 hanya bergantung
> praktis pada Phase 3 (Owner butuh jalan untuk membuat dan meng-assign lead agar ada yang muncul di
> aplikasi employee). Urutan berikutnya ditentukan setelah demo — apa yang calon pengguna minta lebih
> berhak menentukannya daripada tebakan hari ini.
