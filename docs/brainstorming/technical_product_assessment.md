# Jualin CRM — Technical & Product Assessment

> ## ⚠️ SEBAGIAN TERSALIP
>
> Dokumen ini ditulis **sebelum** Project Context & Brainstorming Directive diterbitkan.
> Beberapa rekomendasi di sini **sudah tidak berlaku**. Lihat
> [`architecture_product_review.md`](./architecture_product_review.md) bagian 0 untuk daftar lengkapnya.
>
> Ringkasan yang berubah:
>
> | Bagian | Status |
> |---|---|
> | 4.2 — struktur modul `platform/` vs `crm/` | ❌ **Dibatalkan.** Jualin CRM adalah project standalone; gunakan `internal/<domain>/` datar. |
> | 4.5 — domain `app.jualin.id/crm` sebagai shell multi-produk | ❌ **Dibatalkan.** Domain berdiri sendiri. |
> | 3.3 — `subscriptions.product` | ❌ **Dibatalkan.** Tidak ada produk lain di database ini. |
> | 6.2 & Open Question #14 — pemilihan payment gateway | ❌ **Dibatalkan.** Payment service sudah ada dan terpisah. |
> | 3.4 — `custom_fields JSONB` sejak migration pertama | ⚠️ **Diturunkan.** Murah ditambahkan nanti; tidak perlu di MVP. |
> | Bagian 4.3, 5.1, 5.2, 5.4 (isolasi tenant, form public key, API key lookup, idempotency) | ✅ **Tetap berlaku** dan diperkuat. |
>
> Selebihnya masih valid sebagai analisis pendukung.

---

> **Status:** Sebagian tersalip — lihat banner di atas
> **Tanggal:** 17 Agustus 2026
> **Sumber:** Analisis atas [`crm_saas_brainstorming.md`](./crm_saas_brainstorming.md)
> **Tujuan:** Menjadi dasar penyusunan Domain Model Spec, API Contract, dan PRD sebelum implementasi dimulai.

