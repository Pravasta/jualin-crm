# Jualin CRM — Product & Architecture Decisions

## Status

**Status:** Final decision for architecture freeze  
**Tanggal:** 17 Agustus 2026  
**Tujuan:** Dokumen keputusan untuk diberikan ke Claude Code sebelum implementasi.

---

# 1. Keputusan Utama

Saya sudah mereview seluruh Architecture & Product Review.

Secara umum saya setuju dengan arah architecture dan product boundary yang direkomendasikan.

Untuk tahap ini, keputusan berikut dianggap **FINAL**, kecuali nanti ditemukan alasan teknis yang benar-benar kuat untuk mengubahnya melalui ADR.

---

# 2. Employee = Membership

**KEPUTUSAN: SETUJU**

Tidak perlu membuat entity/table `employees` khusus.

Employee adalah user yang mempunyai membership dengan role `employee`.

```text
User
  ↓
Membership
  ↓
Organization
  ↓
Role = owner | admin | manager | employee
```

Assignment Lead akan menunjuk ke `membership_id`, bukan langsung ke `user_id`.

---

# 3. Satu User Dapat Memiliki Banyak Organization

**KEPUTUSAN: YA**

Schema harus mendukung:

```text
User
 ├── Membership → Organization A
 └── Membership → Organization B
```

Namun untuk MVP:

- UI tidak perlu memiliki organization switcher.
- UX boleh mengasumsikan user bekerja pada satu organization aktif.
- Dukungan multi-organization cukup disiapkan di schema/domain model.

Jangan membuat feature organization switching sebelum ada kebutuhan nyata.

---

# 4. Lead → Customer

**KEPUTUSAN: CUSTOMER ADALAH ENTITY TERPISAH**

Jangan mengubah Lead menjadi Customer hanya dengan mengganti status.

Flow:

```text
Lead
  ↓
Qualified / Won
  ↓
Convert
  ↓
Customer
```

Lead tetap disimpan sebagai historical record.

Customer adalah entity baru.

Jika diperlukan, Lead dapat menyimpan `converted_customer_id`.

Alasan:

- Satu Customer dapat berasal dari beberapa Lead.
- Lead adalah event/source capture.
- Customer adalah relationship jangka panjang.
- Reporting conversion membutuhkan historical Lead.

---

# 5. Assignment

**KEPUTUSAN: `assigned_to` DI LEAD**

Tidak perlu entity/table `assignment` untuk MVP.

Gunakan:

```text
lead.assigned_to_membership_id
```

History assignment disimpan sebagai Activity.

Multi-assignee belum diperlukan.

---

# 6. Form Submission

**KEPUTUSAN: TIDAK ADA TABLE `form_submissions` DI MVP**

Flow:

```text
Form
  ↓
Submit
  ↓
Validate
  ↓
Lead
```

Submission yang valid langsung menjadi Lead.

Original payload disimpan pada:

```text
lead.raw_payload
```

Entity `FormSubmission` baru dipertimbangkan jika nanti ada kebutuhan nyata untuk submission yang ditolak/spam atau analytics submission yang lebih kompleks.

---

# 7. UUID

**KEPUTUSAN: UUIDv7**

Gunakan UUIDv7 sebagai primary key.

Tidak menggunakan serial/integer incremental sebagai public identifier.

---

# 8. Manager

**KEPUTUSAN: MANAGER MELIHAT SELURUH ORGANIZATION UNTUK MVP**

Tidak perlu membuat entity `Team` dulu.

```text
Owner
→ Full access

Admin
→ Operational management

Manager
→ Monitoring + CRM management

Employee
→ Assigned leads + own tasks
```

Manager tidak memiliki scope Team pada MVP.

Jangan membuat UI yang menggunakan konsep Team sebelum entity Team benar-benar dibuat.

---

# 9. Product Boundary

## Termasuk

```text
Lead
Customer
Task
Activity
Assignment
Sales Pipeline
Deal
Employee / Membership
Notification
Direct API
Embedded Form
Webhook
Reports
Subscription
```

