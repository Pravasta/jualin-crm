# Project Status

> **Ledger state project.** Dibaca di **awal setiap session**, diperbarui di **akhir setiap session**.
> Ini satu-satunya jawaban atas pertanyaan *"sekarang sudah sampai mana?"* — jangan merekonstruksinya dari kode.

**Last updated:** 17 Agustus 2026 — Bootstrap Documentation
**Phase sekarang:** Pra-Phase 0 (belum ada kode)

---

## Selesai

| Item | Session | Phase | Catatan |
|---|---|---|---|
| Brainstorming & architecture review | — | — | `docs/brainstorming/` — arsip |
| Product & architecture decisions | — | — | `docs/product/decisions.md` |
| ADR-007 — kardinalitas User → Organization | — | — | Accepted |
| Architecture freeze + 6 amandemen | — | — | `docs/architecture/freeze.md` — 🔒 FROZEN |
| Bootstrap documentation | — | — | CLAUDE.md, STATUS.md, product/, architecture/, ADR-001..006, skill backend |
| Delivery workflow + setup repository | — | — | ADR-008, `docs/workflow.md`, template GitHub, git init, label & milestone |

---

## Sedang Dikerjakan

_(kosong)_

---

## Berikutnya

**Issue #1 — Project skeleton: config, logging, error, health, CI**

PRD & TD Phase 0 sudah ditulis, issue #1–#3 sudah dibuat. Setelah PR PRD/TD di-merge, pengerjaan dimulai dari issue #1.

- Cakupan & acceptance: [issue #1](https://github.com/Pravasta/jualin-crm/issues/1)
- TD: `docs/phases/00-foundation/td.md` §2–§7, §12–§14
- Urutan: #1 → #2 → #3 (berurutan, tidak bisa diparalelkan)

---

## Utang Teknis

_(kosong — belum ada kode)_

> Bagian ini sama pentingnya dengan bagian Selesai. Kompromi yang diambil di session 3 akan terlupa di session 12 lalu ditemukan kembali sebagai bug produksi.

---

## Keputusan Belum Diambil

Tidak ada yang memblokir. Semuanya diputuskan saat fitur terkait dikerjakan.

| Kode | Keputusan | Diputuskan sebelum |
|---|---|---|
| B6 | Daftar `lost_reason` final | Phase 2 |
| B7 | Boleh mengubah status lead mundur? | Phase 2 |
| B8 | Task boleh berdiri tanpa Lead? | Phase 2 |
| B9 | Konversi ke Customer otomatis saat `won`? | Phase 2 |
| — | Email provider (Resend / Postmark / SES) | Phase 1 — ⏳ *lead time, lihat bawah* |
| — | Domain final & branding | Phase 1 — ⏳ *lead time, lihat bawah* |
| — | Hosting & managed PostgreSQL | Phase 0 akhir |
| — | Bahasa UI (ID / EN / dwibahasa) | Phase 3 |
| — | Retensi data free tier | Phase 8 |
| — | Push provider detail | Phase 5 — ⏳ *lead time, lihat bawah* |
| — | Pricing final & limit free tier | Phase 8 |
| — | Kontrak integrasi payment service | Sebelum Phase 8 |

Rekomendasi untuk masing-masing ada di `docs/architecture/freeze.md` bagian 7 dan `docs/brainstorming/architecture_product_review.md` bagian 12.

---

## ⏳ Punya Lead Time — mulai lebih awal dari phase-nya

> Yang membuat ketiganya berisiko **bukan** pekerjaan kodingnya, melainkan **menunggu pihak lain**.
> Kalau baru diurus di hari pertama phase-nya, phase itu berhenti sebelum dimulai.

| Hal | Kode dipakai di | Mulai diurus | Kenapa |
|---|---|---|---|
| **Domain + email sender** (SPF/DKIM/DMARC) | Phase 1 | **Sekarang** | Verifikasi email menggerbangi login (keputusan B3), jadi ia jalur kritis Phase 1. Propagasi DNS dan pemanasan reputasi pengirim butuh waktu — dan email verifikasi yang masuk spam akan membunuh funnel registrasi **tanpa menghasilkan satu pun error**. |
| **Apple Developer Program** | Phase 5 | Sebelum Phase 3 | Enrollment bisa berhari-hari sampai berminggu (verifikasi identitas / D-U-N-S untuk organisasi). Tanpa ini, build iOS dan APNs tidak bisa jalan sama sekali. |
| **Firebase project (FCM)** | Phase 5 | Bersama Apple Developer | Pembuatannya cepat, tapi konfigurasi sisi iOS bergantung pada APNs key dari akun Apple di atas. |

**Tidak ada yang memblokir Phase 0.** Dicatat di sini justru supaya tidak tersadar terlambat.

### Status

- [ ] Domain final dipilih & dibeli
- [ ] Email provider dipilih (Resend / Postmark / SES)
- [ ] SPF, DKIM, DMARC terpasang & terverifikasi
- [ ] Apple Developer Program terdaftar
- [ ] Firebase project dibuat

> Domain juga menentukan konfigurasi cookie (`Secure`, `SameSite`, scope), CORS, dan alamat pengirim email — semuanya disentuh di Phase 1.

---

## Progress per Phase

> **Status per issue tidak dicatat di sini** — ia hidup di GitHub Issues (ADR-008).
> Dokumen ini hanya melacak level phase.

| Phase | Nama | PRD | TD | Issues | Selesai |
|---|---|---|---|---|---|
| 0 | Foundation | ✅ | ✅ | ✅ #1–#3 | ⬜ |
| 1 | Auth & Organization | ⬜ | ⬜ | ⬜ | ⬜ |
| 2 | CRM Core | ⬜ | ⬜ | ⬜ | ⬜ |
| 3 | Owner Dashboard | ⬜ | ⬜ | ⬜ | ⬜ |
| 4 | Public API | ⬜ | ⬜ | ⬜ | ⬜ |
| 5 | Employee Mobile | ⬜ | ⬜ | ⬜ | ⬜ |

Pekerjaan yang sedang berjalan: `gh issue list --state open`
