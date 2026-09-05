# Phase 8.5 — Paket Berbayar & Kuota · Issues

> Indeks. **Tanpa kolom status** — status hidup di [GitHub Issues](https://github.com/Pravasta/jualin-crm/milestone/12) (ADR-008).
> Apa & kenapa di [`prd.md`](./prd.md) · bagaimana di [`td.md`](./td.md).

**Milestone:** [Phase 8.5 — Paket Berbayar & Kuota](https://github.com/Pravasta/jualin-crm/milestone/12)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [122](https://github.com/Pravasta/jualin-crm/issues/122) | Peta limit paket: kuota + seat, `limits`/`usage` di `GET /v1/me` | `crm_be` | `planLimits` di sebelah `planChannels`, `Limits`, gagal-tertutup-ke-`free`, `CountCreatedThisMonth`, `/v1/me` | §2, §3, §7 |
| [123](https://github.com/Pravasta/jualin-crm/issues/123) | Kuota lead: satu titik penegakan untuk tiga jalur + `plan_quota_exceeded` | `crm_be` | `PlanQuota` di `lead`, penegakan di `lead.Usecase.Create`, perilaku per-principal, notifikasi Owner | §3, §4, §5 |
| [124](https://github.com/Pravasta/jualin-crm/issues/124) | Batas seat + dua jalur perubahan paket (token internal & test checkout) | `crm_be` | `invitation.Usecase.Create`, `/internal/...` ber-token, `/v1/subscription/test-checkout` ber-flag, `audit_log` | §6, §8, §10, §11 |
| [125](https://github.com/Pravasta/jualin-crm/issues/125) | Dashboard: layar Langganan — paket aktif, pemakaian, perbandingan | `crm_dashboard` | Sidebar + `/subscription`, pembaca pertama `plan.code`, penanganan dua error baru | §9, §7 |
| [126](https://github.com/Pravasta/jualin-crm/issues/126) | Dokumentasi + penutup Phase 8.5 | `crm_be` + docs | `api.md`, `authorization.md`, `multi-tenancy.md`, anotasi ADR-012, `testing/flow/`, review 10 AC. **Penutup phase** | §12, §15 |

---

## Urutan

```
#122 ──┬──► #123 ──┐
       │           ├──► #125 ──► #126
       └──► #124 ──┘
```

| Dependensi | Sifat |
|---|---|
| #123 → #122 | **Keras.** Tidak ada yang bisa ditegakkan sebelum ada peta yang menjawab "batasnya berapa". |
| #124 → #122 | **Keras.** Batas seat membaca peta yang sama; perubahan paket memvalidasi terhadapnya. |
| #123 ‖ #124 | **Paralel.** Keduanya hanya bergantung pada #122, menyentuh domain berbeda (`lead` vs `invitation`+`subscription`), dan tidak ada berkas yang beririsan. Pertama kalinya sejak Phase 6 (#85 ‖ #86) ada dua issue yang benar-benar bisa dikerjakan bersamaan. |
| #125 → #123, #124 | **Keras.** Layar Langganan menampilkan pemakaian yang lahir di #122 tapi menangani dua error yang lahir di #123/#124, dan tombol test checkout-nya memanggil endpoint #124. |
| #126 → semuanya | **Keras.** Prosedur verifikasi hanya bisa ditulis setelah kuota benar-benar menolak sesuatu. |

**#122 dan #123 sengaja dipisah**, alasan yang sama dengan #112/#113 dan `apikey` #46/#47: satu issue
membangun **kapabilitasnya**, issue berikutnya membangun **yang memakainya**. Menggabungkannya
menghasilkan PR yang mencampur dua kelas review — *"apakah petanya benar"* dan *"apakah penegakannya
di tempat yang tepat"*.

---

## Batas per issue

| Issue | Setelah selesai, yang **belum** ada |
|---|---|
| #122 | Angka bisa dibaca dan tampil di `/v1/me`, tapi **nol titik yang menolak apa pun** — persis bentuk #112 |
| #123 | Kuota lead menolak sungguhan lewat `curl`, tapi **batas seat belum ada** dan **pelanggan tidak punya cara melihat sisa jatahnya** |
| #124 | Paket bisa dinaikkan dan seat dibatasi, tapi **hanya lewat `curl`** — tidak ada layar apa pun |
| #125 | Pelanggan melihat dan merasakan batasnya, tapi **dokumen arsitektur belum menyebut kuota** dan **belum ada prosedur verifikasi tertulis** |
| #126 | Phase 8.5 tutup. Payment service, downgrade otomatis, proration/trial/kupon **belum disentuh** — dan memang tidak boleh |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Dua hal yang harus dijawab sebelum issue-nya dikerjakan

| Yang belum ditutup | Memblokir | Siapa |
|---|---|---|
| **Angka provisional final** — kuota Free/Pro, batas seat, kanal per paket, harga Pro | **Rilis**, bukan implementasi. `planLimits` (#122) sudah berisi angka sementara (100/2.000 lead, 2/10 seat) yang ditandai `LimitsAreProvisional = true`; produksi menolak boot selama flag itu `true`. #126 memflipkannya bersama angka final dari pemilik produk | Pemilik produk |