## Tidak termasuk

```text
HRIS
Payroll
Attendance
Accounting
Inventory
ERP
Invoice generation
Manufacturing
Purchasing
Email marketing / campaign
WhatsApp inbox
Instagram inbox
Facebook inbox
Omnichannel chat
```

Jika sebuah feature mulai masuk ke domain tersebut, jangan langsung implementasikan. Flag sebagai scope discussion terlebih dahulu.

---

# 10. Product Catalog

**KEPUTUSAN: TUNDA**

Jangan membuat product/inventory system pada MVP.

Deal nantinya cukup memiliki informasi sederhana seperti:

```text
description
value
currency
status
closed_at
```

Tidak perlu SKU, stock, warehouse, variant, price tier, atau inventory.

---

# 11. Deal

**KEPUTUSAN: TUNDA SEDIKIT**

Deal tetap menjadi bagian dari roadmap Jualin CRM.

Namun core MVP paling awal adalah:

```text
Lead
 ↓
Customer
```

Deal minimal dapat dibangun setelah Lead, Assignment, Task, Activity, Customer, dan Dashboard dasar sudah stabil.

Jangan membuat Deal menjadi blocker untuk Phase 1/2.

---

# 12. Embedded Form

**KEPUTUSAN: IFRAME UNTUK MVP**

Gunakan:

```text
Customer Website
      ↓
iframe
      ↓
Jualin Form
      ↓
Submit
      ↓
Jualin CRM
      ↓
Lead
```

Jangan membuat inline JavaScript embed terlebih dahulu.

---

# 13. Public Form Authentication

**KEPUTUSAN: API KEY TIDAK BOLEH ADA DI BROWSER**

Embedded Form menggunakan:

```text
public_form_key
```

bukan API secret.

Direct API menggunakan secret API key.

Contoh:

```text
Direct API
→ jln_live_xxxxx

Embedded Form
→ public_form_xxxxx
```

Public form hanya boleh submit Lead ke form tersebut.

Tidak boleh:

```text
Read Lead
Update Lead
Read Customer
Manage Employee
Manage API Key
```

---

# 14. API Key

**KEPUTUSAN: API KEY MILIK ORGANIZATION**

API key tidak terikat langsung dengan employee.

Struktur minimal:

```text
key_id
secret_hash
name
scope
created_by_membership_id
created_at
revoked_at
last_used_at
```

Raw secret hanya ditampilkan satu kali ketika API key dibuat.

MVP boleh hanya menggunakan:

```text
leads:write
```

Namun struktur scope tetap disiapkan.

---

# 15. Webhook

**KEPUTUSAN: TUNDA**

Urutan integration:

```text
Manual
   ↓
Direct API
   ↓
Embedded Form
   ↓
Webhook
```

Jangan implement webhook sebelum model Lead dan Direct API sudah stabil.

Inbound dan outbound webhook sama-sama ditunda.

Architecture documentation tetap boleh mencatat requirement security seperti signature verification, replay protection, SSRF protection, retry, timeout, dan delivery log.

---

# 16. Subscription

**KEPUTUSAN: FREE PLAN TERSEDIA SEJAK AWAL**

Setiap Organization mendapatkan Free Plan.

Payment tidak perlu dibuat pada MVP awal.

Flow:

```text
Register
 ↓
Organization
 ↓
Free Subscription
 ↓
Gunakan CRM
 ↓
Upgrade
 ↓
Payment Service
 ↓
Subscription Updated
```

Payment service sudah tersedia secara terpisah.

Jangan membuat payment gateway implementation di project CRM.

CRM hanya perlu memiliki domain:

```text
Plan
Subscription
Usage / Limits
External Payment Reference
```

---

# 17. Multi-Tenant

**KEPUTUSAN: SHARED DATABASE + SHARED SCHEMA**

Gunakan:

```text
PostgreSQL
 └── shared schema
      └── organization_id
```

