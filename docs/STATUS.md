# Project Status

> **Ledger state project.** Dibaca di **awal setiap session**, diperbarui di **akhir setiap session**.
> Ini satu-satunya jawaban atas pertanyaan *"sekarang sudah sampai mana?"* — jangan merekonstruksinya dari kode.

**Last updated:** 21 Agustus 2026 — Issue #11 selesai, **Phase 1 selesai**
**Phase sekarang:** Phase 2 — CRM Core (belum dimulai — PRD+TD berikutnya)

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
| **Issue #8 — Schema 0002, tenant context, pola repository, test katalog** | — | 1 | 9 tabel identity, `internal/shared/tenant`, `db.Querier`, `internal/membership` + `internal/user` sebagai contoh repository tenant-scoped/global. Test katalog **diverifikasi bisa gagal** secara adversarial (lihat notes.md). |
| **Issue #9 — Registrasi atomik, argon2id, verifikasi email** | — | 1 | `POST /v1/auth/{register,verify-email,verify-email/resend}`. `httpx.DomainError` (mekanisme error domain generik, baru), `internal/{organization,subscription,auditlog,auth}`, `internal/shared/{password,token,mailer,ratelimit}`. Registrasi 6-insert atomik dalam satu `db.InTx`; email di luar transaksi (Aturan #32). Rate limit **dibuktikan aktif** di test HTTP. |
| **ADR-011 — Layering per-paket + Unit of Work** | — | — | Diminta pemilik produk di tengah Phase 1, diprioritaskan sebelum #10. Merevisi Aturan #8, menegakkan Aturan #11 yang sejak awal dilanggar. |
| **Issue #15 — Refactor: layering, interface, Unit of Work** | — | 1 | `internal/auth` dipecah jadi `entity/port/usecase/repository_postgres/handler_http.go`. `Store`/`Repos` (Unit of Work) menggantikan `db.InTx` langsung di usecase. Composition root di `cmd/api/auth_store.go`. **7 unit test baru lolos tanpa Docker** (dibuktikan lewat `DOCKER_HOST` tak valid + inspeksi `go list -deps`). 33 test lama lolos tanpa perubahan asersi. |
| **Issue #10 — Login, refresh rotation, logout, reset password, CSRF, GET /v1/me** | — | 1 | `POST /v1/auth/{login,refresh,logout,password/forgot,password/reset}`, `GET /v1/me`. Access token JWT (`internal/shared/accesstoken`) + refresh token opaque dengan rotasi & deteksi penggunaan ulang (`SELECT ... FOR UPDATE`, revoke seluruh `family_id`). Dashboard = cookie `HttpOnly`; Mobile = body — dibuktikan lewat test HTTP & smoke test manual. CSRF double-submit (`httpx.VerifyCSRF`) untuk request cookie non-GET. `LoginLimiter` (backoff progresif). `docs/architecture/authentication.md` baru. **12 test integrasi + 13 unit test baru** (plus 5 test `LoginLimiter`, 4 test `accesstoken`), semuanya lolos tanpa perubahan asersi lama. |
| **Issue #11 — RBAC, invitation, penonaktifan membership, harness isolasi tenant** | — | 1 | `internal/shared/{authn,authz}` (session middleware diekstrak dari `auth`, RBAC baru). `internal/membership` naik jadi domain penuh (`port/usecase/handler_http.go`). `internal/invitation` baru — undangan dua cabang (B4), keduanya diuji termasuk test keamanan wajib. `POST/GET/DELETE /v1/invitations`, `GET/PATCH/DELETE /v1/memberships`. Penonaktifan membership mencabut refresh token dalam transaksi yang sama — **dibuktikan lewat smoke test end-to-end**. `docs/architecture/authorization.md` baru. **Harness isolasi tenant (`cmd/api/tenant_isolation_test.go`) dibangun & dibuktikan bisa gagal** (lapis 4, blocking CI). Satu bug nyata (email tidak terverifikasi otomatis saat accept undangan) ditemukan lewat smoke test manual, diperbaiki, dan mendapat test regresi di kedua level. **Phase 1 selesai.** |

---

## Sedang Dikerjakan

_(kosong)_

---

## Berikutnya

**Phase 2 — CRM Core**

- Phase 1 (Auth & Organization) selesai — #8, #9, #10, #11, #15, plus ADR-011.
- Sesuai `docs/workflow.md`/ADR-008: mulai dengan PRD + TD Phase 2, **bukan** langsung issue implementasi. Cakupan Phase 2 per freeze bagian 3.1/4: Lead (CRUD, status + transisi, source, filter, pagination, idempotency, `lead_number`, `version`), Assignment manual ke membership, Activity append-only, Task, Customer (konversi dari Lead), Notification.
- `cmd/api/tenant_isolation_test.go`'s `[]isolationCase` sudah siap menerima entri `lead` begitu endpoint-nya ada — lihat notes.md `## #11`.
- `internal/shared/authz`'s matriks baru mencakup Membership; Phase 2 menambah `Action` untuk Lead/Customer/dst mengikuti `architecture_product_review.md` §6.2.

---

## Utang Teknis

| Item | Dari | Catatan |
|---|---|---|
| Tidak ada test end-to-end otomatis untuk graceful shutdown | Issue #1 | Diverifikasi manual (build binary + SIGINT). Otomatisasi butuh test yang menjalankan binary sungguhan dan mengirim sinyal OS — `go run` tidak meneruskan sinyal ke child process, jadi tidak bisa diuji lewat itu. Belum ada issue yang mencakup ini; angkat saat menyentuh area shutdown lagi. |
| Tidak ada auto-migrate saat container `api` start | Issue #2 | `make migrate-up` dijalankan manual. Sengaja dipisah dari entrypoint `api` — migration dan serving punya kelas kegagalan berbeda. |
| `ratelimit.FixedWindow` tidak pernah membersihkan key lama | Issue #9 | Map tumbuh tanpa batas seiring IP/email baru muncul. Tidak masalah di volume MVP; perlu eviction sebelum traffic produksi nyata. |
| Angka rate limit (register 5/jam, resend 3/jam+10/jam) belum final | Issue #9 | Cukup untuk membuktikan mekanisme aktif, bukan hasil tuning. Freeze mencatat "strategi rate limit final" sebagai keputusan terbuka hingga Phase 4. |

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
| 1 | Auth & Organization | ✅ | ✅ | ✅ #8–#11, #15 | ✅ |
| 2 | CRM Core | ⬜ | ⬜ | ⬜ | ⬜ |
| 3 | Owner Dashboard | ⬜ | ⬜ | ⬜ | ⬜ |
| 4 | Public API | ⬜ | ⬜ | ⬜ | ⬜ |
| 5 | Employee Mobile | ⬜ | ⬜ | ⬜ | ⬜ |

Pekerjaan yang sedang berjalan: `gh issue list --state open`
