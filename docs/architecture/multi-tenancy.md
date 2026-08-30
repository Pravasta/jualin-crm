# Multi-Tenancy

> **Dokumen ini hampir selalu relevan.** Baca di setiap session yang menyentuh data.
> Sumber: `freeze.md` bagian 5 (Aturan #1–#7) · ADR-002

---

## Strategi

```
PostgreSQL
  └── shared database
       └── shared schema
            └── organization_id di setiap tabel bisnis
```

Satu-satunya yang layak secara ekonomi di model harga Jualin. Schema-per-tenant meledak saat 1000 tenant (migrasi harus jalan 1000 kali); database-per-tenant terlalu mahal.

**Kegagalan di area ini bersifat fatal.** Satu kebocoran lintas tenant menghapus kredibilitas B2B secara permanen, dan tidak ada permintaan maaf yang memulihkannya.

`organization_id` saja **tidak cukup** — itu hanya mengandalkan disiplin developer, dan disiplin gagal pada jam 2 pagi di sprint yang sibuk. Karena itu: empat lapis.

---

## Lapis 1 — Repository yang tidak bisa lupa

Tidak boleh ada jalur query yang menerima org sebagai parameter opsional.

```go
// BENAR — org_id di-inject di dalam, bukan diserahkan ke caller
func (r *LeadRepo) FindByID(ctx context.Context, t TenantContext, id uuid.UUID) (*Lead, error)

// DILARANG — method seperti ini tidak boleh ada sama sekali
func (r *LeadRepo) FindByID(ctx context.Context, id uuid.UUID) (*Lead, error)
```

**Prinsipnya:** kalau method tanpa tenant tidak ada, tidak ada yang bisa memanggilnya secara salah. *Make illegal states unrepresentable*, diterapkan pada tenancy.

Ini bukan abstraksi tambahan — ini bentuk signature yang sama-sama harus ditulis, hanya dipilih yang aman.

**Konsekuensi:** ini alasan tambahan menolak ORM. Dengan sqlc/pgx, setiap query bisa dibaca dan diaudit sebagai teks.

---

## Lapis 2 — Composite foreign key

**Bagian yang hampir selalu terlewat.** Lapis 1 melindungi *pembacaan*. Yang tidak terlindungi: **referensi silang tenant**.

### Skenario nyata

```
Owner Org A → PATCH /v1/leads/{lead_milik_A}
              { "assigned_to_membership_id": "<membership milik Org B>" }
```

- Query lead-nya tenant-scoped → **lolos**
- FK ke `memberships(id)` valid → **lolos**
- **Hasil: data korup lintas tenant tanpa satupun error**

Kalau `assigned_to` menunjuk `user_id`, kerusakannya lebih parah: karyawan Org B melihat lead Org A di aplikasi mobile-nya.

### Pencegahan di database

```sql
-- Pada SETIAP tabel tenant-scoped — jangkar composite FK
CONSTRAINT uq_<tabel>_id_org UNIQUE (id, organization_id)

-- Pada SETIAP FK antar entity bisnis
CONSTRAINT fk_leads_assigned_membership
  FOREIGN KEY (assigned_to_membership_id, organization_id)
  REFERENCES memberships (id, organization_id)
```

Sekarang database **mustahil** menyimpan referensi lintas tenant, apapun bug di kode aplikasi.

> `UNIQUE (id, organization_id)` terlihat berlebihan karena `id` sudah PK. Ia **wajib**: PostgreSQL mensyaratkan unique constraint tepat pada kolom yang direferensikan composite FK.

### Berlaku untuk

`lead → membership` · `lead → customer` · `lead → api_key` · `lead → form` · `task → lead` · `task → membership` · `activity → lead` · `activity → membership` · `notification → membership` · `invitation → membership` · `refresh_token → membership` · `audit_log → membership` · `deal → customer`

**Tanpa pengecualian.** FK yang menunjuk `organizations` atau `users` memakai bentuk biasa, karena keduanya bukan tabel tenant-scoped.

> Biayanya satu unique index tambahan per tabel. **Ini rasio manfaat-per-biaya tertinggi di seluruh sistem**, dan ia bekerja bahkan ketika kode aplikasi salah.

### Pengecualian tertulis — `api_keys.key_id` unik lintas organization, bukan pelanggaran

`uq_api_keys_key_id` (migration `0005`, Phase 4 #46) **bukan** composite `UNIQUE (key_id,
organization_id)` seperti pola di atas — ia unik **lintas** seluruh organization. Ini terlihat seperti
pelanggaran Lapis 2 bila dibaca sekilas; alasannya sama persis dengan `refresh_tokens.token_hash` di
Phase 1: lookup kredensial terjadi **sebelum** organization diketahui — organization justru *hasil* dari
lookup itu (Aturan #5, `authentication.md` bagian "API key"). Composite unique di sini tidak mungkin
dibuat: tidak ada `organization_id` untuk dijadikan bagian kunci sebelum baris ditemukan.

`Repository.FindByKeyID` (`internal/apikey`) adalah pengecualian tertulis yang sama seperti
`RefreshTokenRepository.FindByHashForUpdate` — tidak menerima `tenant.Context` sama sekali, untuk alasan
yang sama.

### Pengecualian ketiga — `device_tokens.token` unik lintas organization (Phase 5, issue #68)

`uq_device_tokens_token` (migration `0006`) unik **lintas** seluruh organization juga — tapi alasannya
**berbeda** dari dua pengecualian di atas, bukan diulang begitu saja. `api_keys.key_id`/
`refresh_tokens.token_hash` unik lintas organization karena lookup-nya terjadi **sebelum** organization
diketahui. `device_tokens.token` unik lintas organization karena perangkat fisik bisa **berpindah
pemilik** — seseorang keluar dari satu organization dan bergabung ke organization lain di perangkat
yang sama, dan token FCM-nya tetap sama. Registrasi ulang di organization baru memakai `ON CONFLICT
(token) DO UPDATE` untuk **memindahkan** baris (`organization_id`, `membership_id` diperbarui), bukan
menolak sebagai duplikat atau membiarkan dua baris tenant berbeda memegang token yang sama sekaligus
(`internal/device/repository_postgres.go`'s `Upsert`).

---

## Lapis 3 — Row Level Security (ditunda)

RLS PostgreSQL dengan `SET LOCAL app.current_org_id` + policy per tabel.

**Ditunda ke pasca-MVP.** Alasan jujur: RLS berinteraksi rumit dengan connection pooling dan menambah beban debugging nyata untuk tim kecil. Lapis 1, 2, dan 4 sudah memberi perlindungan sangat besar.

**Dua hal yang wajib dijaga sekarang agar RLS bisa masuk nanti tanpa nyeri** — keduanya gratis hari ini, mahal diperbaiki nanti:

1. Nama kolom **konsisten `organization_id`** di semua tabel, tanpa pengecualian
2. **Semua** akses DB lewat satu transaction manager — satu tempat untuk menyisipkan `SET LOCAL`

---

## Lapis 4 — Test suite isolasi tenant

**Wajib. Blocking di CI.** Dibangun di issue #11 — `cmd/api/tenant_isolation_test.go`.

### Bentuknya generik, bukan per endpoint

Test manual per endpoint akan tertinggal begitu endpoint bertambah. Buat **satu test yang berjalan di atas daftar route**:

```
Untuk setiap endpoint tenant-scoped:
  siapkan Org A + data, Org B + data
  panggil endpoint dengan kredensial A terhadap resource id milik B
  harapkan 404
```

**Selalu 404, jangan 403** — 403 mengonfirmasi bahwa resource dengan id tersebut ada, dan itu kebocoran informasi tersendiri.

### Kasus wajib

| # | Kasus | Menjaga | Status Phase 1 |
|---|---|---|---|
| 1 | Baca resource tenant lain → 404 | Lapis 1 | ✅ `GET /v1/leads/{id}`, `GET /v1/leads/{id}/activities`, `GET /v1/customers/{id}` (dan `PATCH`/`DELETE` yang setara) — `TestTenantIsolation_CrossOrgMutatingByID_Returns404`, ditambah issue #23. Tidak ada endpoint GET-by-id lintas tenant di Phase 1 sendiri (hanya list yang di-scope query, dan `GET /v1/invitations/token/{token}` yang memang publik by design) |
| 2 | Ubah/hapus resource tenant lain → 404 | Lapis 1 | ✅ `PATCH`/`DELETE /v1/memberships/{id}`, `DELETE /v1/invitations/{id}` (Phase 1); `PATCH`/`DELETE /v1/leads/{id}`, `PATCH`/`DELETE /v1/tasks/{id}`, `PATCH`/`DELETE /v1/customers/{id}` (Phase 2, #23); `DELETE /v1/api-keys/{id}` (Phase 4, #46) — `TestTenantIsolation_CrossOrgMutatingByID_Returns404` |
| 3 | Menunjuk membership tenant lain di body → ditolak **database** | Lapis 2 | Ditegakkan lewat composite FK sejak #8; belum ada endpoint Phase 1 yang menerima id membership lewat body request (invitation accept menunjuk lewat token, bukan id) |
| 4 | Employee membaca lead employee lain di org yang sama → 404 | Otorisasi | ✅ `internal/lead/handler_test.go`'s `TestHandler_Get_Employee_OtherPersonsLead_Returns404`, dan padanannya di `internal/task`, `internal/activity`, `internal/customer` (#20–#23) — `authz` sudah memberi Employee akses nol ke membership/invitation sejak Phase 1, `leads` adalah tempat pertama aturan ini punya kasus nyata |
| 5 | **User dengan dua membership** tidak bisa melihat data org yang tidak sedang aktif di token | ADR-007 | ✅ `TestTenantIsolation_MultiMembership_OnlySeesActiveOrgInToken` |
| 6 | Katalog: setiap tabel tenant-scoped punya `organization_id` + `UNIQUE (id, organization_id)` | Aturan #1, #2 | ✅ Sejak #8 |
| 7 | Endpoint agregat (tanpa `:id`) tidak membocorkan bentuk bisnis tenant lain | Lapis 1 | ✅ `GET /v1/metrics/{summary,employees}` — `TestTenantIsolation_MetricsAggregate_ScopedToOrganization` (Phase 3, #30) |
| 8 | Hapus resource tenant lain yang dialamatkan lewat **token**, bukan `:id` → 404 | Lapis 1 | ✅ `DELETE /v1/device-tokens` — tidak masuk bentuk kasus #1/#2 (keduanya mengasumsikan `:id` di path); diuji terpisah karena target-nya ada di body (Phase 5, #68) |

**Kasus #1 dan #4 sekarang ✅ — ditutup di issue #23**, penutup Phase 2. Harness
(`cmd/api/tenant_isolation_test.go`) tetap satu slice `[]isolationCase` generik yang sama sejak #11;
#23 menambah entri untuk `lead`/`task`/`activity`/`customer` ke slice itu, bukan harness baru — persis
seperti direncanakan.

**Kasus #5 tidak boleh dilewatkan.** Schema mengizinkan multi-membership yang tidak diekspos UI; satu-satunya penjaganya adalah test ini.

**Kasus #6 adalah penegak otomatis.** Sekali ditulis, ia menangkap setiap tabel baru yang lupa mengikuti konvensi — untuk selamanya, tanpa ada yang perlu mengingatnya saat review.

### Kasus #7 — endpoint agregat (Phase 3, issue #30)

`GET /v1/metrics/summary` dan `GET /v1/metrics/employees` tidak punya `:id` — mereka tidak masuk bentuk
"kasus #1/#2" (mutating/reading resource by id). Kebocoran di sini bukan satu baris salah tenant, tapi
**bentuk bisnis** tenant lain (jumlah lead, conversion rate) yang bocor lewat agregat. Diuji terpisah:
`TestTenantIsolation_MetricsAggregate_ScopedToOrganization` di file yang sama, dibuktikan bisa gagal
dengan cara yang sama seperti kasus lain — lihat komentar di test itu.

### `POST /v1/leads` jalur API key — sengaja tanpa kasus baru (Phase 4, issue #47)

Endpoint publik satu-satunya di Phase 4 **tidak** menerima `:id` tenant lain sama sekali — tidak masuk
bentuk kasus #1/#2, dan bukan endpoint agregat seperti kasus #7. `t.OrganizationID` untuk request ini
selalu berasal dari hasil `Repository.FindByKeyID`, tidak pernah dari input request apa pun — secara
struktural tidak ada permukaan bagi kredensial org A untuk "menunjuk" ke data org B, karena tidak ada
field yang bisa dipakai menunjuk ke sana. Dianalisis, bukan diuji dengan harness generik yang bentuknya
tidak cocok untuk kasus ini — dicatat di `docs/phases/04-public-api/notes.md` bagian `## #47`.

### Kriteria kualitas harness

> Harness harus terbukti **bisa gagal**. Hapus `organization_id` dari satu query secara sengaja; kalau test tetap hijau, harness-nya belum benar.
>
> Test isolasi yang selalu hijau karena tidak benar-benar menguji apapun **lebih berbahaya daripada tidak ada test** — ia memberi rasa aman palsu pada semua session berikutnya.

**Dibuktikan di #11**: predikat `AND organization_id = $2` dihapus sementara dari
`membership.postgresRepository.FindByID`, harness dijalankan ulang — dua dari tiga subtest
`TestTenantIsolation_CrossOrgMutatingByID_Returns404` langsung merah (500, bukan 404). Predikat
dikembalikan sebelum commit; tidak pernah masuk riwayat git. Detail lengkap di
`docs/phases/01-auth-organization/notes.md`'s `## #11`.

**Diulang di #23** untuk resource Phase 2: predikat tenant-scoping yang sama dihapus sementara dari
`lead.postgresRepository.FindByID`, harness dijalankan ulang — `GET /v1/leads/{id} on another org's
lead` langsung merah, bocor **200 dengan data lengkap** lead org lain (bukan sekadar 500); subtest
`PATCH` juga merah (409 `version_conflict` alih-alih 404, karena baris jadi terlihat lintas tenant).
Predikat dikembalikan sebelum commit. Detail lengkap di `docs/phases/02-crm-core/notes.md`'s `## #23`.

---

## Tenant context

```
Request
  → Auth middleware (user session | API key | public form key)
  → resolve principal
  → resolve organization_id     ← SELALU dari kredensial
  → cek status subscription
  → cek quota                    (Phase 8)
  → rate limit
  → Service
  → Repository
```

```go
type TenantContext struct {
    OrganizationID uuid.UUID
    PrincipalType  PrincipalType   // user | api_key | public_form | system
    MembershipID   *uuid.UUID      // nil bila API key
    UserID         *uuid.UUID
    Role           Role            // kosong bila API key — lihat Scopes
    APIKeyID       *uuid.UUID
    Scopes         []string        // hanya terisi bila PrincipalType == api_key (Phase 4, #47)
    RequestID      string
}
```

`Scopes` adalah mekanisme otorisasi untuk principal **tanpa role** — lihat `authorization.md` bagian
"Otorisasi berbasis scope". Jalur `PrincipalUser` tidak pernah membacanya; jalur `PrincipalAPIKey` tidak
pernah membaca `Role`. Keduanya jalur eksklusif, tidak pernah tercampur di satu keputusan otorisasi.

### Dua aturan yang mudah dilanggar tanpa sadar

**1. `organization_id` tidak pernah datang dari client** (Aturan #5)

Tidak ada DTO yang boleh punya field `organization_id`. Bukan divalidasi — **tidak ada sama sekali**. Kalau field-nya tidak ada, tidak ada yang bisa memalsukannya.

**2. `PrincipalType` wajib diperiksa, bukan hanya `OrganizationID`** (Aturan #24)

Dua request dengan `organization_id` sama tapi principal berbeda **bukan** request yang setara. Tanpa pemeriksaan ini, endpoint yang mengecek tenant dengan benar tetap bisa menerima API key di tempat yang seharusnya hanya menerima user session.

---

## Checklist saat menambah tabel baru

- [ ] Diklasifikasikan: tenant root / global identity / tenant-scoped / session
- [ ] Bila tenant-scoped: `organization_id NOT NULL`
- [ ] Bila tenant-scoped: `UNIQUE (id, organization_id)`
- [ ] Setiap FK ke tabel tenant-scoped berbentuk composite
- [ ] Index tenant-aware berawalan `organization_id`
- [ ] Repository-nya menerima `TenantContext` di parameter pertama
- [ ] Masuk ke harness test isolasi
- [ ] Bila melanggar salah satu di atas: **tulis ADR**, jangan diam-diam