Setiap entity bisnis tenant-aware memiliki `organization_id`.

Tenant isolation melalui:

1. Tenant Context
2. Repository tenant-scoped
3. Composite tenant-aware foreign key
4. Tenant isolation tests

RLS PostgreSQL ditunda.

Prinsip penting:

> `organization_id` selalu berasal dari authenticated principal / tenant context dan tidak pernah berasal dari body atau query client.

---

# 18. Documentation System

**KEPUTUSAN: SETUJU**

Gunakan:

```text
CLAUDE.md

docs/
├── STATUS.md
├── product/
├── architecture/
├── features/
├── decisions/
└── brainstorming/

.claude/
└── skills/
```

`docs/STATUS.md` adalah project state ledger untuk mencatat:

- apa yang sudah selesai,
- apa yang sedang dikerjakan,
- apa berikutnya,
- technical debt,
- keputusan yang belum dibuat.

---

# 19. Feature Documentation

**KEPUTUSAN: 2 FILE PER FEATURE**

Gunakan:

```text
docs/features/<feature>/
├── spec.md
└── notes.md
```

`spec.md` berisi:

- Requirement
- Domain
- API
- Authorization
- Acceptance Criteria

`notes.md` berisi:

- Implementation decision
- Deviation from spec
- Technical debt
- Important notes untuk session berikutnya

Jangan membuat terlalu banyak file documentation kecuali feature memang besar.

---

# 20. Claude Skills

**KEPUTUSAN: MULAI DENGAN SATU SKILL**

Untuk sekarang:

```text
.claude/skills/jualin-backend/
```

Skill lain dibuat ketika phase tersebut dimulai.

Contoh:

```text
jualin-frontend
```

dibuat ketika dashboard mulai dikerjakan.

```text
jualin-mobile
```

dibuat ketika Flutter mulai dikerjakan.

Skill berisi bagaimana cara menulis kode di project ini. Dokumentasi berisi apa yang harus dibangun dan kenapa.

---

# 21. CLAUDE.md

CLAUDE.md harus tetap ringkas.

Target:

```text
< 150 lines
```

Hanya berisi:

- project identity
- scope
- stack
- rules yang tidak boleh dilanggar
- architecture principle
- workflow
- source of truth

Detail feature jangan dimasukkan ke CLAUDE.md.

---

# 22. Development Workflow

Pertahankan prinsip:

> **ONE FEATURE = ONE SESSION**

Workflow:

```text
New Session
 ↓
Read CLAUDE.md
 ↓
Read docs/STATUS.md
 ↓
Read relevant skill
 ↓
Read relevant architecture docs
 ↓
Read feature spec
 ↓
Inspect relevant code
 ↓
Create implementation plan
 ↓
Review / approval
 ↓
Implement
 ↓
Test
 ↓
Security review
 ↓
Update documentation
 ↓
Update STATUS.md
```

Jangan membaca seluruh project documentation jika tidak relevan dengan feature yang sedang dikerjakan.

---

# 23. MVP Core Flow

Core MVP:

```text
Register
 ↓
Email Verification
 ↓
Login
 ↓
Create Organization
 ↓
Invite Employee
 ↓
Employee Login
 ↓
Owner Create API Key
 ↓
External Website
 ↓
POST Lead
 ↓
Lead masuk CRM
 ↓
Owner / Manager assign Lead
 ↓
Employee menerima notification
 ↓
Employee membuka Flutter
 ↓
Employee follow-up
 ↓
Update Lead
 ↓
Convert Lead
 ↓
Customer
```

Ini adalah core product loop.

Setelah core loop stabil baru:

```text
Embedded Form
 ↓
Webhook
 ↓
Subscription
 ↓
Payment
 ↓
Advanced Pipeline
 ↓
Advanced Reports
 ↓
Automation
```

---

# 24. Roadmap

## Phase 0 — Foundation

- Go project
- Docker
- PostgreSQL
- Config
- Migration
- Logging
- Error handling
- Health check
- CI
- Basic testing infrastructure