**Dokumen ini belum final.** Bagian [8. Open Questions](#8-open-questions) berisi keputusan yang masih harus diambil oleh pemilik produk. Tujuh pertanyaan bertanda 🔴 memblokir penulisan migration pertama.

---

## Daftar Isi

1. [Understanding](#1-understanding)
2. [Product Assessment](#2-product-assessment)
3. [Core Domain](#3-core-domain)
4. [Architecture Assessment](#4-architecture-assessment)
5. [Security Assessment](#5-security-assessment)
6. [MVP Boundary](#6-mvp-boundary)
7. [Recommended Development Roadmap](#7-recommended-development-roadmap)
8. [Open Questions](#8-open-questions)
9. [Recommended Next Step](#9-recommended-next-step)
10. [Ringkasan Perubahan terhadap Dokumen Asli](#10-ringkasan-perubahan-terhadap-dokumen-asli)

---

# 1. Understanding

Jualin CRM adalah multi-tenant B2B SaaS dengan tesis produk:

> CRM lain fokus mengelola data yang sudah masuk. Jualin fokus pada **jalur masuknya** — capture layer sebagai first-class product — lalu mendistribusikan lead ke sales lapangan lewat mobile.

Tiga lapisan yang sebenarnya dijual:

| Layer | Fungsi | Permukaan |
|---|---|---|
| **Capture Layer** | Lead masuk dari mana saja | API, Embedded Form, Inbound Webhook |
| **Management Layer** | Owner mengatur, menugaskan, memantau | Next.js dashboard |
| **Execution Layer** | Employee mengeksekusi follow-up | Flutter mobile |

Backend Go monolith + PostgreSQL menjadi satu-satunya pemegang business logic dan tenant isolation. Landing page murni marketing.

**Konteks tambahan di luar dokumen asli:** Jualin adalah brand payung; CRM adalah produk pertama, akan menyusul **Jualin HRIS** dan **Jualin Invoice**. Ini bukan detail kosmetik — ini mengubah keputusan arsitektur di level identity, organization, dan billing (dibahas di [bagian 4](#4-architecture-assessment)).

Alur inti yang dipegang sebagai kontrak produk:

```
Capture → Lead → Assignment → Follow-up (Activity/Task) → Customer → Deal
```

---

# 2. Product Assessment

## 2.1 Yang sudah bagus — jangan diubah

| # | Keputusan | Kenapa benar |
|---|---|---|
| 1 | **Multi-tenant sejak awal** | Retrofit tenancy ke schema single-tenant adalah salah satu migrasi termahal yang ada. Dokumen menghindarinya. |
| 2 | **Monolith dulu** | Untuk tim kecil, microservices di tahap ini menambah 5× ops cost untuk 0 business value. |
| 3 | **"Semua source menghasilkan model Lead yang sama"** (§14) | Insight terkuat di seluruh dokumen. Integration layer = adapter tipis, core tetap satu. Ini yang menjaga produk tidak bercabang jadi 3 sistem. |
| 4 | **API key eksplisit, tidak auto-generate, tidak dikirim email, disimpan hash** (§8) | Security hygiene yang benar dan sering dilewatkan produk sejenis. |
| 5 | **Activity ≠ Task** (§17) | Pemisahan "sudah terjadi" vs "harus dilakukan" benar secara domain. Banyak CRM amatir menggabungkannya dan timeline-nya jadi tidak berguna. |
| 6 | **Dashboard ≠ Landing page** | Batas yang jelas, menghindari kebingungan deployment dan auth. |
| 7 | **Audit log dipikirkan sejak awal** | B2B enterprise akan menanyakan ini. Murah kalau dari awal, mahal kalau ditambal. |
| 8 | **§30 Product Principle** | Filter fitur yang tajam. Pertahankan dan pakai sungguhan. |

## 2.2 Yang perlu diperbaiki

### A. Positioning belum tajam — risiko produk terbesar, bukan risiko teknis

Dokumen mendeskripsikan **kapabilitas**, bukan **untuk siapa dan kenapa mereka pindah**.

- Kompetitor lokal: Mekari Qontak, Barantum, Sales1.
- Kompetitor global gratis: **HubSpot Free** (unlimited contacts, forms, gratis selamanya), Zoho Bigin (~$7/user).
- **HubSpot Free sudah memberikan lead capture form + CRM gratis tanpa batas kontak.** Kalau pembeda Jualin hanya "capture + CRM", posisi kalah sejak hari pertama.

Pembeda yang realistis dan bisa dimenangkan:

1. **Mobile-first untuk sales lapangan** — mobile app HubSpot/Zoho adalah afterthought yang berat. Sales lapangan Indonesia (properti, otomotif, furniture, asuransi, klinik) butuh app ringan.
2. **WhatsApp-native** — gap terbesar di dokumen (lihat poin C).
3. **Harga Rupiah, invoice lokal, PPN, support Bahasa** — friction nyata untuk UMKM/SMB berlangganan produk USD.
4. **Onboarding integrasi 5 menit** — capture layer yang benar-benar mudah dipasang.

> **Rekomendasi:** tulis satu kalimat ICP (Ideal Customer Profile) sebelum coding. Contoh:
> *"Perusahaan Indonesia 5–50 orang dengan tim sales lapangan yang lead-nya masuk dari website dan WhatsApp, saat ini dikelola pakai spreadsheet + grup WA."*
>
> Kalimat ini akan memutuskan puluhan keputusan produk berikutnya secara otomatis.

### B. Ambisi §3.2 bertentangan dengan §29 MVP Principle

§3.2 mendaftar **13 area fitur dashboard**. §29 bilang jangan besar-besar. Keduanya di dokumen yang sama.

> **Rekomendasi:** §29 yang menang. Tandai §3.2 secara eksplisit sebagai *target 12 bulan*, bukan MVP.

### C. WhatsApp hampir tidak ada — padahal ini kanal lead #1 di Indonesia

Dokumen hanya menyebut WhatsApp sebagai **tombol** di Flutter (§3.3, §16). Realitas pasar:

- Mayoritas lead SMB Indonesia masuk lewat WhatsApp, bukan form website.
- Bila WhatsApp tidak tertangkap sistem, CRM hanya melihat sebagian kecil kenyataan, dan pipeline report jadi tidak dipercaya.

WhatsApp Business API **tidak** disarankan masuk MVP (mahal, verifikasi Meta lambat, kompleks). Minimal:

| Tahap | Aksi |
|---|---|
| MVP | `wa.me` deeplink + **wajib mencatat Activity otomatis** saat employee menekan tombol WhatsApp/Call |
| Pasca-MVP awal | **WhatsApp click-tracking link** (`jln.id/w/xxx`) dipasang di bio/iklan → klik tercatat sebagai Lead sebelum chat terjadi. Murah, langsung terasa nilainya. |
| Nanti | WhatsApp Cloud API sebagai capture source keempat |

Catat sekarang agar model Lead punya ruang untuk `source = whatsapp`.

### D. Customer/Deal ditaruh Phase 10, padahal ada di core mental model (§26, §33)

Kontradiksi prioritas. `Revenue` ada di mental model inti tapi tidak dibangun sampai fase terakhir. Konsekuensinya: selama 9 fase, produk secara teknis adalah **lead inbox**, bukan CRM. Owner tidak bisa menjawab *"berapa nilai deal yang saya menangkan bulan ini"* — dan itulah pertanyaan yang membuat orang mau membayar.

> **Rekomendasi:** naikkan versi paling sederhana dari Customer + Deal ke lebih awal (setelah assignment berjalan). Bukan pipeline drag-and-drop multi-stage — cukup: lead `won` → buat Customer + Deal dengan `value` dan `closed_at`. Itu 1 tabel dan 1 endpoint, tapi membuka seluruh reporting revenue.

### E. Employee invitation flow tidak ada sama sekali

Phase 6 dan 7 mengasumsikan employee sudah ada dan bisa login, tapi tidak ada satu pun bagian dokumen yang menjelaskan **bagaimana employee mendapat akun**.

> Ini bukan detail kecil — ini **blocker untuk seluruh Flutter app**. Wajib masuk MVP.

### F. Angka free tier berisiko secara ekonomi

`100 leads/month, 2 employees` (§6):

- **Batas employee adalah pembatas yang salah untuk free tier.** Sales team 2 orang sudah cukup untuk banyak UMKM, sehingga mereka tidak akan pernah upgrade.
- Pembatas yang lebih baik untuk mendorong konversi: **jumlah lead/bulan**, **retensi data historis**, dan **fitur** (assignment otomatis, report, integrasi).
- Ambigu: **apakah Owner dihitung sebagai 1 dari 2 employee?** Harus diputuskan sebelum billing logic ditulis.

---

# 3. Core Domain

## 3.1 Koreksi domain terpenting: Employee bukan entity terpisah

Dokumen memperlakukan `User` dan `Employee` sebagai dua hal berbeda:

- §19 mencantumkan keduanya sebagai anak Organization
- §24 punya `internal/user/` dan `internal/employee/`
- §15 memakai `lead.assigned_to = employee_id`

**Ini akan menjadi sumber bug identity yang berkepanjangan.** Pertanyaan yang tidak akan punya jawaban bersih:

- Kalau Manager ikut memegang lead, dia User atau Employee?
- Kalau Employee dipromosikan jadi Manager, record-nya pindah tabel?
- Kalau Owner ikut follow up lead sendiri (sangat umum di UMKM), apakah dia perlu record Employee?

### Model yang benar

```
User ──< Membership >── Organization
              │
              └── role: owner | admin | manager | member
```

| Konsep | Definisi |
|---|---|
| **User** | Identitas login (email, password_hash, MFA). Global, lintas organization, **lintas produk Jualin**. |
| **Membership** | Keanggotaan user pada satu organization + role. **Ini yang menjadi "employee".** |
| **Employee** | Bukan tabel — melainkan *membership dengan role `member`*. |

**Assignment menunjuk ke `membership_id`**, bukan `user_id` dan bukan `employee_id`.

> **Kenapa `membership_id`?** Satu `user_id` bisa berada di dua organization. Kalau lead menunjuk `user_id`, isolasi tenant bergantung pada disiplin query. Kalau menunjuk `membership_id` (yang secara definisi terikat ke satu org), isolasi menjadi **struktural**.

Bila nanti butuh atribut HR (NIP, departemen, atasan, foto), buat `employee_profile` 1:1 ke membership — dan itu justru pintu masuk alami ke **Jualin HRIS**.

> **Catatan penamaan:** role `Employee` di §18 sebaiknya diganti `Member` atau `Sales`, agar tidak tertukar dengan konsep employee HRIS nanti.

## 3.2 Peta entity — dua bounded context

Dipisahkan sejak awal **di dalam satu monolith yang sama**, hanya sebagai batas modul:

```
╔═══════════════════ PLATFORM (dipakai semua produk Jualin) ═══════════════════╗
║                                                                              ║
║  User ──────< Membership >────── Organization                                ║
║   │              │                    │                                      ║
║   │              │                    ├──< Subscription >── Plan             ║
║   │              │                    │        │                             ║
║   │              │                    │        └──< UsageCounter             ║
║   │              │                    ├──< ApiKey                            ║
║   │              │                    ├──< Invitation                        ║
║   │              │                    └──< AuditLog                          ║
║   │              │                                                           ║
║   ├──< Session / RefreshToken                                                ║
║   └──< DeviceToken (push)                                                    ║
╚══════════════════════════════════════════════════════════════════════════════╝

╔═══════════════════════════ CRM (produk Jualin CRM) ══════════════════════════╗
║                                                                              ║
║   Form ────┐                                                                 ║
║   Webhook ─┼──> LeadSource ──> LEAD                                          ║
║   ApiKey ──┘                     │ assigned_to → Membership                  ║
║                                  │                                           ║
║                    ┌─────────────┼─────────────┐                             ║
║                    ▼             ▼             ▼                             ║
║               Activity        Task         Customer                          ║
║               (past)        (future)          │                              ║
║                                               └──< Deal ──> Pipeline/Stage   ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

## 3.3 Tabel relasi entity

| Relasi | Kardinalitas | Catatan penting |
|---|---|---|
| User ↔ Organization | many-to-many **via Membership** | Jangan `user.organization_id`. Satu orang bisa jadi konsultan di 3 org. Schema mendukung banyak, UI MVP boleh asumsikan satu. |
| Membership → Role | many-to-one (enum kolom di MVP) | Kolom `role` saja. Jangan tabel Role + Permission + RolePermission di MVP. |
| Employee | **bukan entity** | = Membership dengan role `member`. Opsional: `employee_profile` 1:1 nanti. |
| Organization → Subscription | one-to-one (aktif) + histori | Simpan sebagai baris ber-status agar ada riwayat plan. Satu org = satu subscription aktif **per produk** → kolom `product`. |
| Subscription → Plan | many-to-one | `Plan` menyimpan definisi limit (JSONB entitlements). **Jangan hard-code limit di Go.** |
| Organization → ApiKey | one-to-many | Key **milik organization**, bukan milik user. Kalau user keluar, integrasi tidak boleh mati. Simpan `created_by_membership_id` untuk audit saja. |
| Organization → Form | one-to-many | Form punya `public_key` (publishable) — **berbeda dari API key**. |
| Organization → Webhook | one-to-many | Inbound: `endpoint_token` + `signing_secret`. Outbound: `target_url` + `signing_secret`. **Dua konsep berbeda, jangan satu tabel.** |
| Form/ApiKey/Webhook → Lead | one-to-many (provenance) | Lead menyimpan `source_type` + `source_id`. **Jangan** FK keras ke 3 tabel — pakai `source_type ENUM` + `source_id UUID` nullable. |
| Lead → Membership | many-to-one (`assigned_to`) | Nullable. **Wajib composite FK ber-org** (lihat [4.3 lapis 2](#lapis-2--composite-foreign-key)). |
| Lead → Activity | one-to-many | Activity **append-only, immutable**. Ini timeline. |
| Lead → Task | one-to-many | Task mutable: `due_at`, `status`, `assignee`. |
| Lead → Customer | **one-to-one saat konversi** | Lead **tidak berubah** jadi Customer. Lihat catatan di bawah. |
| Customer → Deal | one-to-many | Satu customer bisa beli berkali-kali. Ini yang membuat revenue bisa dilaporkan. |
| Deal → Pipeline/Stage | many-to-one | Pasca-MVP. |
| Activity/Task → Membership | many-to-one (`actor`) | Siapa yang melakukan. |
| AuditLog → Membership | many-to-one (`actor`), nullable | Nullable karena aktor bisa API key atau sistem. Simpan `actor_type`. |

### ⚠️ Catatan kritis — Lead vs Customer

Godaan terbesar adalah membuat Customer = Lead dengan `status='customer'`. **Jangan.**

- Satu Customer bisa berasal dari **beberapa** Lead (orang yang sama mengisi form 3× dalam setahun).
- Lead adalah **peristiwa capture**; Customer adalah **relasi berkelanjutan**.
- Menggabungkannya akan merusak metrik konversi secara permanen.

**Keputusan:** Lead tetap ada (histori capture), Customer adalah baris baru, dihubungkan lewat `lead.converted_customer_id`.

## 3.4 Field yang wajib ada sejak migration pertama

Sering telat ditambahkan, dan mahal ditambal.

| Field | Di mana | Kenapa sekarang |
|---|---|---|
| `custom_fields JSONB` | leads, customers | Pelanggan CRM **selalu** minta field custom. Kolomnya murah sekarang, migrasinya mahal nanti. Sediakan kolomnya, **jangan bangun UI-nya** di MVP. |
| `phone_e164` | leads, customers | Normalisasi `08123` → `+628123` **saat ingest**. Tanpa ini dedup dan deeplink WhatsApp rusak, dan memperbaikinya belakangan berarti backfill jutaan baris. |
| `raw_payload JSONB` | leads | Simpan payload asli dari API/form/webhook. Menyelamatkan saat debugging integrasi pelanggan — dan itu akan sering terjadi. |
| `idempotency_key` | leads | Unik per org. Lihat [5.4](#54--idempotency--duplikasi-lead). |
| `deleted_at` | semua tabel bisnis | Soft delete konsisten. |
| `created_at` / `updated_at` UTC | semua | Simpan UTC, render di timezone org. Simpan `organizations.timezone` (default `Asia/Jakarta`). |

### Primary key

Gunakan **UUIDv7** (time-ordered), bukan UUIDv4.

> UUIDv4 acak sebagai PK menyebabkan fragmentasi B-tree index yang signifikan pada tabel yang tumbuh cepat seperti `leads` dan `activities`. UUIDv7 memberi keunikan yang sama tanpa penalti tersebut, dan bisa diurutkan secara natural.

---

# 4. Architecture Assessment

## 4.1 Stack — setuju, dengan tiga catatan

Go + Gin + PostgreSQL + monolith: **tepat**. Tidak ada alasan menyarankan yang lain.

1. **Gin vs net/http** — Go 1.22+ punya routing pattern (`GET /v1/leads/{id}`) di stdlib. Gin masih memberi ergonomi middleware/binding yang berguna. Pilihan ini bukan keputusan yang perlu diperdebatkan.
   **Yang lebih penting: jangan pakai ORM berat.** Rekomendasi `sqlc` (type-safe, generate Go dari SQL) atau `pgx` + query manual. GORM akan menyulitkan penegakan tenant isolation karena query di-generate implisit.

2. **Jangan tambahkan Redis di MVP.** Rate limiting, idempotency, dan job queue semuanya bisa di PostgreSQL pada skala awal. Setiap komponen infrastruktur tambahan adalah beban ops permanen. Tambahkan Redis saat benar-benar butuh horizontal scaling.

3. **Background job: Postgres-backed queue.** River, atau tabel `jobs` sederhana + worker goroutine dengan `SELECT ... FOR UPDATE SKIP LOCKED`. Dibutuhkan untuk: kirim email, push notification, outbound webhook + retry. `SKIP LOCKED` membuat ini benar dan sederhana tanpa broker terpisah.

## 4.2 Struktur modul — revisi dari §24

§24 sudah baik, tapi perlu **batas platform vs produk** karena rencana multi-produk Jualin:

```
jualin-backend/
  cmd/
    api/            # HTTP server
    worker/         # background jobs (proses terpisah, binary sama boleh)
    migrate/

  internal/
    platform/            # ← dipakai semua produk Jualin
      auth/              # login, token, password, verification
      user/
      organization/
      membership/
      invitation/
      subscription/      # plan, entitlement, usage
      apikey/
      audit/
      notification/

    crm/                 # ← spesifik Jualin CRM
      lead/
      form/
      webhook/
      assignment/
      activity/
      task/
      customer/
      deal/

    shared/
      tenant/            # TenantContext, scoped repository
      httpx/             # error envelope, response, validation
      db/                # pool, tx manager, migration
      config/
      logger/
```

### Aturan dependensi

Tegakkan dengan linter (`go-arch-lint` atau `depguard`):

| Aturan | |
|---|---|
| `crm/*` → `platform/*` | ✅ boleh |
| `platform/*` → `crm/*` | ⛔ tidak boleh |
| modul sesama level (`lead` ↔ `customer`) | Lewat **service interface**, bukan langsung ke repository satu sama lain |

> Kalau aturan ini ditegakkan sejak hari pertama, memisahkan HRIS nanti (atau mengekstrak platform jadi service tersendiri) menjadi **refactor mekanis**, bukan penulisan ulang. Kalau tidak ditegakkan, dalam 6 bulan semua akan saling import dan batasnya hilang.

**Ini murni batas paket di dalam satu binary.** Jangan buat service terpisah sekarang.

## 4.3 Multi-tenant isolation

Prioritas #3 pemilik produk, dan bagian di mana kegagalan bersifat fatal — satu kebocoran cross-tenant = kehilangan kepercayaan yang tidak bisa dipulihkan di B2B.

**Strategi: shared database, shared schema, `organization_id` di setiap tabel bisnis.**

> Ini pilihan yang benar untuk skala Jualin. Schema-per-tenant akan meledak saat 1000 tenant (migrasi harus jalan 1000 kali); database-per-tenant terlalu mahal.

Tapi `organization_id` saja **tidak cukup** — itu hanya mengandalkan disiplin developer. Bangun **empat lapis pertahanan**:

### Lapis 1 — Repository yang secara struktural tidak bisa lupa

Tidak boleh ada satu pun jalur query yang menerima org_id sebagai parameter opsional. Setiap repository method **menerima `TenantContext` sebagai parameter pertama** dan meng-inject `organization_id` ke WHERE clause **di dalam**, bukan diserahkan ke caller.

```go
// Konsep, bukan implementasi final
repo.Leads.List(ctx, tenant, filter)   // org_id di-inject di dalam
```

Dan **tidak ada** method seperti `FindByID(id)` tanpa tenant.

> Kalau method itu tidak ada, tidak ada yang bisa memanggilnya secara salah. Ini prinsip *"make illegal states unrepresentable"* diterapkan pada tenancy.

### Lapis 2 — Composite foreign key

**Bagian yang hampir selalu dilewatkan.** Isolasi query melindungi *pembacaan*. Yang tidak dilindungi: **referensi silang tenant**.

**Contoh serangan:**
Owner org A mengirim `PATCH /leads/{lead_A}` dengan body `{"assigned_to": "<membership_id milik org B>"}`.
Query lead-nya tenant-scoped dan lolos. FK ke `memberships(id)` juga valid.
→ Data korup lintas tenant **tanpa satu pun error**.

**Cegah di level database:**

```sql
-- pada memberships
UNIQUE (id, organization_id)

-- pada leads
FOREIGN KEY (assigned_to, organization_id)
  REFERENCES memberships (id, organization_id)
```

Sekarang database **mustahil** menyimpan referensi lintas tenant, apapun bug di kode aplikasi.

> Terapkan pola ini untuk **setiap** FK antar entity bisnis (lead→form, task→lead, deal→customer, dst). Biayanya satu index tambahan per tabel. **Ini investasi keamanan dengan rasio manfaat/biaya tertinggi di seluruh sistem.**

### Lapis 3 — Row Level Security sebagai jaring pengaman

PostgreSQL RLS dengan `SET LOCAL app.current_org_id` di awal setiap transaksi, policy `USING (organization_id = current_setting('app.current_org_id')::uuid)`.

**Rekomendasi: tunda ke pasca-MVP, tapi rancang agar mudah ditambahkan.**

Alasan jujur: RLS berinteraksi rumit dengan connection pooling dan menambah beban debugging yang nyata untuk tim kecil. Lapis 1+2+4 sudah memberi perlindungan sangat besar. Tambahkan RLS saat ada pelanggan enterprise yang menanyakannya, atau saat tim bertambah.

**Yang harus disiapkan sekarang agar RLS bisa masuk nanti tanpa nyeri:**

- Konsisten menamai kolom `organization_id` di **semua** tabel
- **Semua** akses DB lewat satu transaction manager (satu tempat untuk menyisipkan `SET LOCAL` nanti)

### Lapis 4 — Test suite isolasi tenant

**Wajib, non-negotiable.**

Satu file test yang membuat Org A dan Org B beserta datanya, lalu **menembak setiap endpoint** dengan kredensial A terhadap resource ID milik B, dan mengharapkan **404** (bukan 403 — 403 membocorkan keberadaan resource).

Buat ini **generik dan otomatis atas daftar route**, sehingga endpoint baru otomatis ikut teruji. **Jadikan blocking di CI.**

> Ini satu-satunya cara memastikan isolasi tidak mengalami regresi seiring produk tumbuh.

## 4.4 Tenant context & resolusi org

```
Request
  → Auth middleware (JWT | API key | form public key)
  → resolve principal: (user+membership) | (api_key) | (public form)
  → resolve organization_id
  → cek subscription status (active/past_due/suspended)
  → cek entitlement & quota
  → rate limit (per org, per key, per IP)
  → handler
```

`TenantContext` di §20 sudah benar arahnya, perlu diperkaya:

```go
type TenantContext struct {
    OrganizationID uuid.UUID
    PrincipalType  PrincipalType   // user | api_key | public_form | system
    MembershipID   *uuid.UUID      // nil kalau API key
    UserID         *uuid.UUID
    Role           Role
    APIKeyID       *uuid.UUID
    RequestID      string          // untuk korelasi log & audit
}
```

> `PrincipalType` penting: audit log harus bisa membedakan *"Budi mengubah lead"* dari *"integrasi website mengubah lead"*. Tanpa ini, audit trail berbohong.

## 4.5 Domain & API namespace

Perlu diputuskan sekarang karena rencana multi-produk.

| Fungsi | Rekomendasi |
|---|---|
| Marketing | `jualin.id` |
| Dashboard | `app.jualin.id` — **satu shell untuk semua produk Jualin**, CRM di `/crm` |
| API | `api.jualin.id` |
| Embed script / CDN | `cdn.jualin.id` atau `js.jualin.id` — **domain terpisah**, penting untuk isolasi keamanan |
| Namespace API | `/v1/...` untuk platform (`/v1/auth`, `/v1/organizations`); `/v1/crm/...` untuk resource CRM |

> **Jangan** `crm.jualin.id` terpisah — cross-domain auth antar produk nanti akan menyakitkan.
>
> Menempatkan dashboard semua produk di satu origin (`app.jualin.id`) berarti session cookie berlaku lintas produk secara alami — ini yang membuat "satu akun Jualin untuk semua produk" terwujud **tanpa membangun SSO server**.

---

# 5. Security Assessment

Beberapa masalah di dokumen asli bersifat serius. Diurutkan berdasarkan tingkat keparahan.

## 5.1 🔴 KRITIS — Embedded Form tidak boleh memakai API Key

**§10 menyarankan:**

```http
POST /v1/forms/{form_id}/submit
Authorization: Bearer {api_key}
```

Untuk **server-to-server** ini benar. Untuk **embedded form** ini **kerentanan serius**: form berjalan di browser publik, sehingga API key harus ada di client-side. Siapapun bisa membuka DevTools, mengambil key tersebut, dan menggunakannya untuk mengakses **seluruh API organization itu** — membaca semua lead, menghapus data, apapun scope key tersebut.

> Ini bukan risiko teoretis. Ini **kebocoran otomatis pada setiap pemasangan form**.

### Perbaikan — pisahkan dua jalur secara tegas

| | Server-to-server | Embedded form (browser) |
|---|---|---|
| Endpoint | `POST /v1/crm/leads` | `POST /v1/crm/forms/{public_key}/submit` |
| Kredensial | `Authorization: Bearer jln_live_...` (**secret**) | `public_key` (**publishable**, aman terekspos) |
| Sifat | Rahasia, hash di DB, revocable | Publik, hanya identifier |
| Kemampuan | Full API sesuai scope | **Hanya submit ke form itu.** Tidak bisa baca apapun. |
| CORS | Tidak relevan | Dibatasi ke domain allowlist milik form |
| Proteksi | Rate limit per key | Domain allowlist + rate limit per IP + honeypot + CAPTCHA |

> **Analogi:** Stripe punya `pk_live_` (publishable, aman di browser) dan `sk_live_` (secret, hanya server). Jualin butuh pemisahan yang sama persis.

### Proteksi wajib untuk endpoint form publik

1. **Origin allowlist per form** — owner mendaftarkan `www.tokoku.com`; server memverifikasi header `Origin`.
   *Catatan jujur:* `Origin` bisa dipalsukan oleh non-browser, jadi ini menghalangi penyalahgunaan biasa, bukan penyerang tertarget — karena itu ia dipasangkan dengan lapisan lain, bukan berdiri sendiri.
2. **CAPTCHA** — Cloudflare Turnstile (gratis, tanpa puzzle, UX bagus). Wajib untuk free tier; opsional untuk plan berbayar.
3. **Honeypot field** — field tersembunyi; kalau terisi, sunyi-sunyi buang (**jangan** kembalikan error, agar bot tidak belajar).
4. **Rate limit per IP + per form** — mis. 5 submit/menit/IP.
5. **Time-trap** — tolak submit < 2 detik setelah form render.
6. **Batas ukuran payload** — mis. 32KB.

> Tanpa ini, form publik + free tier = **magnet spam**, dan biaya penyimpanan serta reputasi email Anda yang menanggung.

## 5.2 🔴 KRITIS — Skema penyimpanan & lookup API Key

§8 benar menyatakan "disimpan sebagai hash". Tapi ada jebakan implementasi:

> Kalau hanya menyimpan bcrypt hash, **lookup tidak mungkin dilakukan** — bcrypt di-salt, jadi setiap request harus membandingkan terhadap seluruh baris. Pada 10.000 key, itu 10.000 operasi bcrypt per request. Sistem berhenti berfungsi.

### Format key yang benar

```
jln_live_<key_id:12char>_<secret:32char>
        │       │                │
        │       │                └─ entropi rahasia (crypto/rand)
        │       └─ pencari, disimpan plaintext & di-index
        └─ environment (live/test)
```

### Cara verifikasi

1. Parse `key_id` dari kredensial
2. `SELECT ... WHERE key_id = $1 AND revoked_at IS NULL` (index hit, O(1))
3. Bandingkan `SHA-256(secret)` dengan `secret_hash` menggunakan **`crypto/subtle.ConstantTimeCompare`**

### Kenapa SHA-256, bukan bcrypt/argon2, untuk API key?

bcrypt/argon2 didesain lambat untuk melawan brute-force pada *password* — yang entropinya rendah karena dipilih manusia. API key yang di-generate `crypto/rand` punya 256-bit entropi; brute-force mustahil terlepas dari kecepatan hash.

> Memakai bcrypt di sini hanya menambah ~100ms ke **setiap request API panas** tanpa manfaat keamanan nyata. SHA-256 tepat untuk kasus ini.
>
> **Untuk password user, tetap gunakan argon2id atau bcrypt.**

### Kolom tambahan

| Kolom | Fungsi |
|---|---|
| `key_prefix` | 8 char pertama, untuk ditampilkan `jln_live_a3f9…` di dashboard |
| `last_used_at` | Update **async/throttled**, jangan setiap request — itu akan menjadi write hotspot |
| `expires_at` | Kedaluwarsa opsional |
| `scopes` | `leads:write`, `leads:read` — ada sejak awal (lihat 5.6) |

## 5.3 🟠 Multi-tenant IDOR

Sudah dibahas di [4.3](#43-multi-tenant-isolation). Tambahan:

> **Selalu kembalikan 404, jangan 403**, untuk resource milik tenant lain. 403 mengonfirmasi bahwa resource dengan ID tersebut ada — itu kebocoran informasi.

## 5.4 🟠 Idempotency & duplikasi lead

**Tidak ada sama sekali di dokumen asli.** Ini bug correctness yang **pasti terjadi**, bukan kemungkinan:

- Client integrasi retry karena timeout → lead ganda
- Visitor menekan tombol submit dua kali → lead ganda
- Webhook pihak ketiga mengirim ulang (Shopify, Meta, dll. semua melakukan retry) → lead ganda

> Lead duplikat merusak kepercayaan pada CRM lebih cepat daripada hampir semua bug lain — sales menelepon orang yang sama dua kali, dan pelanggan Anda melihatnya.

### Dua mekanisme berbeda, keduanya perlu

**1. Idempotency teknis**
Header `Idempotency-Key` pada `POST /v1/crm/leads`. Simpan `UNIQUE (organization_id, idempotency_key)`.
Request ulang dengan key sama → kembalikan **response asli** (200 + lead yang sama), bukan error. Simpan hasil response, bukan hanya flag. Retensi 24–48 jam.

**2. Dedup bisnis**
Lead baru dengan `phone_e164` atau `email` yang sama dalam window (mis. 30 hari) di org yang sama → **jangan tolak**, tandai `possible_duplicate_of` dan tampilkan di UI untuk digabung manual.

> Auto-merge itu berbahaya — dua orang bisa berbagi nomor kantor. Untuk MVP cukup deteksi + tampilkan; merge manual menyusul.

## 5.5 🟠 Webhook security

### Inbound

- Endpoint `/v1/crm/webhooks/in/{endpoint_token}` — token panjang & acak di URL
- Bila provider mendukung signature (Shopify HMAC, Meta), verifikasi dengan `ConstantTimeCompare`
- **Wajib** proteksi replay (timestamp + toleransi 5 menit)

### Outbound — lebih berbahaya bagi Anda

| Risiko | Mitigasi |
|---|---|
| **SSRF** | Pelanggan mendaftarkan `http://169.254.169.254/latest/meta-data/` (endpoint metadata cloud) atau `http://localhost:5432`. **Server Anda** yang melakukan request tersebut, **dari dalam jaringan Anda**. Wajib blokir private IP range (RFC1918, loopback, link-local, IPv6 equivalents) **setelah resolusi DNS**, dan tangani DNS rebinding (resolve → validasi → connect ke IP yang sudah divalidasi). Hanya izinkan HTTPS port 443. |
| **Forgery** | HMAC-SHA256 atas `timestamp + body`, header `X-Jualin-Signature`, sertakan timestamp agar penerima bisa cegah replay |
| **Resource exhaustion** | Retry dengan exponential backoff + jitter, batas percobaan, **circuit breaker** — endpoint pelanggan yang mati tidak boleh menghabiskan worker Anda |
| **Hang** | Timeout ketat (5 detik), **tanpa mengikuti redirect** |

## 5.6 🟠 Autentikasi — tiga jalur

### Dashboard (Owner/Admin/Manager) — Next.js

- **Access token JWT, TTL pendek (15 menit)** + **refresh token opaque di database** (bisa direvoke — JWT murni tidak bisa)
- Keduanya di cookie **`HttpOnly; Secure; SameSite=Lax; Domain=.jualin.id`**
- **Jangan simpan token di localStorage.** Satu kerentanan XSS di dashboard = semua token pelanggan tercuri. HttpOnly cookie tidak bisa dibaca JavaScript.
- Karena memakai cookie: **wajib proteksi CSRF** — double-submit token atau `SameSite=Strict` pada endpoint mutasi. *Ini konsekuensi yang sering dilupakan saat memilih cookie.*
- `Domain=.jualin.id` berarti Jualin HRIS nanti di origin yang sama otomatis ikut ter-autentikasi

### Mobile (Flutter)

- Tidak ada cookie yang nyaman → **access JWT + refresh token opaque** di `flutter_secure_storage` (Keychain iOS / Keystore Android)
- **Refresh token rotation dengan reuse detection**: setiap refresh menerbitkan token baru dan membatalkan yang lama. Bila token lama dipakai lagi → asumsikan pencurian, **revoke seluruh family token**, paksa login ulang
- **Device binding** — kaitkan refresh token ke `device_id`, agar push notification bisa dialamatkan dan owner bisa melihat "perangkat aktif" serta logout jarak jauh (relevan saat sales resign — kasus nyata yang akan ditemui)
- Access token TTL boleh lebih panjang (1 jam) karena mobile lebih sering offline

### External API

- API key seperti di [5.2](#52--kritis--skema-penyimpanan--lookup-api-key)
- **Scoped** (`leads:write`, `leads:read`) sejak awal — meski MVP hanya memakai satu scope, kolomnya harus ada, karena menambahkan scope ke key yang sudah beredar adalah **breaking change**
- **Tanpa akses ke endpoint dashboard.** API key tidak boleh bisa mengelola employee, mengubah subscription, atau membuat API key lain. Batas ini di **middleware**, bukan di masing-masing handler.

### Ringkasan matriks

| Principal | Kredensial | Penyimpanan | Revocation | Lifetime |
|---|---|---|---|---|
| Dashboard user | JWT + refresh opaque | HttpOnly cookie | ✅ via DB | 15m / 30d |
| Mobile employee | JWT + refresh opaque | Secure storage | ✅ + device | 1h / 90d |
| External system | API key | Milik pelanggan | ✅ instan | Sampai direvoke |
| Public form | Public key | Terekspos publik | ✅ | Sampai form dihapus |

## 5.7 🟡 Risiko lain

| Risiko | Mitigasi |
|---|---|
| **Password** | argon2id (atau bcrypt cost 12). Cek terhadap daftar password bocor (HIBP k-anonymity API — gratis). Minimum 12 karakter, jangan paksa aturan komposisi rumit (NIST SP 800-63B). |
| **Enumerasi email** | Response registrasi & forgot-password **identik** baik email ada maupun tidak. |
| **Brute-force login** | Rate limit per IP **dan** per akun. Backoff progresif. Jangan lock permanen — itu jadi vektor DoS terhadap pengguna sah. |
| **Verifikasi email** | Token sekali pakai, kedaluwarsa 24 jam, hash di DB. Tanpa verifikasi: **tidak boleh** buat API key, **tidak boleh** undang employee. |
| **Token undangan** | Kedaluwarsa 7 hari, sekali pakai, terikat ke email spesifik. |
| **Escalation privilege** | Member tidak boleh mengubah role sendiri. Owner terakhir tidak boleh menghapus/menurunkan dirinya sendiri (org tanpa owner = tenant yatim). |
| **PII & UU PDP** | Data pelanggan Anda berisi PII pihak ketiga — Anda adalah *processor*. Perlu: export data, penghapusan permanen, retention policy, enkripsi at-rest (disk-level cukup untuk mulai). |
| **Penghapusan tenant** | Soft delete → grace period 30 hari → hard delete berjadwal. Jangan `CASCADE DELETE` langsung. |
| **Log** | Jangan pernah log: password, API key mentah, token, body payload lead penuh (PII). Redaksi terstruktur di logger. |
| **Embed script** | Sajikan dari domain terpisah (`cdn.jualin.id`) dengan CSP ketat. Kalau memakai iframe, kirim `X-Frame-Options`/`frame-ancestors` sesuai allowlist form. |
| **Upload file** | Bila ada attachment: validasi content-type sebenarnya (magic bytes, bukan ekstensi), sajikan dari domain terpisah, jangan pernah eksekusi. |

---

# 6. MVP Boundary

## 6.1 Yang over-engineered untuk tahap ini

| Item di dokumen | Penilaian | Alasan |
|---|---|---|
| **Inbound webhook (§13)** sebagai capture method ketiga | ⛔ Tunda | 90% tumpang tindih dengan Direct API. Bedanya hanya field mapping per-provider — itu pekerjaan besar tersendiri. Dua capture method sudah cukup membuktikan tesis produk. |
| **Outbound webhook + retry + delivery log (Phase 9)** | ⛔ Tunda | Butuh queue, backoff, circuit breaker, proteksi SSRF, UI delivery log. Fitur 2–3 minggu untuk sesuatu yang **belum ada pelanggan yang memintanya**. |
| **Form builder dinamis** (owner memilih field bebas) | ⚠️ Kecilkan | Dynamic field schema + validation + rendering + storage jauh lebih besar dari kelihatannya. **MVP: field tetap** (name, email, phone, company, message) dengan toggle required + ubah label. Builder penuh menyusul. |
| **Tabel Role + Permission + RolePermission** | ⛔ Tunda | Enum `role` di membership + matriks permission hard-coded di Go sudah menangani 4 role. RBAC dinamis diperlukan saat pelanggan minta custom role — itu sinyal enterprise, dan belum ada. |
| **API key environment (live/test)** | ⚠️ Sebagian | Simpan **format** `jln_live_`/`jln_test_` sejak awal (murah), tapi jangan bangun sandbox environment terpisah. |
| **Pipeline dengan stage yang bisa dikustomisasi** | ⛔ Tunda | Lead status enum sudah cukup untuk MVP. |
| **Contact terpisah dari Customer** | ⛔ Tunda | Relevan untuk B2B kompleks (satu perusahaan, banyak PIC). Untuk SMB Indonesia, Customer sudah cukup. |
| **Payment gateway (Phase 2)** | ⛔ Tunda | Lihat 6.2 — koreksi roadmap terpenting. |
| **Automation / assignment rule engine** | ⛔ Tunda | Manual + round-robin sederhana. Rule engine adalah lubang kelinci tanpa dasar. |
| **Advanced reports & analytics** | ⛔ Tunda | 5 angka di dashboard sudah menjawab 80% kebutuhan. |
| **Developer portal** | ⛔ Tunda | Satu halaman dokumentasi statis sudah cukup. |
| **Microservices** | ⛔ Tidak | Setuju penuh dengan dokumen. |

## 6.2 Koreksi penting: pisahkan Entitlement dari Billing

Phase 2 dokumen menaruh "Subscription + Billing" sangat awal. Ini keliru — **tapi hanya separuhnya**.

| Komponen | Kapan | Alasan |
|---|---|---|
| **Entitlement / quota** (limit plan, usage counter, penegakan limit) | ✅ **Awal** | Menyentuh schema dan setiap jalur ingest. Menambahkannya belakangan berarti membongkar banyak handler. |
| **Payment gateway** (Midtrans/Xendit), invoice, proration, dunning | ⛔ **Tunda** | Pekerjaan 2–3 minggu. Melakukannya sebelum ada permintaan berbayar adalah waktu yang hangus. |

> Untuk 10 pelanggan pertama, **upgrade manual** (Anda ubah plan lewat admin internal) sepenuhnya wajar — dan justru memberi Anda percakapan langsung dengan pelanggan.

## 6.3 MVP — yang wajib dibuat

> **Definisi MVP:** satu organization bisa memasang form di website mereka, lead masuk, ter-assign ke sales, sales menindaklanjuti dari HP, dan owner melihat hasilnya.
>
> **Semua hal di luar kalimat itu bukan MVP.**

### Platform

- Register (user + organization + membership owner + subscription free)
- Verifikasi email
- Login / logout / refresh / forgot password
- **Undang employee** (email → set password → membership)
- Tenant context + RBAC 4 role
- Plan + entitlement + usage counter + penegakan quota
- API key: create, list, revoke (scoped)
- Audit log untuk aksi sensitif (auth, API key, role, penghapusan)

### CRM

- Lead: create (API + form), list dengan filter/pagination, detail, update status, catatan
- **Idempotency + deteksi duplikat**
- Assignment manual + round-robin
- Activity (append-only timeline, auto-log pada perubahan status/assignment)
- Task dengan due date
- Konversi Lead → Customer + **Deal sederhana** (nilai + tanggal closing)
- Embedded form: field tetap, domain allowlist, Turnstile, snippet embed
- Rate limiting

### Dashboard (Next.js)

- Auth + onboarding
- **Lead list & detail** — layar yang paling sering dibuka, investasikan di sini
- Assignment
- Employee management + undangan
- Form management + snippet
- API key management
- Dashboard metrik: lead masuk, terdistribusi, tingkat konversi, nilai deal
- Pengaturan organization

### Mobile (Flutter)

- Login (+ biometric unlock)
- My Leads / My Tasks
- Lead detail + tombol Call/WhatsApp (**otomatis mencatat Activity**)
- Tambah catatan, ubah status, jadwalkan follow-up
- Push notification (FCM)
- **Baca offline** — cache lead yang di-assign. Sales lapangan sering di area sinyal buruk; ini pembeda nyata, bukan nice-to-have.

### Infra

- Migration, structured logging, error tracking (Sentry), health check, backup harian + **uji restore**, CI

## 6.4 Ditunda — urutan prioritas

1. Payment gateway & self-serve upgrade
2. Inbound webhook + integrasi pihak ketiga
3. Outbound webhook + event system
4. WhatsApp click-tracking → WhatsApp Cloud API
5. Form builder dinamis + custom field UI
6. Pipeline dengan stage kustom, drag & drop
7. Import/export CSV — *naikkan prioritas jika calon pelanggan datang dari spreadsheet, dan mereka biasanya begitu*
8. Contact terpisah, multi-currency
9. RBAC dinamis, SSO/SAML
10. Automation rules
11. Advanced reporting

---

# 7. Recommended Development Roadmap

> **Prinsip: vertical slice, bukan horizontal layer.**
>
> Roadmap dokumen asli membangun per-lapisan (semua backend dulu, Flutter di Phase 7). Risikonya: Anda baru tahu apakah produknya bekerja di bulan ke-5. Roadmap di bawah menghasilkan sesuatu yang bisa didemokan jauh lebih awal.

Estimasi mengasumsikan **satu developer full-time**. Skalakan sesuai realita tim.

## Milestone

### M0 — Foundation *(1 minggu)*

Go + Gin + pgx/sqlc, Docker Compose, config, structured logging, error envelope, migration tool, health check, CI (lint + test + build).

**Selesai bila:** `docker compose up` → server jalan, migration jalan, CI hijau.

---

### M1 — Platform Identity *(2 minggu)*

User, Organization, Membership, register, verifikasi email, login/refresh/logout, forgot password, tenant context, RBAC, audit log, **test suite isolasi tenant**.

**Selesai bila:** dua org bisa dibuat, dan test isolasi membuktikan tidak ada kebocoran silang.

> Undangan employee sengaja diletakkan di sini, bukan nanti — ia memblokir seluruh alur mobile.

---

### M2 — Lead Core + Dashboard Slice Pertama *(2 minggu)*

Model Lead, CRUD internal, filter/pagination, status lifecycle, Activity auto-log.
Next.js: auth, layout, lead list, lead detail.

**Selesai bila:** owner bisa login, membuat lead manual, dan melihatnya di dashboard.
🎯 **Ini demo pertama Anda.**

---

### M3 — Capture Layer *(2 minggu)*

API key (format, hash, lookup, revoke, scope). `POST /v1/crm/leads` publik. Idempotency. Deteksi duplikat. Normalisasi telepon. Rate limiting.
Dashboard: API key management + halaman dokumentasi.

**Selesai bila:** curl dari luar → lead muncul di dashboard.
🎯 **Ini tesis produk Anda terbukti.**

---

### M4 — Employee & Assignment *(1,5 minggu)*

Undangan employee, assignment manual + round-robin (dengan composite FK dari 4.3), Task, Activity manual, notifikasi in-app.

**Selesai bila:** owner mengundang sales, meng-assign lead, sales melihatnya via API.

---

### M5 — Flutter Employee App *(3 minggu)*

Login + secure storage + refresh rotation, My Leads, My Tasks, lead detail, Call/WhatsApp (dengan auto-Activity), ubah status, catatan, FCM, cache offline.

**Selesai bila:** siklus penuh berjalan di HP nyata.
🎯 **Ini titik di mana produk menjadi nyata.**

---

### M6 — Embedded Form *(2 minggu)*

Model Form + public key, endpoint submit publik, domain allowlist + CORS, Turnstile, honeypot, embed script (`cdn.jualin.id`), dashboard form management + snippet.

**Selesai bila:** paste snippet ke HTML statis → submit → lead masuk, dan submit dari domain tidak terdaftar ditolak.

---

### M7 — Konversi & Revenue *(1 minggu)*

Customer, Deal sederhana, konversi lead, metrik dashboard (lead masuk, konversi, nilai deal, performa per sales).

**Selesai bila:** owner bisa menjawab *"berapa yang saya hasilkan bulan ini"*.
🎯 **Ini yang membuat produk layak dibayar.**

---

### M8 — Quota & Plan Enforcement *(1 minggu)*

Plan, entitlement JSONB, usage counter (UPSERT atomik + reset periodik), penegakan limit, halaman usage, peringatan mendekati limit, **admin internal untuk mengubah plan secara manual**.

**Selesai bila:** free tier benar-benar terbatas dan Anda bisa upgrade pelanggan secara manual.

---

### 🚦 GATE — Berhenti dan cari pelanggan

Setelah M8 Anda punya produk yang lengkap secara siklus.

> **Dapatkan 5–10 design partner yang memakainya sungguhan sebelum menulis baris kode berikutnya.**
>
> Apa yang mereka minta akan berbeda dari tebakan Anda, dan itu yang menentukan M9+.

---

### M9+ — berdasarkan permintaan nyata

Payment gateway → import CSV → WhatsApp → inbound webhook → outbound webhook → pipeline → automation

---

### Ringkasan waktu

| | |
|---|---|
| M0–M8, satu developer full-time | **≈ 15–16 minggu (±4 bulan)** |
| Dengan realitas (bug, revisi desain, hidup) | **rencanakan 5–6 bulan** |
| Bila dikerjakan sambil bekerja penuh waktu | **kalikan dua** |

## 7.1 Roadmap Next.js Dashboard

| Tahap | Isi |
|---|---|
| **D0** | Next.js App Router, TypeScript, Tailwind + shadcn/ui, TanStack Query, API client dengan auto-refresh, error boundary, design token |
| **D1** | Auth (login/register/verify/forgot), route protection via middleware, onboarding wizard |
| **D2** | **Lead list & detail** — layar terpenting. Filter, pencarian, sort, pagination, bulk action, tampilan timeline |
| **D3** | API key + halaman dokumentasi integrasi (copy-paste snippet — kualitas halaman ini menentukan aktivasi) |
| **D4** | Employee management, undangan, assignment |
| **D5** | Form management + preview + snippet embed |
| **D6** | Dashboard metrik, settings |
| **D7** | Usage & subscription |

**Catatan implementasi:**

- **Server Components untuk halaman, Client Components untuk interaksi.** Jangan memaksakan RSC ke tabel interaktif — TanStack Query di client lebih tepat di sana.
- **Jangan proxy semua request lewat Next.js API routes** kecuali dibutuhkan untuk cookie handling. Panggil `api.jualin.id` langsung; satu hop lebih sedikit, satu tempat lebih sedikit untuk bug auth.
- **Optimistic update** untuk perubahan status lead — terasa jauh lebih cepat, murah dilakukan.
- **Empty state adalah fitur.** Dashboard CRM baru itu kosong; halaman kosong yang mengarahkan ("Pasang form Anda →") adalah bagian dari onboarding, bukan dekorasi.

## 7.2 Roadmap Flutter App

| Tahap | Isi |
|---|---|
| **F0** | Setup, flavor (dev/prod), state management (Riverpod), routing (go_router), Dio + interceptor refresh, secure storage |
| **F1** | Login, session, biometric unlock, logout |
| **F2** | My Leads (list + filter), lead detail |
| **F3** | Aksi: Call, WhatsApp deeplink, ubah status, tambah catatan — **semuanya menghasilkan Activity otomatis** |
| **F4** | My Tasks, jadwalkan follow-up, reminder lokal |
| **F5** | FCM push + deeplink ke lead + badge |
| **F6** | Cache offline (Drift/Isar) + antrian aksi offline |
| **F7** | Profil, pengaturan notifikasi, polish |

**Catatan implementasi:**

- **Offline-first sejak awal, jangan ditambal belakangan.** Sales lapangan di basement mall atau area pinggiran akan kehilangan sinyal. Menambahkan sinkronisasi offline ke aplikasi yang dibangun online-only adalah **penulisan ulang**, bukan penambahan. Bahkan cache baca sederhana sudah membuat perbedaan besar.
- **Antrian aksi offline** perlu id yang di-generate client (UUIDv7) + idempotency key, agar sinkronisasi ulang tidak menduplikasi Activity.
- **Ukuran app dan waktu buka** penting — pengguna memakai Android kelas menengah, bukan flagship.
- **Auto-log Activity pada tombol Call/WhatsApp** adalah fitur kecil dengan dampak besar: sales tidak akan pernah mengisi log secara manual, jadi sistem harus melakukannya untuk mereka. Tanpa ini, timeline akan kosong dan seluruh nilai reporting hilang.

---

# 8. Open Questions

Diurutkan berdasarkan seberapa besar dampaknya jika terjawab salah **setelah** coding dimulai.

## 🔴 Harus dijawab sebelum baris pertama migration

| # | Pertanyaan | Rekomendasi | Jawaban |
|---|---|---|---|
| 1 | **Employee = Membership, atau entity terpisah?** | Membership ([3.1](#31-koreksi-domain-terpenting-employee-bukan-entity-terpisah)) | _(belum diisi)_ |
| 2 | **Bolehkah satu user berada di banyak organization?** | Ya di schema, tidak di UI MVP. Membalik keputusan ini nanti = migrasi besar. | _(belum diisi)_ |
| 3 | **Lead → Customer: mutasi baris atau baris baru?** | Baris baru + link ([3.3](#33-tabel-relasi-entity)) | _(belum diisi)_ |
| 4 | **Apa status lifecycle Lead final?** | `new → assigned → contacted → qualified → won \| lost \| unqualified`, dengan `lost_reason` (enum + free text). Transisi divalidasi di service layer. Tambahkan `is_open` terkomputasi. | _(belum diisi)_ |
| 5 | **Apakah ini "produk pertama" atau "aplikasi tunggal"?** | Produk pertama → terapkan batas platform/crm sekarang ([4.2](#42-struktur-modul--revisi-dari-24)). Gratis sekarang, mahal nanti. | _(belum diisi)_ |
| 6 | **Satu subscription per org, atau per org per produk?** | Per org per produk (`subscriptions.product = 'crm'`). Menambahkan kolom sekarang gratis. | _(belum diisi)_ |
| 7 | **UUIDv7 atau ULID untuk PK?** | UUIDv7 — native, time-ordered, mudah di-generate dari Go | _(belum diisi)_ |

## 🟠 Harus dijawab sebelum M3 (capture layer)

| # | Pertanyaan | Rekomendasi | Jawaban |
|---|---|---|---|
| 8 | **Apa yang terjadi saat kuota lead habis?** | **Jangan buang lead pelanggan** — kerusakan bisnis yang tidak bisa dibatalkan, dan mereka akan menyalahkan Anda. Usulan: terima sampai 2× limit dengan penandaan `over_quota`, kunci fitur dashboard, tampilkan peringatan mendesak. Tolak keras hanya di luar batas itu. **Ini keputusan bisnis.** | _(belum diisi)_ |
| 9 | **Apakah owner dihitung dalam kuota employee?** | Tidak. "2 employees" harus berarti 2 sales, bukan 1 sales + owner. Lebih jujur, lebih mudah dijelaskan. | _(belum diisi)_ |
| 10 | **Quota dihitung ulang: kalender bulan atau siklus billing?** | Siklus billing (anchor di `subscription.current_period_start`). Kalender bulan lebih sederhana tapi menimbulkan sengketa saat upgrade di tengah bulan. | _(belum diisi)_ |
| 11 | **Strategi rate limit final?** | Token bucket per (org, key) + per IP untuk endpoint publik. In-memory di MVP dengan interface yang bisa diganti Redis. Kembalikan header `X-RateLimit-*` dan `Retry-After` **sejak awal** — menambahkannya nanti membingungkan integrator yang sudah ada. | _(belum diisi)_ |
| 12 | **Aturan dedup lead: apa dan berapa lama?** | `phone_e164` ATAU `email` sama, dalam 30 hari, di org yang sama → tandai, jangan tolak, jangan auto-merge. | _(belum diisi)_ |
| 13 | **CAPTCHA di semua plan?** | Wajib di free, opsional di berbayar. Turnstile (gratis). | _(belum diisi)_ |

## 🟡 Harus dijawab sebelum M5–M8

| # | Pertanyaan | Catatan |
|---|---|---|
| 14 | **Payment gateway** | Midtrans (ekosistem lokal luas) vs Xendit (DX lebih baik, dokumentasi lebih rapi). Rekomendasi: Xendit — tapi konfirmasi metode pembayaran yang dibutuhkan target pasar (VA? QRIS? kartu?). |
| 15 | **Email provider** | Resend (DX terbaik) / Postmark (deliverability transaksional terbaik) / SES (termurah, setup paling ribet). Rekomendasi: Resend. **Siapkan SPF/DKIM/DMARC sejak hari pertama** — email verifikasi yang masuk spam akan membunuh conversion funnel tanpa Anda sadari. |
| 16 | **Hosting** | VPS + Docker (murah, kontrol penuh, ops manual) vs Fly.io/Railway (cepat, mahal saat tumbuh) vs GCP/AWS. Rekomendasi: VPS + Docker Compose di awal, **managed PostgreSQL** — jangan kelola database produksi sendiri. |
| 17 | **Embedded form: iframe atau inline script?** | iframe = isolasi keamanan lebih baik, styling terbatas. Inline = fleksibel, permukaan risiko lebih besar. Rekomendasi: **iframe** untuk MVP. |
| 18 | **Retensi data** | Berapa lama lead disimpan di free tier? Pembatas yang bagus untuk mendorong upgrade sekaligus mengurangi biaya penyimpanan. |
| 19 | **Bahasa UI** | Indonesia, Inggris, atau dwibahasa? Memengaruhi apakah i18n dibutuhkan sejak awal — retrofit i18n itu menyakitkan. |
| 20 | **Nama pengganti "Employee" di UI** | "Sales", "Anggota Tim", atau "Karyawan"? Memengaruhi copy di seluruh produk. |

## 🟢 Bisa menyusul

Observabilitas (mulai dari structured log + Sentry), strategi backup detail, disaster recovery, kebijakan penghapusan tenant, versioning API v2, developer portal, sertifikasi keamanan.

---

# 9. Recommended Next Step

> **Jangan langsung ke PRD lengkap.** Masih ada 7 pertanyaan 🔴 yang jika salah akan memaksa penulisan ulang schema.

| Langkah | Aksi | Estimasi |
|---|---|---|
| **1** | **Jawab 7 pertanyaan 🔴.** Cukup satu kalimat per pertanyaan. Ini memblokir semuanya. | Hari ini/besok |
| **2** | **Tulis satu kalimat ICP.** *"Jualin CRM untuk **[siapa]** yang **[masalahnya]** dan saat ini **[memakai apa]**."* Kalimat ini menyelesaikan puluhan keputusan produk secara otomatis, dan mencegah scope creep lebih efektif daripada dokumen manapun. | 30 menit |
| **3** | **Domain Model & Schema Specification.** Bukan PRD dulu — **schema dulu**, karena schema yang salah adalah kesalahan termahal di seluruh proyek. Isi: ERD final, DDL setiap tabel + index + composite FK ber-tenant, enum & state machine Lead, aturan tenant isolation, format & lifecycle API key. | Dokumen berikutnya |
| **4** | **API Contract Specification.** OpenAPI 3.1 untuk M1–M3. Ditulis **sebelum** implementasi — Flutter dan Next.js bisa mulai paralel begitu kontrak beku, dan API publik adalah permukaan yang paling mahal untuk diubah setelah ada integrator. | |
| **5** | **PRD per milestone** — ringkas per M1, M2, …, bukan satu PRD raksasa yang usang sebelum selesai dibaca. | |
| **6** | **M0 Foundation** — baru coding. | |

## Tiga hal yang paling mengkhawatirkan

### 1. Scope

Empat permukaan (landing, dashboard, mobile, API platform) + billing + multi-tenancy adalah beban yang berat untuk tim kecil. Roadmap di [bagian 7](#7-recommended-development-roadmap) sudah disusun untuk memberi demo yang bisa dilihat di M2 dan produk nyata di M5 — tapi **disiplin menolak fitur akan lebih menentukan hasilnya daripada pilihan teknis manapun**.

### 2. Membangun 5 bulan tanpa pengguna

GATE setelah M8 itu serius. Bila memungkinkan, cari 2–3 calon pelanggan **sekarang**, sebelum M0, dan bangun bersama mereka.

> Kegagalan paling umum untuk produk seperti ini bukan arsitektur yang buruk — melainkan **arsitektur yang bagus untuk produk yang tidak dibutuhkan siapa pun**.

### 3. Kebocoran cross-tenant

Satu insiden menghapus kredibilitas B2B secara permanen. **Composite FK** ([4.3 lapis 2](#lapis-2--composite-foreign-key)) dan **test suite isolasi** ([4.3 lapis 4](#lapis-4--test-suite-isolasi-tenant)) adalah dua hal yang paling ditekankan di seluruh analisis ini. Keduanya murah bila dilakukan di M1, dan hampir mustahil di-retrofit dengan benar.

---

# 10. Ringkasan Perubahan terhadap Dokumen Asli

Tabel referensi cepat untuk melihat di mana analisis ini menyimpang dari `crm_saas_brainstorming.md`.

| Area | Dokumen asli | Rekomendasi analisis | Alasan |
|---|---|---|---|
| **Employee** | Entity terpisah dari User (§19, §24) | Membership dengan role `member` | Menghindari identity ambiguity permanen |
| **Assignment target** | `lead.assigned_to = employee_id` (§15) | `assigned_to = membership_id` + composite FK | Isolasi tenant menjadi struktural, bukan bergantung disiplin query |
| **Embedded form auth** | API key di URL form submit (§10) | `public_key` publishable + domain allowlist + CAPTCHA | 🔴 API key di browser = kebocoran total |
| **API key storage** | "disimpan sebagai hash" (§8) | `key_id` (indexed) + SHA-256 `secret_hash` + constant-time compare | Lookup mustahil dengan bcrypt murni |
| **Idempotency** | Tidak ada | `Idempotency-Key` + dedup bisnis | Lead duplikat pasti terjadi |
| **Employee invitation** | Tidak ada | Wajib di M1 | Blocker untuk seluruh Flutter app |
| **Customer/Deal** | Phase 10 (terakhir) | M7, versi sederhana | Tanpanya produk hanya "lead inbox", bukan CRM |
| **Billing** | Phase 2 (awal) | Entitlement awal, payment gateway ditunda | Payment sebelum ada pembeli = waktu hangus |
| **Inbound webhook** | Capture method MVP (§13) | Ditunda pasca-MVP | 90% tumpang tindih dengan Direct API |
| **Form builder** | Owner pilih field bebas (§12) | Field tetap + toggle required | Dynamic schema jauh lebih besar dari kelihatannya |
| **Flutter app** | Phase 7 (akhir) | M5 (tengah) | Nilai produk baru terasa saat mobile jalan |
| **Roadmap** | Horizontal (per layer) | Vertical slice | Demo di M2, bukan bulan ke-5 |
| **Struktur modul** | Flat `internal/*` (§24) | `platform/` vs `crm/` + aturan dependensi | Menyiapkan Jualin HRIS & Invoice tanpa biaya sekarang |
| **PK** | `uuid.UUID` (§20) | UUIDv7 | Menghindari fragmentasi index pada tabel besar |
| **WhatsApp** | Hanya tombol di Flutter | Auto-Activity di MVP, click-tracking pasca-MVP | Kanal lead #1 di Indonesia |
| **Domain** | `crm.com`, `app.crm.com` | `jualin.id`, `app.jualin.id/crm` | Satu origin untuk semua produk Jualin |

---

*Dokumen ini adalah hasil analisis teknis dan produk. Keputusan final ada pada pemilik produk. Setiap rekomendasi disertai alasan agar bisa ditolak secara sadar, bukan diikuti secara buta.*
