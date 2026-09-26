# Phase 8.7 — Pengaturan, Pengingat, Unduh Laporan, Mode Gelap · Issues

> Indeks. **Tanpa kolom status**: status hidup di [GitHub Issues](https://github.com/Pravasta/jualin-crm/milestone/14) (ADR-008).
> Apa & kenapa di [`prd.md`](./prd.md) · bagaimana di [`td.md`](./td.md).

**Milestone:** [Phase 8.7 — Pengaturan, Pengingat, Unduh Laporan, Mode Gelap](https://github.com/Pravasta/jualin-crm/milestone/14)

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [194](https://github.com/Pravasta/jualin-crm/issues/194) | Ubah profil, nama organization, ganti password (API) | `crm_be` | `PATCH /v1/me/profile`, `PATCH /v1/organization`, `POST /v1/auth/password/change`, Action `organization.update` | §2, §3, §7 |
| [195](https://github.com/Pravasta/jualin-crm/issues/195) | Preferensi notifikasi + pengingat jatuh tempo (API + worker) | `crm_be` | migration `0011`, `GET/PUT /v1/me/notification-preferences`, worker klaim `SKIP LOCKED`, push menghormati preferensi | §1, §2, §4 |
| [196](https://github.com/Pravasta/jualin-crm/issues/196) | Pengaturan dashboard | `crm_dashboard` | Profil, organization, keamanan, notifikasi | §5.1 |
| [197](https://github.com/Pravasta/jualin-crm/issues/197) | Mobile: pengaturan notifikasi + notifikasi pengingat | `crm_employee` | Dua saklar, tipe `task_due_soon` | §6 |
| [198](https://github.com/Pravasta/jualin-crm/issues/198) | Laporan: unduh CSV + cetak/PDF | `crm_dashboard` | `lib/report-csv.ts`, `@media print` | §5.2 |
| [199](https://github.com/Pravasta/jualin-crm/issues/199) | Mode gelap dashboard | `crm_dashboard` | Token `.dark` dihitung, varian status, tanpa kedip, pengalih | §5.3 |
| [200](https://github.com/Pravasta/jualin-crm/issues/200) | Dokumentasi + penutup Phase 8.7 | docs | `testing/flow/11`, review 8 AC | §8 |

## Urutan

```
#194 ──┐
       ├──► #196 ──┐
#195 ──┤           │
       └──► #197 ──┤
#198 ──────────────┼──► #199 ──► #200
                   │
```

| Dependensi | Sifat |
|---|---|
| #196 → #194, #195 | **Keras.** Layar memanggil endpoint yang lahir di sana |
| #197 → #195 | **Keras.** Endpoint preferensi dan tipe `task_due_soon` |
| #194 ‖ #195 ‖ #198 | **Paralel.** Paket dan berkas berbeda |
| #199 → #196, #198 | **Lunak.** Mode gelap menyentuh token dan banyak layar. Dikerjakan setelah layar yang berubah di phase ini selesai, supaya tidak ada dua PR yang bentrok di berkas yang sama. Bagian **Tampilan** di Pengaturan lahir di sini |
| #200 → semuanya | **Keras.** |

## Batas per issue

| Issue | Setelah selesai, yang **belum** ada |
|---|---|
| #194 | Endpoint bisa dipanggil lewat `curl`, belum ada layar |
| #195 | Pengingat berjalan dan push terkirim, tapi preferensi hanya bisa diubah lewat `curl` |
| #196 | Dashboard lengkap, tapi Employee belum bisa mengubah preferensinya |
| #197 | Mobile lengkap |
| #198 | Laporan bisa dibawa keluar |
| #199 | Mode gelap. **Mobile tetap terang** (PRD *Di luar cakupan*) |
| #200 | Phase tutup |