## Phase 1 — Auth & Organization

- User
- Organization
- Membership
- Registration
- Email verification
- Login
- Refresh token
- Tenant context
- RBAC
- Employee invitation
- Tenant isolation test harness

## Phase 2 — CRM Core

- Lead
- Lead status
- Lead source
- Activity
- Task
- Assignment
- Customer
- Idempotency

## Phase 3 — Owner Dashboard

- Authentication UI
- Lead list
- Lead detail
- Employee management
- Assignment
- Task
- Basic metrics
- Basic CRM operations

Deal minimal dapat masuk setelah core CRM stabil.

## Phase 4 — Public API

- API Key
- API authentication
- `POST /v1/leads`
- Validation
- Rate limiting
- Idempotency
- API documentation

## Phase 5 — Employee Mobile

Flutter:

- Login
- My Leads
- My Tasks
- Lead detail
- Activity
- Follow-up
- Update status
- Push notification
- Read cache/offline support

## Phase 6 — Embedded Form

- Form
- Public form key
- Submit endpoint
- Domain allowlist
- Anti-spam
- iframe embed

## Phase 7 — Webhook

- Inbound webhook
- Outbound webhook
- Event
- Delivery
- Retry
- Signature
- Webhook log

## Phase 8 — Subscription

- Plan
- Subscription
- Usage
- Limits
- Upgrade
- External payment service integration

## Phase 9 — Advanced CRM

- Advanced pipeline
- Deal
- Reports
- Analytics
- Automation
- Feature berdasarkan kebutuhan customer nyata

---

# 25. Hal yang Belum Perlu Diputuskan Sekarang

Jangan memaksa keputusan final untuk:

- Pricing final
- Exact Free Tier limit
- Exact API rate limit
- Email provider
- Push provider detail
- Webhook retry detail
- Payment service contract detail
- Custom fields
- Team
- Round robin
- Advanced pipeline
- Advanced reports
- Automation
- Contact entity

Keputusan dibuat ketika feature terkait akan dikerjakan.

Prinsip:

> **Jangan melakukan premature design.**

---

# 26. Final Decisions Checklist

```text
✓ Standalone CRM
✓ Multi-tenant
✓ Shared PostgreSQL
✓ Shared schema
✓ Go monolith
✓ Next.js dashboard
✓ Flutter employee app
✓ Employee = Membership
✓ Multi-org supported di schema
✓ Manager melihat seluruh organization
✓ Lead dan Customer entity terpisah
✓ Assignment = field di Lead
✓ Activity menyimpan history
✓ Form Submission bukan entity MVP
✓ UUIDv7
✓ Direct API
✓ API Key organization-level
✓ Public Form Key berbeda dari API Key
✓ Embedded Form menggunakan iframe
✓ Webhook ditunda
✓ Subscription Free Tier
✓ Payment Service external
✓ Product/Inventory ditunda
✓ Deal ditunda sedikit setelah core CRM stabil
✓ No HRIS / ERP / Invoice / Accounting
✓ docs/STATUS.md
✓ 1 feature = 1 session
✓ 2 documentation files per feature
✓ Mulai dengan 1 backend skill
✓ RLS ditunda
✓ Tenant isolation test wajib
```

---

# 27. NEXT STEP

**Jangan coding dulu.**

Berdasarkan keputusan di atas, lakukan **final architecture freeze**.

Output berikutnya yang saya inginkan:

1. Final domain model
2. Final entity relationship
3. Final MVP scope
4. Final Phase 0–5 roadmap
5. Final architecture rules
6. Final documentation structure
7. List keputusan yang masih benar-benar blocking sebelum migration
8. Rekomendasi migration pertama

Setelah output tersebut saya review lagi.

Jika sudah tidak ada keputusan blocking, baru mulai:

```text
Bootstrap Documentation
        ↓
Session 1 — Foundation
```

Jangan langsung membuat production code sebelum architecture freeze selesai.
