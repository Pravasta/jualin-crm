# Glossary

> **Tujuan: mencegah drift penamaan lintas session.**
>
> Tanpa dokumen ini, session 2 menulis `employee_id`, session 5 menulis `member_id`, session 8 menulis `staff_id` — semuanya merujuk hal yang sama, dan tidak ada yang menyadarinya sampai refactor menyakitkan.
>
> **Nama di kolom "Identifier" adalah nama yang dipakai di kode, database, dan API.** Tidak ada sinonim.

---

## Identity & Tenancy

| Istilah | Identifier | Definisi |
|---|---|---|
| **Organization** | `organizations`, `organization_id` | Tenant. Akar seluruh isolasi data. Satu organization = satu business account = satu subscription. |
| **User** | `users`, `user_id` | Identitas login. **Global, lintas organization.** Email unik di seluruh sistem. Tidak punya `organization_id`. |
| **Membership** | `memberships`, `membership_id` | Keanggotaan user pada satu organization + role. **Inilah "Employee".** Assignment dan aktor selalu menunjuk ini. |
| **Employee** | — | **Bukan entity.** = Membership dengan `role = 'employee'`. Jangan pernah membuat tabel `employees`. |
| **Role** | `memberships.role` | Enum: `owner` · `admin` · `manager` · `employee`. **Bukan tabel.** |
| **Invitation** | `invitations` | Undangan bergabung ke organization. Token sekali pakai, kedaluwarsa 7 hari, `role <> 'owner'`. |
| **Tenant context** | `TenantContext` | Struct yang membawa `organization_id`, principal, role, request id. **Parameter pertama setiap method repository.** |
| **Principal** | `TenantContext.PrincipalType` | Siapa yang melakukan request: `user` · `api_key` · `public_form` · `system`. Bukan sekadar `organization_id`. |

---

## CRM Core

| Istilah | Identifier | Definisi |
|---|---|---|
| **Lead** | `leads`, `lead_id` | Domain inti. **Peristiwa capture** — seseorang menunjukkan minat. Semua sumber menghasilkan bentuk yang sama. |
| **Lead number** | `leads.lead_number` | Nomor urut per organization, mulai dari 1. **Ini yang ditampilkan ke pengguna** (`#1024`), bukan UUID. |
| **Lead status** | `leads.status` | `new` · `contacted` · `qualified` · `proposal` · `won` · `lost` · `unqualified` · `spam`. **Ini juga pipeline di MVP** (ADR-006). |
| **Lead source** | `leads.source` | **Metode capture**, bukan channel marketing: `manual` · `api` · `form` · `webhook`. |
| **Assignment** | `leads.assigned_to_membership_id` | **Bukan entity.** Kolom yang menyimpan keadaan sekarang. Riwayatnya ada di Activity. |
| **Customer** | `customers`, `customer_id` | **Relasi berkelanjutan.** Entity baru hasil konversi Lead — bukan Lead yang berganti status. Satu Customer bisa berasal dari beberapa Lead. |
| **Conversion** | `leads.converted_customer_id` | Aksi **eksplisit**, bukan otomatis saat status `won`. |
| **Activity** | `activities` | Sesuatu yang **sudah terjadi**. **Append-only, immutable** — tanpa `updated_at`, `deleted_at`, atau endpoint edit/hapus. |
| **Task** | `tasks` | Sesuatu yang **harus dilakukan**. Mutable, punya `due_at`, `status`, assignee. |
| **Deal** | `deals` | Opportunity bernilai. **Belum ada di MVP** — Phase 9. |
| **Pipeline** | — | Di MVP = urutan `lead.status`. **Tidak ada tabel `pipelines`** sampai Phase 9. |

> **Activity vs Task** adalah pembedaan yang tidak boleh kabur: *sudah terjadi* vs *harus dilakukan*. Menggabungkannya membuat timeline tidak berguna.

---

## Integration

| Istilah | Identifier | Definisi |
|---|---|---|
| **API Key** | `api_keys` | Kredensial **rahasia** milik organization untuk sistem eksternal. Format `jln_live_<key_id>_<secret>`. **Tidak pernah** di sisi klien. |
| **Public form key** | `forms.public_key` | Identifier **publik** untuk embedded form. Bukan rahasia. Hanya bisa submit ke form itu. |
| **Form** | `forms` | Definisi embedded form + domain allowlist. Phase 6. |
| **Webhook** | `webhook_endpoints` | Phase 7. Inbound dan outbound adalah **dua konsep berbeda**, bukan satu tabel. |
| **Idempotency key** | `leads.idempotency_key` | Header dari client, unik per organization. Pengulangan mengembalikan **response asli**, bukan error. |

---

## Subscription

| Istilah | Identifier | Definisi |
|---|---|---|
| **Plan** | `plans.plan_code` | Definisi limit. Di Phase 1 hanya konstanta `'free'` di kode; tabel penuh di Phase 8. |
| **Subscription** | `subscriptions` | Organization → plan + status. **Milik organization, tidak pernah milik user.** |
| **External reference** | `subscriptions.external_reference` | ID di payment service eksternal. CRM **tidak** memproses pembayaran. |
| **Entitlement** | `plans.entitlements` | Batas per plan (JSONB). Phase 8. |
| **Usage counter** | `usage_counters` | Pemakaian terhitung per periode. Phase 8. |

---

## Istilah yang **dilarang** dipakai

| Jangan pakai | Pakai | Alasan |
|---|---|---|
| `employee_id`, `staff_id`, `member_id` | `membership_id` | Employee bukan entity |
| tabel `employees` | `memberships` | ADR-003 |
| tabel `roles`, `permissions` | kolom `memberships.role` | RBAC dinamis ditunda |
| tabel `assignments` | `leads.assigned_to_membership_id` | Assignment bukan entity |
| tabel `form_submissions` | `leads` + `raw_payload` | Submission valid **adalah** Lead |
| "Team" di UI | "semua lead" / "seluruh organisasi" | Entity `Team` belum ada — UI tidak boleh menjanjikannya |
| "Workspace" | "Organization" | Satu istilah saja |
| `tenant_id` | `organization_id` | Konsistensi mutlak — RLS di masa depan bergantung padanya (Aturan #1) |

---

## Catatan bahasa

- **Dokumentasi & komunikasi:** Bahasa Indonesia
- **Kode, nama kolom, nama endpoint, pesan error `code`:** Bahasa Inggris
- **Bahasa UI produk:** belum diputuskan (lihat `docs/STATUS.md`)
