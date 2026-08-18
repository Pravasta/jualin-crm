# Project Status

> **Ledger state project.** Dibaca di **awal setiap session**, diperbarui di **akhir setiap session**.
> Ini satu-satunya jawaban atas pertanyaan *"sekarang sudah sampai mana?"* — jangan merekonstruksinya dari kode.

**Last updated:** 18 Agustus 2026 — Issue #3 selesai, Phase 0 tutup
**Phase sekarang:** Phase 0 — Foundation **selesai** (3/3 issue). Berikutnya: buka Phase 1.

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
| Restrukturisasi monorepo | — | — | ADR-009 — `crm_be/`, `crm_dashboard/`, `crm_landing_page/`, `crm_employee/` |
| **Issue #1 — Project skeleton** | — | 0 | PR [#5](https://github.com/Pravasta/jualin-crm/pull/5). Config ter-validasi saat boot, logger + request_id, error envelope, `/health`, graceful shutdown, CI. |
| **Issue #2 — Database, Docker Compose, migration** | — | 0 | `db.InTx`, `cmd/migrate` (goose), `0001_baseline`, Dockerfile + `docker-compose.yml` di akar, `/health/ready`. Diverifikasi end-to-end: `docker compose up` tanpa langkah manual, migration up/down bersih, `/health/ready` degradasi & pulih otomatis saat DB mati/hidup. |
| **ADR-010 — Fail-fast startup** | — | — | Muncul dari review PR #6, dikonfirmasi pemilik produk sebagai prinsip umum (bukan hanya DB). Aturan #36 di `CLAUDE.md`. |
| **Issue #3 — Test harness PostgreSQL asli** | — | 0 | `internal/shared/db/dbtest` (subpaket terpisah dari `db` produksi — testcontainers tidak ikut ter-link ke binary). Mengotomasi `db.InTx`, migration round-trip, `/health/ready` yang tadinya manual. **Phase 0 selesai.** |

---

## Sedang Dikerjakan

_(kosong)_

---

## Berikutnya

**Buka Phase 1 — Auth & Organization**

Phase 0 selesai. Sebelum implementasi apapun: tulis `docs/phases/01-auth-organization/{prd,td,issues}.md` (prosedur: `docs/workflow.md` bagian 1), lalu buat issue-nya di GitHub dengan milestone Phase 1.

Cakupan sesuai `docs/architecture/freeze.md` bagian 4 (Phase 1): `organizations`, `users`, `memberships`, `invitations`, token tables, `subscriptions` minimal, `audit_logs` · registrasi atomik · verifikasi email · login · refresh + rotasi · undangan employee · penonaktifan membership · tenant context · RBAC · **harness test isolasi tenant** (freeze lapis 4 — dibangun di atas `dbtest` dari issue #3).

Keputusan identity (B1–B5) sudah final di freeze bagian 7 — tidak ada yang memblokir migration `0002`.

---

## Utang Teknis

| Item | Dari | Catatan |
|---|---|---|
| Tidak ada test end-to-end otomatis untuk graceful shutdown | Issue #1 | Diverifikasi manual (build binary + SIGINT). Otomatisasi butuh test yang menjalankan binary sungguhan dan mengirim sinyal OS — `go run` tidak meneruskan sinyal ke child process, jadi tidak bisa diuji lewat itu. Belum ada issue yang mencakup ini; angkat saat menyentuh area shutdown lagi. |
| Tidak ada auto-migrate saat container `api` start | Issue #2 | `make migrate-up` dijalankan manual. Sengaja dipisah dari entrypoint `api` — migration dan serving punya kelas kegagalan berbeda. |

> ~~Test otomatis `db.InTx` dan migration round-trip~~ — selesai di issue #3.

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
| 0 | Foundation | ✅ | ✅ | ✅ #1–#3 | ✅ |
| 1 | Auth & Organization | ⬜ | ⬜ | ⬜ | ⬜ |
| 2 | CRM Core | ⬜ | ⬜ | ⬜ | ⬜ |
| 3 | Owner Dashboard | ⬜ | ⬜ | ⬜ | ⬜ |
| 4 | Public API | ⬜ | ⬜ | ⬜ | ⬜ |
| 5 | Employee Mobile | ⬜ | ⬜ | ⬜ | ⬜ |

Pekerjaan yang sedang berjalan: `gh issue list --state open`
