# Jualin CRM — Documentation

Dokumentasi produk dan teknis untuk **Jualin CRM** — CRM SaaS multi-tenant standalone.

**Status:** Architecture frozen · Bootstrap documentation selesai · Belum ada kode.
**Berikutnya:** Session 1 — Foundation.

---

## Mulai dari mana

| Kalau Anda... | Baca |
|---|---|
| Memulai session baru | [`../CLAUDE.md`](../CLAUDE.md) → [`STATUS.md`](./STATUS.md) |
| Ingin tahu sudah sampai mana | [`STATUS.md`](./STATUS.md) |
| Perlu keputusan arsitektur yang mengikat | [`architecture/freeze.md`](./architecture/freeze.md) |
| Menulis kode Go | `.claude/skills/jualin-backend/` |
| Menulis kode dashboard (Next.js) | `.claude/skills/jualin-dashboard/` |
| Ragu sebuah fitur masuk scope | [`product/scope.md`](./product/scope.md) |
| Ragu penamaan | [`product/glossary.md`](./product/glossary.md) |

---

## Struktur

```
CLAUDE.md                    aturan global, < 150 baris

docs/
├── STATUS.md                ⭐ state ledger — update SETIAP akhir session
├── product/
│   ├── decisions.md         keputusan pemilik produk
│   ├── scope.md             in/out + batas yang akan diuji
│   └── glossary.md          ⭐ definisi istilah — cegah drift penamaan
├── architecture/
│   ├── freeze.md            🔒 acuan tunggal
│   ├── multi-tenancy.md     4 lapis isolasi
│   └── api.md               konvensi API
├── features/                <feature>/spec.md + notes.md
├── decisions/               ADR-001 … ADR-007
└── brainstorming/           arsip, bukan acuan

.claude/skills/
├── jualin-backend/          konvensi menulis kode Go
└── jualin-dashboard/        konvensi menulis kode Next.js
```

---

## Architectural Decision Records

| ADR | Keputusan | Status |
|---|---|---|
| [001](./decisions/ADR-001-monolith.md) | Go monolith — satu binary, satu database | ✅ Accepted |
| [002](./decisions/ADR-002-multi-tenancy.md) | Shared schema + 4 lapis isolasi; RLS ditunda | ✅ Accepted |
| [003](./decisions/ADR-003-employee-as-membership.md) | Employee = Membership, bukan entity | ✅ Accepted |
| [004](./decisions/ADR-004-api-key-format.md) | Format API key; kenapa SHA-256 bukan argon2 | ✅ Accepted |
| [005](./decisions/ADR-005-public-form-key.md) | Public form key terpisah dari API key | ✅ Accepted |
| [006](./decisions/ADR-006-lead-status-as-pipeline.md) | `lead.status` adalah pipeline di MVP | ✅ Accepted |
| [007](./decisions/ADR-007-user-organization-cardinality.md) | Produk 1:1, schema 1:N, tanpa `UNIQUE(user_id)` | ✅ Accepted |

**Perubahan arsitektur hanya melalui ADR baru.** Freeze tidak diubah tanpa catatan.

---

## Konteks Produk

| | |
|---|---|
| **Produk** | Jualin CRM — CRM SaaS multi-tenant, **standalone** |
| **Strategi** | Price-oriented. Biaya infrastruktur per tenant harus tetap rendah. |
| **Alur inti** | Lead Capture → Lead Management → Assignment → Follow-up → Customer → Deal |
| **Backend** | Go + Gin + PostgreSQL, monolith |
| **Dashboard** | Next.js (owner / admin / manager) |
| **Mobile** | Flutter (employee) |
| **Payment** | Service terpisah yang sudah ada — CRM hanya menyimpan referensi |

**Di luar cakupan:** HRIS · ERP · Accounting · Inventory · Payroll · Invoice generation · Email campaign · Chat inbox

---

## Roadmap

| Phase | Isi | Status |
|---|---|---|
| 0 | Foundation | ⬜ Berikutnya |
| 1 | Auth & Organization | ⬜ |
| 2 | CRM Core | ⬜ |
| 3 | Owner Dashboard | ⬜ |
| 4 | Public API | ⬜ |
| 5 | Employee Mobile | ⬜ |
| 🚦 | **Gate — cari 3–5 pengguna nyata** | |
| 6–9 | Form · Webhook · Subscription · Advanced | ⬜ |

Detail dan acceptance criteria: [`architecture/freeze.md`](./architecture/freeze.md) bagian 4.

---

## Arsip

`brainstorming/` berisi jejak keputusan — konsep awal, dua putaran review, dan analisis yang mendasari freeze. **Bukan acuan.** Dibaca hanya bila perlu memahami *kenapa* sebuah keputusan diambil dan ADR-nya tidak menjelaskan cukup.
