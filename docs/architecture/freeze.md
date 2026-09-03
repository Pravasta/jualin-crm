# Jualin CRM — Architecture Freeze

> ## 🔒 FROZEN — 17 Agustus 2026
>
> Dokumen ini adalah **acuan tunggal** untuk domain model, scope, dan aturan arsitektur Jualin CRM.
> Perubahan setelah titik ini **hanya melalui ADR** — tidak boleh diubah secara diam-diam,
> tidak boleh dilanggar tanpa catatan.
>
> **Basis keputusan:**
>
> | Sumber | Peran |
> |---|---|
> | Product & Architecture Decisions (dokumen pemilik produk, 17 Agu 2026) | Keputusan produk — final |
> | [`ADR-007`](../decisions/ADR-007-user-organization-cardinality.md) | Kardinalitas User → Organization — Accepted |
> | `docs/brainstorming/*` | Arsip. Tidak lagi menjadi acuan. |
>
> **Status implementasi:** belum ada satu baris kode pun. Langkah berikutnya adalah
> Bootstrap Documentation, lalu Session 1 — Foundation.
>
> ### Amandemen — 17 Agustus 2026
>
> Enam penambahan setelah freeze awal. Semuanya bersifat **tambahan**; tidak ada keputusan
> sebelumnya yang dibatalkan.
>
> | # | Amandemen | Bagian |
> |---|---|---|
> | 1 | Kebijakan pengiriman email & batas transaksi (Aturan #32) | 5.3, Phase 1 |
> | 2 | Kebijakan penonaktifan membership — reassign lead + cabut token | 2.3, Phase 1 |
> | 3 | Konvensi API dikunci sejak bootstrap (Aturan #33) | 5.2, 6.1 |
> | 4 | `leads.lead_number` — identitas yang bisa diucapkan manusia | 2.3, 8.4 |
> | 5 | Kolom `version` — deteksi konflik tulis (Aturan #35) | 2.3, 8.4 |
> | 6 | Rate limit endpoint pengirim email (Aturan #34) | 5, Phase 1 |
> | 7 | **Delivery workflow** — PRD → TD → Issue → Branch → PR. `docs/features/` diganti `docs/phases/`. Lihat [ADR-008](../decisions/ADR-008-delivery-workflow.md) dan [`docs/workflow.md`](../workflow.md). | 6 |
>
> Jumlah aturan arsitektur: **31 → 35**. Penomoran 1–31 tidak bergeser.

---

## Daftar Isi

- [A. Rekonsiliasi Inkonsistensi](#a-rekonsiliasi-inkonsistensi)
- [1. Final Domain Model](#1-final-domain-model)
- [2. Final Entity Relationship](#2-final-entity-relationship)
- [3. Final MVP Scope](#3-final-mvp-scope)
- [4. Final Roadmap Phase 0–5](#4-final-roadmap-phase-05)
- [5. Final Architecture Rules](#5-final-architecture-rules)
- [6. Final Documentation Structure](#6-final-documentation-structure)
- [7. Keputusan Identity — Terselesaikan](#7-keputusan-identity--terselesaikan)
- [8. Rekomendasi Migration Pertama](#8-rekomendasi-migration-pertama)

---

# A. Rekonsiliasi Inkonsistensi

Tiga hal di dokumen keputusan saling bertentangan secara halus. Saya rekonsiliasi dengan interpretasi paling masuk akal. **Bila ada yang salah, koreksi sebelum menyetujui freeze ini** — ketiganya akan membuat session membangun hal yang keliru.

## A1. Organization dibuat saat registrasi, bukan setelah login

| Sumber | Menyatakan |
|---|---|
| Decisions §23 (MVP Core Flow) | `Register → Email Verification → Login → Create Organization → Invite Employee` |
| Decisions §16 (Subscription) | `Register → Organization → Free Subscription` |
| Brainstorming §7 | Registrasi mengisi Organization Name; backend membuat User + Organization + Membership + Subscription |

**Rekonsiliasi:** Organization dibuat **atomik di dalam transaksi registrasi**, bukan sebagai langkah terpisah setelah login.

```
POST /v1/auth/register  { organization_name, full_name, email, password }
    └── satu transaksi:
          User + Organization + Membership(owner) + Subscription(free)
    └── kirim email verifikasi
```

**Alasan:** tidak ada state "user tanpa organization" yang valid di produk ini. Bila organization dibuat setelah login, Anda harus menangani user yatim — dan setiap endpoint harus mengecek kemungkinan tenant context kosong. Itu percabangan yang tidak dibayar oleh manfaat apapun.

> Urutan di §23 saya baca sebagai urutan **pengalaman pengguna**, bukan urutan teknis. Bila maksud Anda benar-benar organization dibuat terpisah setelah login, ini harus dikoreksi sekarang.

## A2. "Sales Pipeline" masuk scope, tapi Pipeline ditunda

| Sumber | Menyatakan |
|---|---|
| Decisions §9 | `Sales Pipeline` termasuk dalam scope produk |
| Decisions §25 | `Advanced pipeline` belum perlu diputuskan |
| Decisions §24 Phase 9 | `Advanced pipeline` |

**Rekonsiliasi:** dua hal berbeda dengan nama mirip.

| Istilah | Arti | Kapan |
|---|---|---|
| **Pipeline (konsep)** | Urutan `lead.status` — `new → contacted → qualified → proposal → won/lost` | ✅ MVP, sebagai enum |
| **Pipeline (entity)** | Stage yang bisa dikonfigurasi per organization, tabel tersendiri, drag & drop | ⛔ Phase 9 |

Di MVP, **`lead.status` adalah pipeline-nya**. Tidak ada tabel `pipelines` atau `pipeline_stages`.

## A3. Notification tidak ada di fase manapun sebelum Phase 5

| Sumber | Menyatakan |
|---|---|
| Decisions §23 | Core flow: `Owner assign Lead → Employee menerima notification` |
| Decisions §24 Phase 2 | Lead, status, source, Activity, Task, Assignment, Customer, Idempotency — **tanpa notification** |
| Decisions §24 Phase 5 | Push notification |

**Rekonsiliasi:** pisahkan penyimpanan dari pengiriman.

| Komponen | Fase |
|---|---|
| Tabel `notifications` + pembuatan record saat assignment + endpoint daftar/mark-read | **Phase 2** |
| `device_tokens` + pengiriman FCM + deeplink | **Phase 5** |

**Alasan:** notification in-app adalah source of truth; push hanya pengantar yang *best-effort*. Bila record hanya dibuat saat push dibangun, Phase 3 dan 4 tidak punya jejak assignment, dan Anda kehilangan data yang tidak bisa direkonstruksi.

---

# 1. Final Domain Model

## 1.1 Klasifikasi tabel

Klasifikasi ini menentukan aturan yang berlaku pada setiap tabel. Setiap tabel baru **wajib** dinyatakan masuk kelas mana.

| Kelas | Ciri | Tabel |
|---|---|---|
| **Tenant root** | Merepresentasikan tenant itu sendiri | `organizations` |
| **Global identity** | Lintas tenant, **tanpa** `organization_id` | `users`, `email_verification_tokens`, `password_reset_tokens` |
| **Tenant-scoped** | Wajib `organization_id` + `UNIQUE (id, organization_id)` | semua tabel bisnis lainnya |
| **Session** | Terikat user **dan** organization aktif | `refresh_tokens` |

> **`users` sengaja tidak punya `organization_id`.** Keanggotaan tinggal di `memberships`, bukan di `users`. Ini satu-satunya pengecualian terhadap Aturan #1 di bagian 5, dan pengecualian ini bersifat tertutup — tidak boleh ada tabel *bisnis* lain yang mengikutinya.

## 1.1a Model User → Organization (ADR-007)

Dua lapisan dengan aturan berbeda. Membedakan keduanya adalah inti keputusan ini.

| Lapisan | Aturan |
|---|---|
| **Produk / UX** | **Normal product experience: 1 user = 1 organization.** Registrasi selalu membuat organization baru. **Tanpa** organization switcher. **Tanpa** UI "buat organisasi lain". |
| **Schema** | `User → N Membership`. **Tanpa** `UNIQUE(user_id)` pada `memberships`. |

> *"1 user = 1 organization" adalah **pengalaman produk normal**, bukan invariant sistem.*
>
> Ia menggambarkan apa yang dialami hampir semua pengguna, **bukan** batasan yang ditegakkan database. Karena itu ia **tidak** berarti "1 user hanya boleh punya 1 membership" sebagai aturan data.

**Kenapa dibedakan seperti ini.** Pengalaman normal itu benar dan ingin dipertahankan — pengguna tidak perlu memahami konsep workspace, tidak memilih organization saat login, tidak melihat switcher. Tetapi ia **normal**, bukan **selalu**: undangan employee membuat multi-membership tidak terhindarkan, karena seseorang yang sudah punya akun bisa diundang ke organization lain, dan employee yang pindah kerja harus bisa memakai akun yang sama.

Menegakkan pengalaman normal sebagai constraint database tidak menambah kesederhanaan apapun — kesederhanaan datang dari absennya UI, bukan dari adanya constraint — tetapi menciptakan fragmentasi akun yang tidak bisa dibatalkan.

**Jalur pengecualian yang diakui:** menjadi anggota organization kedua hanya bisa terjadi lewat **undangan**, tidak pernah atas inisiatif pengguna sendiri. Tidak ada endpoint atau tombol untuk membuat organization kedua.

**Konsekuensi yang wajib ditegakkan:**

| # | Kewajiban | Ditegakkan di |
|---|---|---|
| 1 | Registrasi selalu membuat organization baru; email yang sudah ada ditolak dengan arahan untuk login | Service layer |
| 2 | Tidak ada endpoint/UI untuk membuat organization kedua | Absennya endpoint |
| 3 | Undangan untuk user yang sudah ada **wajib** melalui login — tidak boleh menyetel password | Service layer + test keamanan |
| 4 | Token selalu terikat pada satu `organization_id` | Aturan #5 |
| 5 | Test isolasi **wajib** mencakup user dengan dua membership | Harness Phase 1 |

> Kewajiban #5 tidak boleh dilewatkan. Karena schema mengizinkan keadaan yang tidak diekspos UI, satu-satunya penjaga adalah test. Tanpa kasus ini di harness, kebocoran lintas organization pada akun multi-membership tidak akan pernah tertangkap.

## 1.2 Entity MVP (Phase 0–5)

### Identity & Tenancy — Phase 1

| Entity | Peran |
|---|---|
| `organizations` | Tenant. Akar seluruh isolasi data. |
| `users` | Identitas login global. Email unik lintas sistem. |
| `memberships` | Keanggotaan user pada organization + role. **Inilah "Employee".** |
| `invitations` | Undangan bergabung ke organization |
| `email_verification_tokens` | Verifikasi email sekali pakai |
| `password_reset_tokens` | Reset password sekali pakai |
| `refresh_tokens` | Sesi, dengan rotasi & deteksi penggunaan ulang |
| `audit_logs` | Jejak audit bisnis, tenant-scoped |

### CRM Core — Phase 2

| Entity | Peran |
|---|---|
| `leads` | Domain inti. Semua sumber capture menghasilkan bentuk ini. |
| `customers` | Hasil konversi Lead. Entity terpisah. |
| `activities` | Yang **sudah terjadi**. Append-only, immutable. |
| `tasks` | Yang **harus dilakukan**. Mutable. |
| `notifications` | Notifikasi in-app (lihat A3) |

### Public API — Phase 4

| Entity | Peran |
|---|---|
| `api_keys` | Kredensial rahasia milik organization |

### Embedded Form — Phase 6

| Entity | Peran |
|---|---|
| `forms` | Definisi form + `public_key` |

### Mobile — Phase 5

| Entity | Peran |
|---|---|
| `device_tokens` | Token push per perangkat |

## 1.3 Entity yang **tidak** dibuat di MVP

Ditegaskan agar tidak muncul kembali di session mendatang:

| Entity | Alasan | Kapan dipertimbangkan |
|---|---|---|
| `employees` | Employee = membership dengan `role='employee'` | Tidak pernah |
| `roles`, `permissions` | Role adalah enum. RBAC dinamis belum dibutuhkan. | Saat pelanggan minta custom role |
| `assignments` | `leads.assigned_to_membership_id` + Activity untuk riwayat | Saat butuh beberapa assignee aktif sekaligus |
| `form_submissions` | Submission valid **adalah** Lead + `raw_payload` | Saat perlu menyimpan submission yang ditolak/spam |
| `teams` | Manager melihat seluruh organization | Saat ada beberapa divisi sales |
| `contacts` | Customer sudah cukup untuk SMB | Saat ada pelanggan B2B dengan banyak PIC |
| `pipelines`, `pipeline_stages` | `lead.status` adalah pipeline (lihat A2) | Phase 9 |
| `deals` | Ditunda (Decisions §11) | Setelah Phase 5 |
| `products` | Ditunda (Decisions §10) | Bersama Deal, bila perlu |
| `webhook_endpoints`, `webhook_deliveries` | Ditunda (Decisions §15) | Phase 7 |
| `plans`, `subscriptions`, `usage_counters` | Phase 8 — **kecuali** lihat catatan di bawah | Phase 8 |

### ⚠️ Pengecualian: `subscriptions` di Phase 1

Decisions §16 menyatakan setiap organization mendapat Free Plan **sejak registrasi**, sementara §24 menempatkan Subscription di Phase 8.

**Rekonsiliasi:** buat versi minimal di Phase 1, kembangkan di Phase 8.

| Phase 1 — minimal | Phase 8 — lengkap |
|---|---|
| `subscriptions` (organization_id, plan_code, status, dates, external_reference) | `plans` sebagai tabel + `entitlements` |
| `plan_code = 'free'` sebagai konstanta di kode | `usage_counters` + penegakan limit |
| Tanpa penegakan limit apapun | Upgrade + integrasi payment service |

**Alasan:** registrasi harus menghasilkan subscription (§16), jadi tabelnya harus ada di Phase 1. Tapi seluruh mesin limit/usage/plan tidak dibutuhkan sampai Phase 8. Membuat baris subscription tanpa mesin penegakan adalah biaya yang mendekati nol; menambahkan tabelnya belakangan berarti backfill untuk semua organization yang sudah ada.

---

# 2. Final Entity Relationship

## 2.1 Diagram

```
┌─ GLOBAL (tanpa organization_id) ────────────────────────────────────┐
│                                                                     │
│   users ──┬──< email_verification_tokens                            │
│           └──< password_reset_tokens                                │
└───────────┼─────────────────────────────────────────────────────────┘
            │
            │ 1..N
            ▼
┌─ TENANT ──────────────────────────────────────────────────────────────┐
│                                                                       │
│   organizations ──┬──< memberships >── users                          │
│                   │        │  (role: owner|admin|manager|employee)    │
│                   │        │                                          │
│                   │        ├──< refresh_tokens                        │
│                   │        ├──< device_tokens          [Phase 5]      │
│                   │        └──< invitations (invited_by)              │
│                   │                                                   │
│                   ├──< subscriptions        (1 aktif)                 │
│                   ├──< invitations                                    │
│                   ├──< audit_logs                                     │
│                   ├──< api_keys             [Phase 4]                 │
│                   ├──< forms                [Phase 6]                 │
│                   │                                                   │
│                   ├──< LEADS ──────────┬──< activities  (append-only) │
│                   │     │              └──< tasks                     │
│                   │     │                                             │
│                   │     ├─ assigned_to_membership_id ──> memberships  │
│                   │     ├─ source_api_key_id ──────────> api_keys     │
│                   │     ├─ source_form_id ─────────────> forms        │
│                   │     └─ converted_customer_id ──────> customers    │
│                   │                                                   │
│                   ├──< customers                                      │
│                   └──< notifications ──> memberships (recipient)      │
└───────────────────────────────────────────────────────────────────────┘
```

## 2.2 Tabel relasi

| Relasi | Kardinalitas | FK | Catatan |
|---|---|---|---|
| users ↔ organizations | M-N via memberships | — | Satu membership aktif per (user, org) |
| memberships → organizations | N-1 | biasa | |
| memberships → users | N-1 | biasa | |
| refresh_tokens → memberships | N-1 | **composite** | Sesi terikat pada satu organization |
| invitations → organizations | N-1 | biasa | |
| invitations → memberships (`invited_by`) | N-1 | **composite** | |
| subscriptions → organizations | 1-1 aktif + histori | biasa | Partial unique untuk yang aktif |
| leads → memberships (`assigned_to`) | N-1, nullable | **composite** | |
| leads → customers (`converted_customer_id`) | 1-1, nullable | **composite** | |
| leads → api_keys (`source_api_key_id`) | N-1, nullable | **composite** | Phase 4 |
| leads → forms (`source_form_id`) | N-1, nullable | **composite** | Phase 6 |
| activities → leads | N-1 | **composite** | |
| activities → memberships (`actor`) | N-1, nullable | **composite** | Null bila aktor sistem/API key |
| tasks → leads | N-1 | **composite** | |
| tasks → memberships (`assigned_to`, `created_by`) | N-1 | **composite** | |
| notifications → memberships (`recipient`) | N-1 | **composite** | |
| customers → organizations | N-1 | biasa | |
| audit_logs → memberships (`actor`) | N-1, nullable | **composite** | |

> **"composite"** berarti `FOREIGN KEY (x_id, organization_id) REFERENCES t (id, organization_id)`.
> Aturannya sederhana dan tanpa pengecualian: **setiap FK yang menunjuk tabel tenant-scoped memakai bentuk composite.** FK yang menunjuk `organizations` atau `users` memakai bentuk biasa.

## 2.3 Keputusan modeling yang perlu dicatat

### `leads.source` — metode capture, bukan channel marketing

```
source ∈ { manual, api, form, webhook }
```

Ini menjawab **"lewat pintu mana lead masuk"**, bukan "dari kampanye mana".

Attribution channel (Website, Facebook, Instagram, Referral, WhatsApp) adalah konsep terpisah dan **ditunda**. Bila digabung sekarang, enum akan mencampur dua dimensi berbeda dan menjadi mustahil dilaporkan dengan benar.

### Sumber lead memakai kolom FK, bukan kolom polymorphic

Alih-alih `source_id UUID` tanpa FK, gunakan kolom terpisah dengan FK sungguhan:

```
source_api_key_id  → api_keys    (Phase 4)
source_form_id     → forms       (Phase 6)
```

**Alasan:** Decisions §38 mensyaratkan *proper foreign key*. Kolom polymorphic `source_id` tidak bisa memiliki FK, sehingga integritas referensialnya bergantung sepenuhnya pada kode aplikasi. Dengan hanya 2–3 sumber, kolom terpisah lebih murah daripada kehilangan integritas.

### Riwayat assignment tinggal di `activities`

`leads.assigned_to_membership_id` menyimpan **keadaan sekarang**. Riwayat perpindahan tersimpan sebagai activity bertipe `lead_assigned` / `lead_unassigned` beserta `metadata` (from/to). Sesuai Decisions §5.

### Lead boleh tidak punya kontak

Tidak ada `CHECK` yang mewajibkan email atau telepon terisi.

**Alasan:** menolak lead di titik ingest berarti membuang data pelanggan — kerusakan yang tidak bisa dibatalkan, dan pelanggan akan menyalahkan Anda, bukan pengirimnya. Lead tanpa kontak tetap diterima, `raw_payload` menyimpan aslinya, dan UI menampilkannya sebagai tidak dapat ditindaklanjuti.

### `leads.lead_number` — identitas yang bisa diucapkan manusia

Primary key berupa UUIDv7 benar untuk database, tetapi **tidak bisa dipakai manusia**. Sales akan menyebut lead di grup WhatsApp, di telepon, dan saat rapat. Tidak ada yang akan membacakan `01931f2e-8c4a-7...`.

| Aspek | Ketentuan |
|---|---|
| Kolom | `lead_number integer NOT NULL` pada `leads` |
| Keunikan | `UNIQUE (organization_id, lead_number)` — bernomor **per organization**, mulai dari 1 |
| Alokasi | Kolom penghitung `organizations.next_lead_number`, dikunci `SELECT … FOR UPDATE` di dalam transaksi pembuatan lead yang sama |
| Tampilan | `#1024` di UI. UUID tidak pernah ditampilkan kepada pengguna. |

**Kenapa sekarang:** menambahkannya belakangan berarti backfill seluruh tabel **dan** menentukan urutan retroaktif untuk lead yang sudah ada — urutan yang tidak akan pernah benar-benar sesuai dengan yang diingat pengguna.

> Alokasi ini menyerialkan pembuatan lead **per organization**. Pada volume MVP dampaknya tidak terasa; bila suatu saat menjadi bottleneck, itu pertanda pertumbuhan yang bagus dan bisa diatasi terpisah.

### `version` — deteksi konflik tulis

Konsekuensi langsung dari antrian aksi offline di Phase 5:

```
09:00  Employee (mode pesawat)   ubah status lead → contacted
09:05  Owner (dashboard)         reassign lead itu ke orang lain
09:30  Employee dapat sinyal     antrian tersinkron
```

Tanpa kebijakan, sinkronisasi **menimpa perubahan owner secara diam-diam**. Tidak ada error, tidak ada jejak — owner hanya melihat assignment-nya "kembali sendiri".

| Aspek | Ketentuan |
|---|---|
| Kolom | `version integer NOT NULL DEFAULT 1` pada `leads` dan `tasks` |
| Update | `WHERE id = $1 AND organization_id = $2 AND version = $3`, lalu `SET version = version + 1` |
| Konflik | 0 baris terpengaruh → **409 Conflict** dengan keadaan terkini di body |
| Client | Menampilkan konflik kepada pengguna. **Tidak pernah** menimpa otomatis. |

Ini **deteksi**, bukan merge otomatis dan bukan CRDT. Kolomnya murah; yang mahal adalah membangun antrian offline dengan asumsi last-write-wins lalu menyadari datanya sudah hilang.

> `activities` tidak memerlukan `version` — ia append-only, sehingga tidak ada yang bisa ditimpa.

### Kebijakan penonaktifan membership

Soft delete menjaga integritas FK, tetapi tidak menjawab apa yang terjadi pada pekerjaan yang sedang dipegang. Tanpa kebijakan eksplisit, perilaku bawaannya merusak:

> Lead tetap ter-assign ke membership yang sudah tidak aktif. Ia **tidak muncul di "My Leads" siapapun** karena pemiliknya tidak bisa login lagi, dan **tidak tertangkap filter "belum ter-assign"** karena secara teknis ia sudah ter-assign. Lead membusuk dalam diam.

Sales resign adalah kejadian normal. Ini kehilangan pendapatan yang tidak menghasilkan satupun error.

**Ketentuan:**

| # | Aturan | Ditegakkan di |
|---|---|---|
| 1 | Operasi penonaktifan membership **menolak** dijalankan bila masih ada lead atau task terbuka, kecuali disertai keputusan eksplisit: reassign ke membership lain, atau lepas assignment (`assigned_to = NULL`) | Service layer |
| 2 | Seluruh `refresh_tokens` milik membership itu **dicabut** dalam transaksi yang sama | Service layer |
| 3 | Dashboard menyediakan filter permanen **"lead tanpa pemilik aktif"** sebagai jaring pengaman | Phase 3 |
| 4 | Perubahan assignment tetap tercatat sebagai Activity | Otomatis |

> Ketentuan #2 tidak boleh dilewatkan. Tanpanya, employee yang sudah keluar tetap bisa membuka aplikasi sampai token kedaluwarsa — beserta seluruh daftar lead di dalamnya.

## 2.4 Lead status — final MVP

```
                    ┌──────────────────────────────────► spam
                    │
new ──► contacted ──► qualified ──► proposal ──► won
 │          │             │             │
 │          │             │             │
 └──────────┴─────────────┴─────────────┴────────────► lost
 │
 └──────────────────────────────────────────────────► unqualified
```

| Status | Arti |
|---|---|
| `new` | Baru masuk, belum disentuh |
| `contacted` | Sudah dihubungi |
| `qualified` | Terverifikasi sebagai calon pembeli |
| `proposal` | Penawaran sudah diberikan |
| `won` | Berhasil — memicu konversi ke Customer |
| `lost` | Gagal — **wajib** `lost_reason` |
| `unqualified` | Bukan calon pembeli (salah sasaran, iseng) |
| `spam` | Sampah dari form/API publik |

**Aturan:**

1. `spam` dan `unqualified` **dikecualikan** dari seluruh metrik konversi. Tanpa pemisahan ini, sampah dari form publik akan merusak angka yang justru menjadi alasan owner membayar.
2. **Tidak ada status `assigned`.** Assignment ortogonal terhadap status — lead bisa `new` dan sudah ter-assign.
3. Transisi divalidasi di service layer. Mundur satu langkah diizinkan (`qualified → contacted`); melompat dari `new` ke `won` tidak.
4. `lost_reason` wajib saat `lost`: `price` · `competitor` · `timing` · `no_response` · `not_interested` · `other`.
5. Saat masuk `won`, konversi ke Customer **tidak otomatis** — ia aksi eksplisit. `won` berarti kesepakatan tercapai; konversi berarti relasi pelanggan dibuat.

> **Catatan untuk masa depan:** ketika Deal dibangun (pasca-Phase 5), `proposal` dan `won` berpindah ke Deal dan daftar status Lead menyusut menjadi `new → contacted → qualified → converted | lost | unqualified | spam`. **Ini rencana yang disengaja, bukan inkonsistensi** — dicatat di `ADR-006`.

## 2.5 Activity type — final MVP

| Type | Dibuat oleh | Dipicu oleh |
|---|---|---|
| `lead_created` | sistem | Lead dibuat dari sumber manapun |
| `lead_assigned` | sistem | Assignment ditetapkan/diubah |
| `lead_unassigned` | sistem | Assignment dilepas |
| `status_changed` | sistem | Perubahan status |
| `lead_converted` | sistem | Konversi ke Customer |
| `note_added` | pengguna | Catatan manual |
| `call_logged` | pengguna | Tombol Call di mobile — **otomatis** |
| `whatsapp_opened` | pengguna | Tombol WhatsApp di mobile — **otomatis** |
| `task_created` | sistem | Task dibuat |
| `task_completed` | sistem | Task diselesaikan |

**`call_logged` dan `whatsapp_opened` dibuat otomatis oleh mobile app**, bukan diisi manual. Sales tidak akan pernah mengisi log secara manual; bila sistem tidak melakukannya, timeline akan kosong dan seluruh nilai reporting hilang.

Activity bersifat **append-only**: tanpa `updated_at`, tanpa `deleted_at`, tanpa endpoint edit/hapus.

---

# 3. Final MVP Scope

**Definisi MVP** — core product loop dari Decisions §23:

> Owner mendaftar → verifikasi email → login → undang employee → employee login →
> owner membuat API key → website mengirim lead → owner meng-assign →
> employee menerima notifikasi → follow-up dari HP → update → konversi ke Customer

Semua yang tidak diperlukan kalimat itu bukan MVP.

## 3.1 Wajib — Backend

| Area | Cakupan | Phase |
|---|---|---|
| Foundation | Config, migration, logging, error, health, Docker, CI, test harness | 0 |
| Identity | Register (atomik: user+org+membership+subscription), verifikasi email, login, refresh + rotasi, logout, forgot/reset password | 1 |
| Invitation | Undang, terima, batalkan, kedaluwarsa, kirim ulang | 1 |
| Penonaktifan membership | Reassign/lepas lead terbuka + cabut refresh token (2.3) | 1 |
| Pengiriman email | Setelah commit, dengan endpoint kirim ulang (5.3) | 1 |
| Tenant context | Resolusi principal, tenant-scoped repository | 1 |
| RBAC | 4 role, ditegakkan di service layer | 1 |
| Isolation test | Harness generik atas daftar route | 1 |
| Audit log | Aksi sensitif | 1 |
| Subscription minimal | Baris free saat registrasi (tanpa penegakan) | 1 |
| Lead | CRUD, status + transisi, source, filter, pagination, idempotency, `lead_number`, `version` | 2 |
| Assignment | Manual, ke membership | 2 |
| Activity | Append-only, auto-log | 2 |
| Task | due_at, status, assignee | 2 |
| Customer | Konversi dari Lead | 2 |
| Notification | Tabel + record saat assignment + daftar/mark-read | 2 |
| API Key | Create, list, revoke, scope `leads:write` | 4 |
| Public Lead API | `POST /v1/leads`, validasi, rate limit, idempotency, dokumentasi | 4 |

## 3.2 Wajib — Dashboard (Phase 3)

Auth UI · **Lead list & detail** · assignment · employee management + undangan + penonaktifan · filter **"lead tanpa pemilik aktif"** · task · konversi ke customer · metrik dasar · settings organization

> **Lead list & detail adalah produknya.** Layar itu dibuka ratusan kali sehari; sisanya sesekali. Alokasikan usaha sesuai perbandingan itu.

**Metrik dasar Phase 3** — tanpa Deal, tidak ada angka revenue:

| Metrik | Tersedia |
|---|---|
| Lead masuk per periode | ✅ |
| Lead per status | ✅ |
| Lead belum ter-assign | ✅ |
| Conversion rate (mengecualikan spam & unqualified) | ✅ |
| Performa per employee (jumlah, waktu respons, konversi) | ✅ |
| **Nilai revenue / deal** | ❌ Menunggu Deal, pasca-Phase 5 |

Catat ini sebagai ekspektasi yang disepakati, agar tidak muncul sebagai kejutan di Phase 3.

## 3.3 Wajib — Mobile (Phase 5)

Login + biometric unlock · My Leads · My Tasks · lead detail · Call/WhatsApp dengan auto-Activity · ubah status · catatan · push notification · **cache baca offline**

## 3.4 Di luar MVP

| Ditunda | Ke |
|---|---|
| Embedded Form | Phase 6 |
| Webhook (inbound & outbound) | Phase 7 |
| Penegakan limit, usage, upgrade, payment | Phase 8 |
| Deal, Pipeline entity, reports lanjutan, automation | Phase 9 |
| Organization switcher | Saat ada kebutuhan nyata |
| Round robin, assignment rule | Setelah manual terbukti dipakai |
| Custom field, Contact, Team, RBAC dinamis, SSO | Saat diminta |
| Import/export CSV | **Naikkan prioritas bila calon pelanggan datang dari spreadsheet — dan biasanya begitu** |

---

# 4. Final Roadmap Phase 0–5

## Phase 0 — Foundation

| | |
|---|---|
| **Tujuan** | Menetapkan setiap pola yang akan ditiru 15 session berikutnya |
| **Cakupan** | Struktur project Go, `cmd/api`, Docker Compose (app + PostgreSQL), config ter-validasi saat boot, structured logging + request ID, error terpusat, tooling migration + migration baseline, health check, test harness (Postgres asli), CI (lint + test + build) |
| **Tidak termasuk** | Business logic, endpoint domain, tabel domain, auth |
| **Selesai bila** | `docker compose up` → `/health` merespons · migration up & down bersih · config invalid gagal saat startup dengan pesan jelas · log JSON ber-request-id · error berbentuk konsisten · test jalan terhadap PostgreSQL asli · CI hijau |
| **Session** | 1 |

## Phase 1 — Auth & Organization

| | |
|---|---|
| **Tujuan** | Fondasi tenancy yang benar sebelum ada satupun data bisnis |
| **Cakupan** | organizations, users, memberships, invitations, token tables, subscription minimal, audit_logs · registrasi atomik · verifikasi email · login · refresh + rotasi + deteksi penggunaan ulang · logout · forgot/reset password · undangan employee · **penonaktifan membership + pencabutan token** · tenant context · RBAC · **harness test isolasi** · rate limit login **dan endpoint pengirim email** |
| **Pengiriman email** | Setelah commit, tanpa tabel `jobs` (Aturan #32, lihat 5.3). Wajib ada endpoint kirim ulang untuk verifikasi dan undangan. |
| **Selesai bila** | Dua organization bisa dibuat · owner mengundang employee, employee menyetel password & login · menonaktifkan employee mencabut sesinya dan menolak berjalan bila masih ada lead terbuka tanpa keputusan reassignment · harness isolasi berjalan di CI dan **gagal** bila `organization_id` sengaja dihapus dari satu query |
| **Session** | 2 — users, organizations, memberships, registrasi<br>3 — verifikasi email, login, refresh, tenant context<br>4 — RBAC, invitation, harness isolasi |

> **Kriteria "selesai" Phase 1 disengaja lebih keras dari biasanya:** harness isolasi harus terbukti bisa gagal. Test isolasi yang selalu hijau karena tidak benar-benar menguji apapun lebih berbahaya daripada tidak ada test — ia memberi rasa aman palsu pada 15 session berikutnya.

## Phase 2 — CRM Core

| | |
|---|---|
| **Tujuan** | Model domain lengkap, tervalidasi lewat API internal sebelum API publik dibekukan |
| **Cakupan** | leads · status + validasi transisi · source · idempotency · normalisasi telepon E.164 · activities (append-only + auto-log) · tasks · assignment manual · customers + konversi · notifications |
| **Selesai bila** | Siklus lengkap create → assign → activity → task → konversi berjalan lewat API internal, semuanya tertutup test isolasi tenant |
| **Session** | 5 — Lead core<br>6 — Activity + Task<br>7 — Assignment + Notification<br>8 — Customer + konversi |

## Phase 3 — Owner Dashboard

| | |
|---|---|
| **Tujuan** | 🎯 **Demo pertama** — produk bisa dipakai tanpa curl |
| **Cakupan** | Setup Next.js, auth UI, lead list + filter + pagination, lead detail + timeline, assignment, employee + undangan, task, konversi, metrik dasar (3.2), settings |
| **Selesai bila** | Owner menyelesaikan seluruh core loop tanpa menyentuh terminal |
| **Session** | 9–12 (dipecah per area) |

## Phase 4 — Public API

| | |
|---|---|
| **Tujuan** | 🎯 **Tesis produk terbukti** — capture layer bekerja dari luar |
| **Cakupan** | api_keys (format, hash, lookup, revoke, scope) · autentikasi API · `POST /v1/leads` · validasi · rate limit per key + header `X-RateLimit-*` · idempotency di permukaan API · UI API key management · halaman dokumentasi integrasi |
| **Selesai bila** | `curl` dari mesin luar → lead muncul di dashboard · key yang direvoke ditolak · request berulang dengan `Idempotency-Key` sama mengembalikan lead yang sama, bukan duplikat |
| **Session** | 13 — API Key<br>14 — Public Lead API + rate limit + dokumentasi |

> Halaman dokumentasi integrasi bukan pelengkap. Di produk berharga murah, pelanggan harus bisa onboard sendiri — kualitas halaman ini berdampak langsung ke biaya support.

## Phase 5 — Employee Mobile

| | |
|---|---|
| **Tujuan** | 🎯 **Produk menjadi nyata** — pembeda utama Anda |
| **Cakupan** | Setup Flutter + flavor · login + secure storage + rotasi refresh + biometric · My Leads · My Tasks · lead detail + timeline · Call/WhatsApp dengan auto-Activity · ubah status · catatan · FCM + device_tokens + deeplink · cache baca offline |
| **Autentikasi** | **User session (JWT + refresh token opaque)** — jalur yang sama dengan dashboard, sesuai Aturan #24. Mobile **tidak** memakai API key. Yang berbeda dari dashboard hanya penyimpanan token (secure storage vs cookie) dan masa berlaku access token. |
| **Selesai bila** | Siklus penuh berjalan di HP nyata, dan daftar lead tetap terbaca saat mode pesawat |
| **Session** | 15+ |

> **Kejar keandalan, bukan jumlah layar.** Login yang selalu bekerja dan daftar lead yang muncul saat sinyal hilang lebih bernilai daripada sepuluh layar tambahan. Sales lapangan kehilangan sinyal di basement mall dan area pinggiran — itu kondisi normal, bukan kasus tepi.

## 🚦 Gate setelah Phase 5

Siklus produk sudah lengkap. **Cari 3–5 pengguna nyata sebelum Phase 6.**

Apa yang mereka minta akan berbeda dari tebakan kita, dan itulah yang seharusnya menentukan urutan Phase 6–9. Sekaligus menguji tesis harga Anda dengan orang yang benar-benar membuka dompet.

## Ringkasan dependensi

```
Phase 0 ──► Phase 1 ──► Phase 2 ──┬──► Phase 3 ──► Phase 5 ──► GATE
                                  │
                                  └──► Phase 4  (paralel, bukan prasyarat)
```

| Dependensi | Sifat |
|---|---|
| Phase 5 → Phase 1 | **Keras.** Mobile memakai user session yang dibangun di Phase 1. |
| Phase 5 → Phase 2 | **Keras.** Tidak ada lead, task, atau activity untuk ditampilkan tanpanya. |
| Phase 5 → Phase 3 | **Praktis.** Owner butuh jalan untuk membuat dan meng-assign lead, agar ada yang muncul di aplikasi employee. |
| Phase 5 → Phase 4 | **Bukan prasyarat.** Mobile tidak menyentuh API key sama sekali (Aturan #24). Phase 4 hanya membuat pengujian lebih realistis karena lead masuk seperti di kondisi nyata. |

Phase 3 dan 4 tidak saling bergantung dan boleh ditukar atau dikerjakan paralel.

---

# 5. Final Architecture Rules

Aturan yang **tidak boleh dilanggar**. Setiap pelanggaran harus melalui ADR.
Setiap aturan ditulis agar bisa diperiksa — bila sebuah aturan tidak bisa diverifikasi, ia hanya niat baik.

## Tenancy

| # | Aturan | Cara memeriksa |
|---|---|---|
| 1 | Setiap tabel tenant-scoped punya `organization_id NOT NULL` | Query katalog: tabel bisnis tanpa kolom itu → gagal. Pengecualian hanya `users` + tabel token global. |
| 2 | Setiap tabel tenant-scoped punya `UNIQUE (id, organization_id)` | Query katalog |
| 3 | Setiap FK ke tabel tenant-scoped berbentuk composite | Query katalog atas `information_schema` |
| 4 | Repository tidak punya method tanpa `TenantContext` sebagai parameter pertama | Review + lint |
| 5 | `organization_id` **selalu** dari principal terautentikasi, **tidak pernah** dari body/query/header client | Review; tidak ada DTO yang boleh punya field `organization_id` |
| 6 | Resource milik tenant lain → **404**, bukan 403 | Harness isolasi |
| 7 | Feature dengan tenant boundary wajib punya test isolasi | CI blocking |

## Layering

> ⚠️ **Aturan #8 direvisi oleh [ADR-011](../decisions/ADR-011-layered-packages-and-unit-of-work.md).** Aturan #9–#11 tidak berubah — #11 sebelumnya dinyatakan tapi tidak ditegakkan; ADR-011 menegakkannya secara harfiah.

| # | Aturan |
|---|---|
| 8 | `Handler → Usecase → Repository (interface) → Repository (implementasi) → PostgreSQL`. Setiap paket domain mendeklarasikan lapisnya lewat penamaan berkas: `entity.go`, `port.go`, `usecase.go`, `repository_postgres.go`, `handler_http.go`. Handler **tidak boleh** memanggil repository langsung. **Bukan** folder-per-lapis (`domain/`, `usecase/`, `adapter/` di level `internal/`) — package-by-feature tetap dipertahankan; lihat ADR-011 untuk alasan penolakannya. |
| 9 | Repository tidak berisi business logic. Usecase tidak tahu tentang HTTP. |
| 10 | Otorisasi ditegakkan di **usecase**. UI yang menyembunyikan tombol bukan otorisasi. |
| 11 | Interface didefinisikan di sisi consumer (`port.go` milik paket yang memakainya), memuat hanya method yang benar-benar dipakai — bukan cermin seluruh repository, bukan paket `interfaces/` terpusat. Transaksi lintas repository lewat pola Unit of Work (`Store.InTx`) — lihat ADR-011. |

## Data

| # | Aturan |
|---|---|
| 12 | PK **UUIDv7**, di-generate aplikasi. Tanpa `DEFAULT` di database. |
| 13 | Waktu selalu `timestamptz`, disimpan UTC, dirender di `organizations.timezone`. |
| 14 | Uang `numeric(15,2)` + kolom `currency`. Tidak pernah float. |
| 15 | Enum sebagai `text` + `CHECK`, bukan tipe `ENUM` PostgreSQL. |
| 16 | Index tenant-aware **selalu** berawalan `organization_id`. |
| 17 | JSONB hanya dengan alasan tertulis di migration. Di MVP hanya: `leads.raw_payload`, `activities.metadata`, `audit_logs.old_values/new_values/metadata`, `notifications.data`. |
| 18 | Soft delete (`deleted_at`) untuk entity bisnis. Activity dan audit log tidak pernah dihapus. |
| 19 | Email disimpan lowercase, ditegakkan `CHECK (email = lower(email))`. |

## Keamanan

> ⚠️ **Aturan #20 diberi klausa pengecualian oleh [ADR-013](../decisions/ADR-013-signing-secret-storage.md).** Aturannya apa adanya tetap berlaku untuk kredensial yang **kita verifikasi** (API key, `public_key`, refresh token). Kredensial yang **kita pakai untuk menghasilkan bukti** dengan arah kepercayaan terbalik — sejauh ini hanya signing secret webhook `whsec_` (Phase 7) — disimpan terenkripsi reversibel, kunci di environment, hanya bila tidak ada alternatif hash yang bisa memenuhi fungsinya. Detail & batas di ADR-013 dan `authentication.md`.

| # | Aturan |
|---|---|
| 20 | Password: **argon2id**. API key: **SHA-256** + `subtle.ConstantTimeCompare`. Alasan perbedaannya di `ADR-004`. Pengecualian untuk kredensial arah-terbalik: `ADR-013`. |
| 21 | Raw secret (API key, token undangan, token reset) hanya ditampilkan **sekali**; database menyimpan hash. |
| 22 | Refresh token opaque, disimpan di DB, dirotasi, dengan deteksi penggunaan ulang per `family_id`. |
| 23 | API key **tidak pernah** hadir di sisi klien. Embedded form memakai `public_key` yang hanya bisa submit. |
| 24 | **User App ≠ API Key.** Aplikasi pengguna dan sistem eksternal adalah dua jalur autentikasi terpisah yang tidak boleh saling menggantikan. Lihat 5.1. |
| 25 | Token dashboard di cookie `HttpOnly; Secure; SameSite=Lax` — **tidak pernah** `localStorage`. Konsekuensinya: proteksi CSRF wajib. |
| 26 | Jangan pernah mencatat ke log: password, raw API key, token, payload lead lengkap. Redaksi di logger, bukan di call site. |

## Proses

| # | Aturan |
|---|---|
| 27 | Bila solusi sederhana sudah cukup, pakai itu. Abstraksi hanya setelah ada implementasi kedua yang **nyata**. |
| 28 | Jangan membuat tabel, modul, atau folder untuk kebutuhan yang belum ada. |
| 29 | Fitur yang menyentuh domain di luar scope → **flag sebagai scope discussion**, jangan diimplementasikan. |
| 30 | Bila dokumentasi bertentangan dengan kode: laporkan. Jangan diam-diam mengubah salah satunya. |
| 31 | Setiap akhir session: update `docs/STATUS.md` + `docs/phases/<NN>-<slug>/notes.md` — **di dalam PR**, bukan setelah merge (ADR-008). |

## API, Transaksi & Konkurensi

> Kelompok ini ditambahkan sebagai amandemen. Dinomori 32–35 agar penomoran aturan yang sudah dirujuk di tempat lain tetap stabil.

| # | Aturan | Cara memeriksa |
|---|---|---|
| 32 | Efek samping eksternal (kirim email, panggilan HTTP) **tidak pernah** dilakukan di dalam transaksi database. Lihat 5.3. | Review |
| 33 | Payload JSON mengikuti konvensi di 5.2. Mengubah bentuk response adalah breaking change. | Review + test kontrak |
| 34 | Setiap endpoint yang memicu pengiriman email wajib dibatasi **per alamat email dan per IP**. | Test |
| 35 | Entity yang bisa diubah dari lebih dari satu client (`leads`, `tasks`) memakai optimistic locking lewat kolom `version`. Konflik → **409**, tidak pernah menimpa diam-diam. | Test konkurensi |

## 5.1 Aturan #24 — User App ≠ API Key

Tiga jalur autentikasi, tiga jenis principal. **Tidak ada satupun yang boleh menggantikan yang lain.**

| Jalur | Dipakai oleh | Kredensial | Principal | Punya identitas orang? |
|---|---|---|---|---|
| **User session** | Dashboard, **Mobile** | JWT + refresh token opaque | `membership` | ✅ Ya |
| **API key** | Sistem eksternal milik pelanggan | `jln_live_...` | `organization` | ❌ Tidak |
| **Public form key** | Browser pengunjung | `public_key` | `form` | ❌ Tidak |

### Yang dilarang

| Larangan | Kenapa |
|---|---|
| **Mobile app memakai API key sebagai kredensial** | API key adalah milik organization dan **tidak membawa identitas orang**. Bila mobile memakainya, sistem tidak tahu employee mana yang menelepon lead — `activities.actor_membership_id` menjadi null, assignment tidak bisa difilter per employee, audit log kehilangan aktor, dan RBAC tidak punya role untuk dievaluasi. Seluruh nilai reporting per-sales hilang. |
| **API key mengakses endpoint aplikasi pengguna** | API key tidak boleh mengelola employee, mengubah subscription, atau membuat API key lain. Ditegakkan di middleware, bukan per handler. |
| **User session dipakai menggantikan API key** untuk integrasi server-to-server | Token user berumur pendek dan terikat orang. Bila seseorang keluar dari organization, integrasi pelanggan ikut mati. |
| **API key di-embed di aplikasi mobile** yang didistribusikan | Aplikasi mobile bisa dibongkar. Ini setara menaruh API key di browser (Aturan #23). |

### Aturan turunan

1. **Mobile memakai jalur autentikasi yang sama dengan dashboard** — user session, bukan API key. Yang berbeda hanya penyimpanan token (secure storage vs cookie) dan masa berlaku.
2. Setiap endpoint menyatakan **secara eksplisit** principal apa yang diterimanya. Tidak ada endpoint yang menerima "salah satu dari keduanya" tanpa alasan tertulis.
3. `TenantContext.PrincipalType` wajib diperiksa, bukan hanya `OrganizationID`. Dua request dengan `organization_id` sama tapi principal berbeda **bukan** request yang setara.

## 5.2 Aturan #33 — Konvensi API

Konvensi ini terbentuk secara de facto di Session 3 dan Session 5, jauh sebelum `architecture/api.md` sempat ditulis. Karena itu ia dikunci **sekarang**, bukan di Phase 4 — begitu Next.js dan Flutter menempel, mengubahnya berarti memutus dua client sekaligus.

| Hal | Ketentuan |
|---|---|
| **Penamaan field JSON** | `snake_case` — konsisten dengan kolom database dan konvensi API publik pada umumnya |
| **Format waktu** | ISO 8601 UTC dengan sufiks `Z` — mis. `2026-08-17T09:30:00Z` |
| **Versioning** | Prefix path `/v1/` pada **seluruh** endpoint, termasuk yang internal |
| **Bentuk error** | `{"error": {"code": "...", "message": "...", "details": [...]}}` |
| **`error.code`** | Stabil, machine-readable, `snake_case` — mis. `lead_not_found`, `quota_exceeded`. Client tidak pernah mem-parsing `message`. |
| **`error.message`** | Untuk manusia. Boleh berubah tanpa dianggap breaking. |
| **Bentuk list** | `{"data": [...], "meta": {...}}` — envelope **sejak awal** |
| **Pagination** | Offset: `?page=1&per_page=25`. `meta` berisi `page`, `per_page`, `total`. |
| **Field tak dikenal pada request** | Diabaikan, bukan ditolak — agar client lama tidak rusak saat field baru ditambahkan. Tetap tersimpan di `raw_payload` untuk lead. |

**Kenapa envelope `data`/`meta` sejak awal:** mengembalikan array telanjang terasa lebih bersih, tapi menambahkan metadata pagination belakangan berarti mengubah tipe akar response — breaking change untuk setiap client. Envelope adalah biaya satu kali yang sangat kecil.

**Kenapa offset, bukan cursor:** offset punya masalah "halaman bergeser" saat data baru masuk di atas. Tapi endpoint list bersifat **internal** (dashboard & mobile) dan bukan bagian dari API publik Phase 4 — yang hanya `POST /v1/leads`. Jadi ia bisa diganti ke cursor kapan saja tanpa memutus integrator siapapun. Offset cukup untuk MVP.

> `docs/architecture/api.md` ditulis saat **bootstrap** dengan isi bagian ini. Yang menyusul di Phase 4 hanyalah bab API publik: autentikasi API key, rate limit header, dan idempotency.

## 5.3 Aturan #32 — Efek samping & batas transaksi

Registrasi wajib atomik (A1), dan registrasi juga harus mengirim email verifikasi. Keduanya bertabrakan bila tidak diputuskan:

```
BEGIN
  user + organization + membership + subscription
  kirim email verifikasi        ← di sini?
COMMIT                          ← atau di sini?
```

| Pilihan | Konsekuensi |
|---|---|
| Kirim **di dalam** transaksi | Panggilan SMTP menahan transaksi terbuka ratusan milidetik. Provider lambat → connection pool habis. Gagal kirim → registrasi ikut rollback padahal akunnya sah. |
| Kirim **setelah** commit ✅ | Crash di antara commit dan kirim → user terdaftar tanpa email |
| Outbox (job row di transaksi yang sama, worker mengirim) | Benar, tapi menuntut tabel `jobs` + proses worker sejak Phase 1 |

### Keputusan: kirim setelah commit, tanpa tabel `jobs`

Satu-satunya mode kegagalannya — crash tepat setelah commit — **sudah tertutup oleh tombol "kirim ulang"**, yang memang wajib ada karena B3 menjadikan verifikasi sebagai gerbang login.

Outbox menyelesaikan masalah yang sudah punya penyelesaian lebih murah. Ia melanggar Aturan #27.

**Ketentuan:**

| # | Aturan |
|---|---|
| 1 | Transaksi database hanya berisi operasi database. Kirim email, panggilan HTTP, dan tulis file berada **di luar** commit. |
| 2 | Kegagalan pengiriman **tidak** membatalkan operasi yang sudah commit. Ia dicatat sebagai error terstruktur, bukan ditelan diam-diam. |
| 3 | Setiap alur yang bergantung pada email wajib punya jalur pemulihan mandiri — kirim ulang verifikasi, kirim ulang undangan. |
| 4 | Tabel `jobs` + worker diperkenalkan saat ada kebutuhan async yang **nyata**: push notification (Phase 5) atau outbound webhook (Phase 7). Bukan sebelumnya. |

---

# 6. Final Documentation Structure

> ⚠️ **Diperbarui oleh [ADR-008](../decisions/ADR-008-delivery-workflow.md).**
>
> `docs/features/<f>/{spec,notes}.md` → **`docs/phases/<NN>-<slug>/{prd,td,issues,notes}.md`**.
> Ditambahkan `docs/workflow.md` sebagai prosedur pengerjaan.
>
> Prinsipnya tidak berubah: dokumentasi seminimal mungkin, `STATUS.md` sebagai ledger,
> update wajib di akhir session. Yang berubah hanya unit organisasinya — dari *feature*
> menjadi *phase*, karena PRD dan TD berlaku untuk satu phase utuh, bukan per feature.
>
> **Prosedur yang berlaku: [`docs/workflow.md`](../workflow.md).**

```
CLAUDE.md                          < 150 baris — aturan global

docs/
├── STATUS.md                      ⭐ state ledger, di-update SETIAP session
├── product/
│   ├── decisions.md               keputusan pemilik produk (dokumen Anda)
│   ├── scope.md                   in/out, batas yang akan diuji
│   ├── roadmap.md                 fase & urutan session
│   └── glossary.md                ⭐ definisi istilah — cegah drift penamaan
├── architecture/
│   ├── freeze.md                  ⭐ dokumen ini — acuan tunggal
│   ├── overview.md                stack, layering, struktur modul
│   ├── multi-tenancy.md           ⭐ 4 lapis isolasi — tulis sebelum kode
│   ├── authentication.md          3 jalur, token, rotasi
│   ├── authorization.md           matriks permission
│   ├── database.md                konvensi (bukan salinan schema)
│   └── api.md                     versioning, error, pagination, rate limit
├── features/
│   └── <feature>/
│       ├── spec.md                sebelum implementasi — kontrak
│       └── notes.md               sesudah implementasi — realitas
├── decisions/
│   └── ADR-XXX-*.md
└── brainstorming/                 arsip, read-only

.claude/skills/
└── jualin-backend/
```

## 6.1 Yang harus dibuat sebelum Session 1

| Berkas | Isi | Sumber |
|---|---|---|
| `CLAUDE.md` | Identity, scope, stack, 35 aturan (diringkas), layering, workflow, source of truth | Bagian 5 |
| `docs/STATUS.md` | Kerangka kosong, siap diisi | — |
| `docs/product/decisions.md` | **Pindahkan** `CRM Architecture Decisions.md` dari root | Dokumen Anda |
| `docs/product/scope.md` | In/out + batas yang akan diuji | Decisions §9, review §2.3 |
| `docs/product/glossary.md` | Organization, Membership, Employee, Lead, Customer, Activity, Task, Assignment, Source, Status | Bagian 1–2 |
| `docs/architecture/freeze.md` | Dokumen ini | — |
| `docs/architecture/multi-tenancy.md` | 4 lapis + aturan 1–7 | Bagian 5 |
| `docs/architecture/api.md` | Konvensi API — penamaan, waktu, error, envelope, pagination | Bagian 5.2 |
| `docs/decisions/ADR-001..006` | Lihat 6.3 (ADR-007 sudah ada) | — |
| `.claude/skills/jualin-backend/` | Konvensi Go, layering, pola repository tenant-scoped, penanganan error, pola testing | Bagian 5 |

Berkas `architecture/` lain dibuat saat fasenya dimulai — `authentication.md` di Phase 1, `authorization.md` di Phase 1, `database.md` di Phase 2.

> `api.md` sengaja **naik ke bootstrap**, tidak menunggu Phase 4. Konvensinya sudah dipakai sejak endpoint pertama di Session 3; bila baru ditulis di Phase 4, ia hanya akan mendokumentasikan apapun yang kebetulan terbentuk. Yang menyusul di Phase 4 hanyalah bab API publik: autentikasi API key, header rate limit, dan idempotency.

> **Jangan buat folder kosong.** Folder kosong adalah utang dokumentasi yang terlihat seperti kemajuan.

## 6.2 `docs/STATUS.md` — kerangka

```markdown
# Project Status
Last updated: <tanggal> (Session N — <feature>)

## Selesai
| Feature | Session | Phase | Catatan |

## Sedang Dikerjakan

## Berikutnya

## Utang Teknis
- [ ] ...

## Keputusan Belum Diambil
- ...
```

Bagian **Utang Teknis** sama pentingnya dengan bagian Selesai. Tanpanya, kompromi yang diambil di session 3 akan terlupa di session 12 lalu ditemukan kembali sebagai bug produksi.

## 6.3 ADR yang ditulis sekarang

| ADR | Keputusan | Kenapa perlu dicatat |
|---|---|---|
| `ADR-001-monolith` | Go monolith; kriteria evaluasi ulang | Akan dipertanyakan saat traffic naik |
| `ADR-002-multi-tenancy` | Shared schema + 4 lapis; kenapa RLS ditunda | Akan dipertanyakan saat audit keamanan |
| `ADR-003-employee-as-membership` | Employee bukan tabel | Akan "diperbaiki" oleh session mendatang bila alasannya tidak tercatat |
| `ADR-004-api-key-format` | Format key, kenapa SHA-256 bukan argon2 | Terlihat seperti kesalahan keamanan bila tanpa penjelasan |
| `ADR-005-public-form-key` | Pemisahan public key vs API key | Akan tergoda disederhanakan menjadi satu kredensial |
| `ADR-006-lead-status-as-pipeline` | Status = pipeline di MVP + rencana perpindahan ke Deal | Terlihat seperti inkonsistensi bila tanpa penjelasan |
| `ADR-007-user-organization-cardinality` | ✅ **Sudah ditulis & Accepted** — model produk 1:1, schema 1:N, tanpa `UNIQUE(user_id)` | Absennya constraint akan tampak seperti kelalaian |

Tujuh ini dipilih dengan satu kriteria: **keputusan yang tampak salah bila alasannya tidak diketahui.** Itulah yang akan diubah tanpa sadar oleh session mendatang.

## 6.4 Protokol session

**Awal:** `CLAUDE.md` → `docs/STATUS.md` → skill → `architecture/` yang relevan (multi-tenancy hampir selalu) → `features/<f>/spec.md` → kode berdekatan → rencana → persetujuan → implementasi

**Akhir:** update `STATUS.md` → tulis `notes.md` → ADR bila perlu → update `architecture/*` bila konvensi berubah

> Bila langkah akhir dilewati **sekali saja**, sistem ini kehilangan nilainya: session berikutnya kembali membaca kode untuk merekonstruksi status, dan Anda membayar biaya menulis dokumen tanpa mendapat manfaatnya.

---

# 7. Keputusan Identity — Terselesaikan

## ✅ B1–B5 — keputusan identity (Phase 1)

**Yang terselesaikan di sini adalah keputusan identity**, yaitu B1–B5. Kelima ini menentukan bentuk `users`, `memberships`, `invitations`, dan `refresh_tokens`.

Konsekuensinya: **migration `0002` tidak lagi diblokir.**

Ini **tidak** berarti seluruh keputusan proyek sudah selesai. Yang tersisa dicatat di bawah dan di [Ringkasan Freeze](#ringkasan-freeze) — semuanya berada di luar cakupan identity dan tidak pernah memblokir `0002`.

| # | Pertanyaan | Keputusan | Sumber |
|---|---|---|---|
| **B1** | Email unik secara global? | **Ya.** `users.email` unik lintas sistem, lowercase, ditegakkan `CHECK`. | ADR-007 |
| **B2** | Login saat user punya > 1 membership? | Login menerima `organization_id` opsional. 1 membership → langsung masuk. > 1 → balas daftar organization, client memilih, panggil ulang. Token **selalu** terikat satu organization. | ADR-007 |
| **B3** | Verifikasi email menggerbangi login? | **Ya**, sesuai alur core flow. Wajib ada tombol kirim ulang. **Menerima undangan sekaligus memverifikasi email** — token dikirim ke alamat itu, kepemilikan sudah terbukti, jadi employee yang diundang tidak perlu verifikasi terpisah. | Adopsi (lihat 7.1) |
| **B4** | Undangan untuk email yang sudah punya akun? | **Boleh.** Alurnya bercabang — lihat di bawah. | ADR-007 |
| **B5** | Nama pengguna global atau per organization? | `users.full_name` — satu nama. | Adopsi (lihat 7.1) |

### Alur penerimaan undangan (B4)

```
Undangan diterima
   ├── email belum punya user  → buat user (set password) + membership + tandai verified
   └── email sudah punya user  → WAJIB login dulu, lalu konfirmasi → tambah membership
```

> Cabang kedua **tidak boleh** mengizinkan penyetelan password tanpa autentikasi. Bila diizinkan, undangan menjadi jalur pengambilalihan akun: siapapun yang bisa mengundang sebuah alamat email bisa menyetel ulang password pemiliknya. Ini harus punya test keamanan tersendiri, bukan sekadar dicatat.

## 7.1 Diadopsi tanpa objeksi eksplisit

Enam hal berikut diusulkan sebelum freeze, tidak ditolak, dan **diadopsi ke dalam freeze**. Dicatat terpisah agar bisa dibatalkan satu per satu tanpa membongkar keseluruhan — bukan disembunyikan sebagai bagian yang seolah sudah disepakati.

| # | Adopsi | Dampak bila dibatalkan |
|---|---|---|
| **A1** | Organization dibuat **atomik di dalam transaksi registrasi**, bukan langkah terpisah setelah login. Tidak ada state "user tanpa organization". | Sedang — mengubah alur registrasi & tenant context |
| **A2** | "Sales Pipeline" di MVP = urutan `lead.status`. Tidak ada tabel `pipelines`. | Kecil — Phase 9 |
| **A3** | Tabel `notifications` di Phase 2; `device_tokens` + FCM di Phase 5. | Kecil — pergeseran fase |
| **S1** | `subscriptions` versi minimal dibuat di Phase 1 (baris `free`, tanpa penegakan limit); mesin lengkap tetap Phase 8. | Kecil — satu tabel |
| **M1** | Dashboard Phase 3 **tanpa angka revenue** sampai Deal dibangun. Metrik: lead masuk, per status, belum ter-assign, conversion rate, performa per employee. | Ekspektasi produk, bukan schema |
| **B3, B5** | Lihat tabel di atas. | Kecil |

## 🟠 Menunggu — CRM (Phase 2), di luar cakupan identity

| # | Pertanyaan | Rekomendasi |
|---|---|---|
| B6 | Daftar `lost_reason` final | `price`, `competitor`, `timing`, `no_response`, `not_interested`, `other` |
| B7 | Boleh mengubah status lead mundur? | Ya, satu langkah mundur; melompat maju tidak |
| B8 | Task boleh berdiri tanpa Lead? | **Tidak** di MVP — `lead_id NOT NULL`. Melonggarkan `NOT NULL` nanti murah; mengetatkannya mahal. |
| B9 | Konversi ke Customer otomatis saat `won`? | **Tidak** — aksi eksplisit (lihat 2.4 aturan 5) |

## 🟢 Sudah diputuskan secara default — koreksi bila tidak setuju

Keputusan ini saya ambil karena ada jawaban baku yang jelas. Tidak memblokir, tapi **tercatat sebagai keputusan**, bukan kelalaian:

| Hal | Diputuskan |
|---|---|
| Extension PostgreSQL | Tidak ada. Email lowercase ditegakkan `CHECK`, bukan `citext`. UUIDv7 di-generate Go. |
| Versi PostgreSQL | 16 atau 17 (managed). Tidak bergantung fitur PG 18. |
| `organizations.slug` | Tidak ada — tidak ada subdomain per tenant, tidak ada URL ber-org |
| Status membership | Tidak ada kolom `status`. Nonaktif = `deleted_at` (soft delete menjaga integritas FK pada lead yang pernah di-assign) |
| Failed login | Masuk **log aplikasi**, bukan `audit_logs` — audit log tenant-scoped, sedangkan login gagal belum punya tenant |
| Rate limit login | Phase 1, bukan Phase 4 |
| Penyimpanan idempotency | Kolom di `leads`, bukan tabel tersendiri — resource-nya **adalah** response-nya |
| Timezone default | `Asia/Jakarta` |

---

# 8. Rekomendasi Migration Pertama

## 8.1 Konvensi

| Aspek | Ketentuan |
|---|---|
| Penomoran | `0001_baseline`, `0002_identity`, … — berurutan, tidak pernah diubah setelah di-merge |
| Reversibilitas | Setiap migration punya `down` yang benar-benar bekerja |
| Isi | Hanya DDL. **Tanpa** seed data bisnis. |
| Alasan | Setiap penggunaan JSONB, setiap pelanggaran aturan, ditulis sebagai komentar SQL |
| Nama constraint | Eksplisit — `fk_leads_assigned_membership`, bukan nama bawaan |

## 8.2 Migration `0001_baseline` — Phase 0

Tujuannya membuktikan tooling migration bekerja dan menetapkan satu-satunya utilitas lintas tabel.

**Isi:**

- Fungsi trigger `set_updated_at()` — dipakai setiap tabel yang punya `updated_at`

**Tidak berisi:** tabel domain apapun, extension, seed data.

**Kenapa terpisah dari `0002`:** Phase 0 tidak punya business logic, sehingga tabel domain di sana akan menjadi schema mati yang tidak dipakai kode manapun. Migration ini kecil dengan sengaja — tugasnya membuktikan `up` dan `down` bekerja bersih, dan sebuah fungsi cukup untuk itu.

## 8.3 Migration `0002_identity` — Phase 1

Migration domain pertama. Delapan tabel.

> Ini **spesifikasi**, bukan berkas migration. Berkas ditulis di Session 2 setelah B1–B5 dijawab.

### `organizations`

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK, UUIDv7 dari aplikasi |
| `name` | text | NOT NULL |
| `timezone` | text | NOT NULL DEFAULT `'Asia/Jakarta'` |
| `created_at` / `updated_at` | timestamptz | NOT NULL DEFAULT `now()` |
| `deleted_at` | timestamptz | NULL |

### `users` — global, tanpa `organization_id`

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `email` | text | NOT NULL, **UNIQUE**, `CHECK (email = lower(email))` |
| `password_hash` | text | NOT NULL (argon2id) |
| `full_name` | text | NOT NULL |
| `email_verified_at` | timestamptz | NULL |
| `created_at` / `updated_at` / `deleted_at` | timestamptz | |

### `memberships` — **jangkar seluruh sistem composite FK**

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `organization_id` | uuid | NOT NULL → `organizations(id)` |
| `user_id` | uuid | NOT NULL → `users(id)` |
| `role` | text | NOT NULL, `CHECK (role IN ('owner','admin','manager','employee'))` |
| `created_at` / `updated_at` / `deleted_at` | timestamptz | |

**Constraint & index:**

```sql
-- Jangkar untuk SETIAP composite FK di seluruh database
CONSTRAINT uq_memberships_id_org UNIQUE (id, organization_id)

-- Satu membership aktif per (user, organization)
CREATE UNIQUE INDEX uq_memberships_org_user_active
  ON memberships (organization_id, user_id)
  WHERE deleted_at IS NULL;

CREATE INDEX ix_memberships_org_role
  ON memberships (organization_id, role)
  WHERE deleted_at IS NULL;

-- Untuk resolusi membership saat login
CREATE INDEX ix_memberships_user
  ON memberships (user_id)
  WHERE deleted_at IS NULL;
```

> **`UNIQUE (id, organization_id)`** terlihat berlebihan karena `id` sudah PK. Ia **wajib**: PostgreSQL mensyaratkan unique constraint tepat pada kolom yang direferensikan sebuah composite FK. Tanpa baris ini, seluruh Aturan #3 tidak bisa diterapkan.

### ⛔ TIDAK ADA `UNIQUE(user_id)` — disengaja

```sql
-- JANGAN PERNAH ditambahkan:
-- CREATE UNIQUE INDEX ... ON memberships (user_id);
```

Ini **keputusan sadar** ([ADR-007](../decisions/ADR-007-user-organization-cardinality.md)), bukan kelalaian.

Pengalaman produk normal — "1 user = 1 organization" — ditegakkan di **UI**, dengan tidak menyediakan endpoint atau tombol untuk membuat organization kedua. Schema sengaja dibiarkan terbuka karena undangan employee membuat multi-membership tidak terhindarkan: orang yang sudah punya akun bisa diundang ke organization lain, dan employee yang pindah kerja harus bisa memakai akun yang sama.

Menambahkan constraint ini akan:

1. Membuat undangan gagal untuk email yang sudah terdaftar
2. Memaksa pengguna membuat akun kedua dengan email berbeda
3. Menjadikan perpindahan ke model multi-organization sebagai **proyek merge akun**, bukan penambahan UI

> Bila session mendatang mengira ini kelalaian dan hendak "memperbaikinya": baca ADR-007 terlebih dahulu. Perubahan hanya melalui ADR baru.

### `subscriptions` — versi minimal

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `organization_id` | uuid | NOT NULL |
| `plan_code` | text | NOT NULL DEFAULT `'free'` |
| `status` | text | NOT NULL, `CHECK (status IN ('active','past_due','suspended','canceled'))` |
| `current_period_start` / `current_period_end` | timestamptz | NULL (free tier tanpa periode) |
| `external_reference` | text | NULL — id di payment service |
| `created_at` / `updated_at` | timestamptz | |

```sql
CONSTRAINT uq_subscriptions_id_org UNIQUE (id, organization_id)

CREATE UNIQUE INDEX uq_subscriptions_org_active
  ON subscriptions (organization_id)
  WHERE status = 'active';
```

### `invitations`

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `organization_id` | uuid | NOT NULL |
| `email` | text | NOT NULL, `CHECK (email = lower(email))` |
| `role` | text | NOT NULL, `CHECK` sama seperti membership, **`role <> 'owner'`** |
| `token_hash` | text | NOT NULL UNIQUE (SHA-256 dari token mentah) |
| `invited_by_membership_id` | uuid | NOT NULL |
| `expires_at` | timestamptz | NOT NULL (7 hari) |
| `accepted_at` / `revoked_at` | timestamptz | NULL |
| `created_at` / `updated_at` | timestamptz | |

```sql
CONSTRAINT fk_invitations_inviter
  FOREIGN KEY (invited_by_membership_id, organization_id)
  REFERENCES memberships (id, organization_id)

-- Satu undangan tertunda per (organization, email)
CREATE UNIQUE INDEX uq_invitations_org_email_pending
  ON invitations (organization_id, email)
  WHERE accepted_at IS NULL AND revoked_at IS NULL;
```

> `role <> 'owner'` disengaja: Owner tidak diundang, ia terbentuk saat registrasi. Mengizinkannya membuka jalur eskalasi hak akses lewat undangan.

### `email_verification_tokens` & `password_reset_tokens` — global

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `user_id` | uuid | NOT NULL → `users(id)` |
| `token_hash` | text | NOT NULL UNIQUE |
| `expires_at` | timestamptz | NOT NULL (24 jam / 1 jam) |
| `used_at` | timestamptz | NULL |
| `created_at` | timestamptz | NOT NULL |

Dua tabel dengan bentuk sama. **Jangan digabung** menjadi satu tabel ber-`type` — masa berlaku, alur, dan konsekuensi keamanannya berbeda, dan menggabungkannya mengundang token reset dipakai untuk verifikasi.

### `refresh_tokens`

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `organization_id` | uuid | NOT NULL |
| `membership_id` | uuid | NOT NULL |
| `token_hash` | text | NOT NULL UNIQUE |
| `family_id` | uuid | NOT NULL — untuk deteksi penggunaan ulang |
| `client` | text | NOT NULL, `CHECK (client IN ('dashboard','mobile'))` |
| `device_id` | text | NULL — mobile |
| `user_agent` | text | NULL |
| `ip` | inet | NULL |
| `expires_at` | timestamptz | NOT NULL |
| `revoked_at` | timestamptz | NULL |
| `replaced_by_id` | uuid | NULL → `refresh_tokens(id)` |
| `created_at` | timestamptz | NOT NULL |

```sql
CONSTRAINT fk_refresh_tokens_membership
  FOREIGN KEY (membership_id, organization_id)
  REFERENCES memberships (id, organization_id)

CREATE INDEX ix_refresh_tokens_family ON refresh_tokens (family_id);
```

**Aturan rotasi:** setiap refresh menerbitkan token baru dan menyetel `replaced_by_id` pada yang lama. Bila token yang sudah punya `replaced_by_id` atau `revoked_at` dipakai lagi → **revoke seluruh `family_id`** dan paksa login ulang.

### `audit_logs`

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK |
| `organization_id` | uuid | NOT NULL |
| `actor_type` | text | NOT NULL, `CHECK (actor_type IN ('user','api_key','system'))` |
| `actor_membership_id` | uuid | NULL |
| `action` | text | NOT NULL — `lead.assigned`, `api_key.revoked`, … |
| `entity_type` / `entity_id` | text / uuid | NULL |
| `old_values` / `new_values` / `metadata` | jsonb | NULL — *heterogen secara definisi* |
| `request_id` | text | NULL |
| `created_at` | timestamptz | NOT NULL |

```sql
CONSTRAINT fk_audit_actor
  FOREIGN KEY (actor_membership_id, organization_id)
  REFERENCES memberships (id, organization_id)

CREATE INDEX ix_audit_org_created ON audit_logs (organization_id, created_at DESC);
```

Append-only: tanpa `updated_at`, tanpa `deleted_at`, tanpa endpoint ubah/hapus.

## 8.4 Migration setelahnya

| Migration | Phase | Isi |
|---|---|---|
| `0003_crm_core` | 2 | `leads`, `customers`, `activities`, `tasks` · `leads.lead_number` + `UNIQUE (organization_id, lead_number)` · `version` pada `leads` & `tasks` · `ALTER organizations ADD next_lead_number integer NOT NULL DEFAULT 1` |
| `0004_notifications` | 2 | `notifications` |
| `0005_api_keys` | 4 | `api_keys` + `leads.source_api_key_id` |
| `0006_device_tokens` | 5 | `device_tokens` |
| `0007_forms` | 6 | `forms` + `leads.source_form_id` |

`leads` dibuat di `0003` **tanpa** kolom `source_api_key_id` dan `source_form_id`; keduanya ditambahkan bersama tabel tujuannya. `leads.source` sudah menampung nilai `api` dan `form` sejak `0003` — nilai enum boleh mendahului FK-nya, dan menambahkan kolom nullable ke tabel yang sudah ada bersifat instan.

`organizations.next_lead_number` sengaja ditambahkan di `0003`, bukan di `0002`. Di `0002` belum ada tabel `leads`, sehingga kolom itu akan menjadi schema mati. `ALTER TABLE ADD COLUMN` dengan `DEFAULT` konstan bersifat instan pada PostgreSQL modern.

## 8.5 Test yang menyertai `0002`

Migration ini tidak dianggap selesai tanpa keempatnya:

1. **Round-trip** — `up` lalu `down` bersih, tanpa objek tersisa
2. **Composite FK** — mencoba menyisipkan membership org A ke tabel ber-FK milik org B **harus ditolak database**, bukan oleh kode aplikasi
3. **Partial unique** — dua membership aktif untuk (user, org) yang sama ditolak; membership yang sudah soft-deleted tidak menghalangi pembuatan yang baru
4. **Multi-membership diizinkan** — satu `user_id` boleh punya membership di dua organization berbeda. **Test ini menjaga ADR-007** dan akan gagal bila ada yang menambahkan `UNIQUE(user_id)`.
5. **Katalog** — query `information_schema` memastikan setiap tabel tenant-scoped punya `organization_id` dan `UNIQUE (id, organization_id)`

> Test #5 adalah penegak Aturan #1 dan #2 secara otomatis. Sekali ditulis, ia menangkap setiap tabel baru yang lupa mengikuti konvensi — untuk selamanya, tanpa ada yang perlu mengingatnya saat review.
>
> Test #4 adalah penegak ADR-007. Karena keputusan itu berupa **ketiadaan** constraint, tidak ada apapun di schema yang menandakannya — hanya test yang bisa menjaganya.

---

# Ringkasan Freeze

## Sudah terkunci

| Area | Status |
|---|---|
| Domain model & klasifikasi tabel | 🔒 Bagian 1 |
| Entity relationship & composite FK | 🔒 Bagian 2 |
| Lead status & activity type | 🔒 Bagian 2.4–2.5 |
| MVP scope | 🔒 Bagian 3 |
| Roadmap Phase 0–5 & pemetaan session | 🔒 Bagian 4 |
| 35 aturan arsitektur (termasuk #24 User App ≠ API Key) | 🔒 Bagian 5 |
| Konvensi API — penamaan, error, envelope, pagination | 🔒 Bagian 5.2 |
| Kebijakan efek samping & batas transaksi | 🔒 Bagian 5.3 |
| Struktur dokumentasi & protokol session | 🔒 Bagian 6 |
| Keputusan identity (B1–B5) | 🔒 Bagian 7 |
| Spesifikasi migration `0001` & `0002` | 🔒 Bagian 8 |

## Masih terbuka — tidak memblokir

| Hal | Diputuskan saat |
|---|---|
| B6–B9 (lost_reason, transisi status, task tanpa lead, konversi otomatis) | Menjelang Phase 2 |
| Email provider, hosting, retensi free tier, bahasa UI, push provider | Menjelang fase terkait |
| Pricing final, limit free tier, rate limit angka pasti | Menjelang Phase 8 |
| Kontrak integrasi payment service | Sebelum Phase 8 |

## Langkah berikutnya

```
1. Bootstrap Documentation        ← belum dikerjakan
   ├── CLAUDE.md
   ├── docs/STATUS.md
   ├── docs/product/{decisions,scope,glossary}.md
   ├── docs/architecture/multi-tenancy.md
   ├── docs/decisions/ADR-001..006
   └── .claude/skills/jualin-backend/
        ↓
2. Session 1 — Foundation
```

---

*Setiap keputusan di dokumen ini disertai alasan agar bisa ditolak secara sadar, bukan diikuti secara buta. Setelah freeze, penolakan itu berbentuk ADR baru.*
